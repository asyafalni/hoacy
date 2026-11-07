package sinkron

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Item is one row as the admin sees it in the preview.
type Item struct {
	Kunci   string `json:"kunci"`
	Ringkas string `json:"ringkas"`
}

type Ubah struct {
	Kunci string `json:"kunci"`
	Lama  string `json:"lama"`
	Baru  string `json:"baru"`
}

// Galat is a Sheet row that can't be brought in (unknown house, bad number…).
type Galat struct {
	Baris int    `json:"baris"` // 1-based row number in the Sheet
	Pesan string `json:"pesan"`
}

type Pratinjau struct {
	Tab    string  `json:"tab"`
	Tambah []Item  `json:"tambah"`
	Ubah   []Ubah  `json:"ubah"`
	Hilang []Item  `json:"hilang"` // in the database, gone from the Sheet: deleted only if ticked
	Galat  []Galat `json:"galat"`
	Sama   int     `json:"sama"`
}

// rekam is one row in a comparable shape. sidik is the canonical text of every
// field that matters: equal sidik = nothing to change.
type rekam struct {
	kunci, ringkas, sidik string
	data                  any
}

type konteks struct {
	alamat map[string]bool // houses that exist (active or not)
}

type adapter struct {
	// dariSheet turns the tab's rows (header included) into records.
	dariSheet func(rows [][]string, k konteks) ([]rekam, []Galat)
	// dariDB is what the database holds for this tab, by key.
	dariDB func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error)
	simpan func(ctx context.Context, tx pgx.Tx, r rekam, baru bool) error
	hapus  func(ctx context.Context, tx pgx.Tx, kunci string) error
	master bool // changes are audited
}

var polaImpor = regexp.MustCompile(`^M-Impor(\d{4})$`)

// Tabs the sync knows, in the order a first import must run them.
var TabTetap = []string{"M-Blok", "M-Rumah", "M-RumahRiwayat", "M-TarifVersi",
	"L-Tunai", "L-Transfer", "L-Keputusan", "L-Setoran", "L-Pengeluaran"}

func adapterUntuk(tab string) (*adapter, error) {
	if m := polaImpor.FindStringSubmatch(tab); m != nil {
		var tahun int
		fmt.Sscan(m[1], &tahun)
		return adapterImpor(tab, tahun), nil
	}
	switch tab {
	case "M-Blok":
		return adapterBlok(), nil
	case "M-Rumah":
		return adapterRumah(), nil
	case "M-RumahRiwayat":
		return adapterRiwayat(), nil
	case "M-TarifVersi":
		return adapterTarif(), nil
	case "L-Tunai":
		return adapterKiriman(tab, "tunai"), nil
	case "L-Transfer":
		return adapterKiriman(tab, "transfer"), nil
	case "L-Keputusan":
		return adapterKeputusan(), nil
	case "L-Setoran":
		return adapterSetoran(), nil
	case "L-Pengeluaran":
		return adapterPengeluaran(), nil
	}
	return nil, fmt.Errorf("tab %q tidak bisa disinkron", tab)
}

// BisaDisinkron filters a spreadsheet's tab list down to the ones this knows.
func BisaDisinkron(tabs []string) []string {
	var out []string
	for _, t := range TabTetap {
		for _, x := range tabs {
			if x == t {
				out = append(out, t)
			}
		}
	}
	var impor []string
	for _, x := range tabs {
		if polaImpor.MatchString(x) {
			impor = append(impor, x)
		}
	}
	sort.Strings(impor)
	return append(out, impor...)
}

type Sinkron struct {
	DB     *pgxpool.Pool
	Sumber Sumber
}

// Pratinjau computes what applying the tab would do, changing nothing.
func (s *Sinkron) Pratinjau(ctx context.Context, tab string) (*Pratinjau, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	p, _, err := s.banding(ctx, tx, tab)
	return p, err
}

// Terapkan applies the tab. hapus lists the keys of "hilang" rows to delete;
// anything not in the fresh comparison is ignored. Logged in sinkron_log.
func (s *Sinkron) Terapkan(ctx context.Context, tab string, hapus []string, oleh string) (*Pratinjau, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// one sync at a time, whoever clicks
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(4247)`); err != nil {
		return nil, err
	}
	p, c, err := s.banding(ctx, tx, tab)
	if err != nil {
		return nil, err
	}
	a := c.adapter
	for _, it := range p.Tambah {
		if err := a.simpan(ctx, tx, c.sheet[it.Kunci], true); err != nil {
			return nil, fmt.Errorf("%s: %w", it.Kunci, err)
		}
		if a.master {
			audit(ctx, tx, tab, it.Kunci, nil, c.sheet[it.Kunci].sidik, oleh)
		}
	}
	for _, u := range p.Ubah {
		if err := a.simpan(ctx, tx, c.sheet[u.Kunci], false); err != nil {
			return nil, fmt.Errorf("%s: %w", u.Kunci, err)
		}
		if a.master {
			lama := c.db[u.Kunci].sidik
			audit(ctx, tx, tab, u.Kunci, &lama, c.sheet[u.Kunci].sidik, oleh)
		}
	}
	boleh := map[string]bool{}
	for _, h := range p.Hilang {
		boleh[h.Kunci] = true
	}
	dihapus := 0
	for _, k := range hapus {
		if !boleh[k] {
			continue
		}
		if err := a.hapus(ctx, tx, k); err != nil {
			return nil, fmt.Errorf("hapus %s: %w", k, err)
		}
		if a.master {
			lama := c.db[k].sidik
			audit(ctx, tx, tab, k, &lama, "", oleh)
		}
		dihapus++
	}
	ringkas, _ := json.Marshal(map[string]int{"tambah": len(p.Tambah), "ubah": len(p.Ubah),
		"hapus": dihapus, "hilang": len(p.Hilang) - dihapus, "galat": len(p.Galat)})
	if _, err := tx.Exec(ctx, `insert into sinkron_log (tab, oleh, ringkasan) values ($1, $2, $3)`, tab, oleh, ringkas); err != nil {
		return nil, err
	}
	return p, tx.Commit(ctx)
}

type perbandingan struct {
	adapter   *adapter
	sheet, db map[string]rekam
}

func (s *Sinkron) banding(ctx context.Context, tx pgx.Tx, tab string) (*Pratinjau, *perbandingan, error) {
	a, err := adapterUntuk(tab)
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.Sumber.Baca(ctx, tab)
	if err != nil {
		return nil, nil, err
	}
	k := konteks{alamat: map[string]bool{}}
	r, err := tx.Query(ctx, `select alamat from rumah`)
	if err != nil {
		return nil, nil, err
	}
	list, err := pgx.CollectRows(r, pgx.RowTo[string])
	if err != nil {
		return nil, nil, err
	}
	for _, x := range list {
		k.alamat[x] = true
	}
	if tab == "M-Rumah" { // its own rows are the houses that will exist
		k.alamat = nil
	}
	daftar, galat := a.dariSheet(rows, k)
	db, err := a.dariDB(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	p := &Pratinjau{Tab: tab, Tambah: []Item{}, Ubah: []Ubah{}, Hilang: []Item{}, Galat: galat}
	if p.Galat == nil {
		p.Galat = []Galat{}
	}
	sheet := map[string]rekam{}
	for _, x := range daftar {
		sheet[x.kunci] = x
		lama, ada := db[x.kunci]
		switch {
		case !ada:
			p.Tambah = append(p.Tambah, Item{x.kunci, x.ringkas})
		case lama.sidik != x.sidik:
			p.Ubah = append(p.Ubah, Ubah{x.kunci, lama.ringkas, x.ringkas})
		default:
			p.Sama++
		}
	}
	for kunci, x := range db {
		if _, ada := sheet[kunci]; !ada {
			p.Hilang = append(p.Hilang, Item{kunci, x.ringkas})
		}
	}
	sort.Slice(p.Hilang, func(i, j int) bool { return p.Hilang[i].Kunci < p.Hilang[j].Kunci })
	return p, &perbandingan{a, sheet, db}, nil
}

func audit(ctx context.Context, tx pgx.Tx, tab, kunci string, sebelum *string, sesudah, oleh string) {
	js := func(s string) any {
		if s == "" {
			return nil
		}
		b, _ := json.Marshal(map[string]string{"sinkron": s})
		return b
	}
	var lama any
	if sebelum != nil {
		lama = js(*sebelum)
	}
	tx.Exec(ctx, `insert into audit (tabel, kunci, sebelum, sesudah, oleh) values ($1, $2, $3, $4, $5)`,
		tab, kunci, lama, js(sesudah), oleh)
}

// data is the rows below the header, with their 1-based Sheet row numbers.
func data(rows [][]string) ([][]string, []int) {
	if len(rows) < 2 {
		return nil, nil
	}
	nomor := make([]int, len(rows)-1)
	for i := range nomor {
		nomor[i] = i + 2
	}
	return rows[1:], nomor
}

func kosong(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

package api

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	fibhttp "github.com/lesismal/fib/http"

	"github.com/asyafalni/hoacy/backend/internal/auth"
	"github.com/asyafalni/hoacy/backend/internal/berkas"
	"github.com/asyafalni/hoacy/backend/internal/store"
	"github.com/asyafalni/hoacy/backend/internal/tagihan"
)

func (a *App) getSaya(c *fibhttp.Context, p *auth.Pengurus) {
	a.json(c, 200, map[string]any{"ok": true, "saya": p})
}

// kasRumah is one house on the Kas lists (phone included — pengurus only).
type kasRumah struct {
	*tagihan.Rumah
	Nama     string   `json:"nama"`
	Telp     string   `json:"telp"`
	Cluster  string   `json:"cluster"`
	Blok     string   `json:"blok"`
	Nomor    string   `json:"rumah"`
	Status   []string `json:"status"` // this year's 12 months
	LunasIni bool     `json:"lunasBulanIni"`
}

// Kiriman is one submission with its months, as the Kas screen verifies it.
type Kiriman struct {
	ID        int64          `json:"kirimanId"`
	Alamat    string         `json:"alamat"`
	Metode    string         `json:"metode"`
	Waktu     *time.Time     `json:"waktu"`
	Petugas   string         `json:"petugas"`
	AdaBukti  bool           `json:"adaBukti"`
	Total     int64          `json:"total"`
	Keabsahan string         `json:"keabsahan"` // shared by its months; "dobel" if any is
	Kurang    bool           `json:"kurang"`    // a month claimed below its tarif
	Items     []KirimanBulan `json:"items"`
}

type KirimanBulan struct {
	Periode   int    `json:"periode"`
	Nominal   int64  `json:"nominal"`
	Tarif     *int64 `json:"tarif"`
	Keabsahan string `json:"keabsahan"`
}

func kelompok(rows []store.Bayar, t *tagihan.Tarif) []Kiriman {
	idx := map[int64]int{}
	var out []Kiriman
	for _, r := range rows {
		i, ada := idx[r.KirimanID]
		if !ada {
			i = len(out)
			idx[r.KirimanID] = i
			out = append(out, Kiriman{ID: r.KirimanID, Alamat: r.Alamat, Metode: r.Metode, Waktu: r.Waktu,
				Petugas: r.Petugas, AdaBukti: r.AdaBukti, Keabsahan: r.Keabsahan, Items: []KirimanBulan{}})
		}
		k := &out[i]
		it := KirimanBulan{Periode: r.Periode, Nominal: r.Nominal, Keabsahan: r.Keabsahan}
		if tr, ok := t.Pada(r.Alamat, r.Periode); ok {
			it.Tarif = &tr
			if r.Nominal < tr {
				k.Kurang = true
			}
		}
		k.Items = append(k.Items, it)
		k.Total += r.Nominal
		if r.Keabsahan != k.Keabsahan && (r.Keabsahan == "dobel" || k.Keabsahan == "dobel") {
			k.Keabsahan = "sebagian-dobel"
		}
	}
	for i := range out {
		sort.Slice(out[i].Items, func(a, b int) bool { return out[i].Items[a].Periode < out[i].Items[b].Periode })
	}
	if out == nil {
		out = []Kiriman{}
	}
	return out
}

// GET /api/kas — everything the Kas home screen shows, in one round trip.
func (a *App) getKas(c *fibhttp.Context, p *auth.Pengurus) {
	sekarang := a.sekarang()
	k, err := a.Store.Kas(c.Context(), sekarang)
	if err != nil {
		a.galat(c, err)
		return
	}
	t, houses := k.Dasar.Hitung(sekarang)
	rumah := make([]kasRumah, len(houses))
	for i, h := range houses {
		r := k.Dasar.Rumah[i]
		telp := ""
		if r.Telp != nil {
			telp = *r.Telp
		}
		rumah[i] = kasRumah{Rumah: h, Nama: r.Nama, Telp: telp, Cluster: r.Cluster, Blok: r.Blok, Nomor: r.Nomor,
			Status: h.KartuTahun(tagihan.Tahun(sekarang)).Status, LunasIni: h.Lunas(sekarang)}
	}
	var antrian, cek []store.Bayar
	for _, b := range k.Antrian {
		if b.Keabsahan == "cek" {
			cek = append(cek, b)
		} else {
			antrian = append(antrian, b)
		}
	}
	a.json(c, 200, map[string]any{"ok": true, "hari": a.hari(), "saya": p, "saldo": k.Saldo, "rumah": rumah,
		"pending": kelompok(antrian, t), "cek": kelompok(cek, t), "tunai": kelompok(k.KasMasuk, t), "warna": k.Dasar.Warna})
}

// GET /api/kas/rumah/{alamat} — one house's full card, every year.
func (a *App) getKasRumah(c *fibhttp.Context, _ *auth.Pengurus) {
	d, err := a.Store.DasarRumah(c.Context(), c.Param("alamat"))
	if errors.Is(err, store.ErrTidakAda) {
		a.gagal(c, 404, "tidak-ada", "Rumah tidak ditemukan.")
		return
	} else if err != nil {
		a.galat(c, err)
		return
	}
	w := bangunWarga(d, a.sekarang())
	telp := ""
	if d.Rumah[0].Telp != nil {
		telp = *d.Rumah[0].Telp
	}
	a.json(c, 200, map[string]any{"ok": true, "hari": a.hari(), "rumah": w, "telp": telp})
}

// GET /api/kas/tahun/{tahun} — one dues year's per-month totals.
func (a *App) getKasTahun(c *fibhttp.Context, _ *auth.Pengurus) {
	tahun, _ := strconv.Atoi(c.Param("tahun"))
	rows, err := a.Store.Tahun(c.Context(), tahun)
	if err != nil {
		a.galat(c, err)
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "tahun": tahun, "ringkasan": tagihan.RingkasTahun(rows)})
}

func (a *App) getBukti(c *fibhttp.Context, _ *auth.Pengurus) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ref, err := a.Store.BuktiKiriman(c.Context(), id)
	if err != nil {
		a.gagal(c, 404, "berkas", "Kiriman ini tidak punya bukti.")
		return
	}
	a.kirimBerkas(c, ref)
}

func (a *App) getNota(c *fibhttp.Context, _ *auth.Pengurus) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	ref, err := a.Store.NotaPengeluaran(c.Context(), id)
	if err != nil {
		a.gagal(c, 404, "berkas", "Pengeluaran ini tidak punya nota.")
		return
	}
	a.kirimBerkas(c, ref)
}

// POST /api/kas/keputusan {kirimanId, keputusan: sah|tolak}. Latest wins, so
// a mistaken tolak is undone with a sah.
func (a *App) postKeputusan(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct {
		KirimanID int64  `json:"kirimanId"`
		Keputusan string `json:"keputusan"`
	}
	if !a.baca(c, &req) {
		return
	}
	if req.Keputusan != "sah" && req.Keputusan != "tolak" {
		a.gagal(c, 400, "keputusan", "Keputusan harus sah atau tolak.")
		return
	}
	err := a.Store.Putuskan(c.Context(), req.KirimanID, req.Keputusan, p.Nama)
	if errors.Is(err, store.ErrTidakAda) {
		a.gagal(c, 404, "tidak-ada", "Kiriman tidak ditemukan.")
		return
	} else if err != nil {
		a.galat(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

// POST /api/kas/tunai {alamat, periode: []} — cash a resident handed to the
// bendahara. Same month rules as a transfer; counted at once.
func (a *App) postTunai(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct {
		Alamat  string `json:"alamat"`
		Periode []int  `json:"periode"`
	}
	if !a.baca(c, &req) {
		return
	}
	ctx := c.Context()
	d, err := a.Store.DasarRumah(ctx, req.Alamat)
	if errors.Is(err, store.ErrTidakAda) {
		a.gagal(c, 404, "tidak-ada", "Rumah tidak ditemukan.")
		return
	} else if err != nil {
		a.galat(c, err)
		return
	}
	sekarang := a.sekarang()
	_, hs := d.Hitung(sekarang)
	bulan, alasan := validasiBulan(hs[0], req.Periode, sekarang)
	if alasan != "" {
		a.gagal(c, 400, "bulan", alasan)
		return
	}
	id, err := a.Store.CatatKiriman(ctx, store.KirimanBaru{Alamat: req.Alamat, Metode: "tunai", Waktu: a.Now(),
		Petugas: p.Nama, Bulan: bulan})
	if errors.Is(err, store.ErrSudahDibayar) {
		a.gagal(c, 409, "bulan", "Sebagian bulan sudah dibayar atau sedang diverifikasi — muat ulang.")
		return
	} else if err != nil {
		a.galat(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]any{"ok": true, "kirimanId": id, "total": total(bulan)})
}

// POST /api/kas/setoran {nominal} — cash into the bank (negative = withdrawn).
func (a *App) postSetoran(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct {
		Nominal int64 `json:"nominal"`
	}
	if !a.baca(c, &req) {
		return
	}
	if req.Nominal == 0 {
		a.gagal(c, 400, "nominal", "Nominal tidak boleh 0.")
		return
	}
	if err := a.Store.Setor(c.Context(), req.Nominal, p.Nama); err != nil {
		a.galat(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

// POST /api/kas/pengeluaran (multipart: keterangan, nominal, sumber, nota?)
func (a *App) postPengeluaran(c *fibhttp.Context, p *auth.Pengurus) {
	ctx := c.Context()
	if err := c.Request.ParseMultipartForm(berkas.MaksUkuran + (1 << 20)); err != nil {
		a.gagal(c, 400, "request", "Unggahan tidak terbaca (maks. 10 MB).")
		return
	}
	v := c.Request.MultipartForm.Value
	ambil := func(k string) string {
		if len(v[k]) > 0 {
			return strings.TrimSpace(v[k][0])
		}
		return ""
	}
	nominal, err := strconv.ParseInt(ambil("nominal"), 10, 64)
	ket, sumber := ambil("keterangan"), ambil("sumber")
	switch {
	case ket == "" || len(ket) > 200:
		a.gagal(c, 400, "keterangan", "Isi keterangan (maks. 200 huruf).")
		return
	case err != nil || nominal <= 0:
		a.gagal(c, 400, "nominal", "Nominal harus lebih dari 0.")
		return
	case sumber != "kas" && sumber != "bank":
		a.gagal(c, 400, "sumber", "Sumber harus kas atau bank.")
		return
	}
	notaRef := ""
	if f, _, err := c.Request.FormFile("nota"); err == nil {
		data, err := io.ReadAll(io.LimitReader(f, berkas.MaksUkuran+1))
		f.Close()
		if err != nil {
			a.galat(c, err)
			return
		}
		mime, err := berkas.Periksa(data)
		if err != nil {
			a.gagal(c, 400, "nota", err.Error())
			return
		}
		if notaRef, err = a.Berkas.Simpan(ctx, fmt.Sprintf("nota-%s", a.Now().In(store.WIB).Format("20060102-150405")), mime, data); err != nil {
			a.galat(c, err)
			return
		}
	}
	if err := a.Store.Keluar(ctx, store.PengeluaranBaru{Keterangan: ket, Nominal: nominal, Sumber: sumber,
		Oleh: p.Nama, NotaURL: notaRef}); err != nil {
		a.galat(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) getPengeluaran(c *fibhttp.Context, _ *auth.Pengurus) {
	list, err := a.Store.DaftarPengeluaran(c.Context(), 200)
	if err != nil {
		a.galat(c, err)
		return
	}
	if list == nil {
		list = []store.Pengeluaran{}
	}
	a.json(c, 200, map[string]any{"ok": true, "pengeluaran": list})
}

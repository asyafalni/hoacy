// Package store is every SQL query the server runs. Reads that one screen
// needs go out as one pgx Batch — a single round trip to the database, which
// sits a TLS hop away.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/asyafalni/hoacy/backend/internal/tagihan"
)

var ErrTidakAda = errors.New("tidak ditemukan")

type Store struct{ DB *pgxpool.Pool }

// WIB is the cluster's time zone: "which month is it" is always asked here.
var WIB = func() *time.Location {
	l, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return l
}()

// Sekarang is this month as a periode, in WIB.
func Sekarang(t time.Time) int {
	t = t.In(WIB)
	return t.Year()*100 + int(t.Month())
}

// ── Master data + paid months: what every house computation needs ──────────

type RumahRow struct {
	Alamat  string  `json:"alamat"`
	Cluster string  `json:"cluster"`
	Blok    string  `json:"blok"`
	Nomor   string  `json:"rumah"`
	Nama    string  `json:"nama"`
	Telp    *string `json:"telp,omitempty"`
}

// Dasar is the active houses, their history, the rate cards and which months
// each house has sah/pending — enough for tagihan.Hitung.
type Dasar struct {
	Rumah   []RumahRow
	Riwayat []tagihan.Riwayat
	Tarif   []tagihan.TarifRow
	Lunas   map[string][]int
	Pending map[string][]int
	Warna   map[string]string // blok → color
}

// queueDasar adds Dasar's queries to b; only = "" for every house, or one alamat.
func queueDasar(b *pgx.Batch, d *Dasar, only string) {
	d.Lunas, d.Pending, d.Warna = map[string][]int{}, map[string][]int{}, map[string]string{}
	b.Queue(`select alamat, cluster, blok, nomor, nama, telp from rumah
	          where nonaktif_at is null and ($1 = '' or alamat = $1) order by alamat`, only).
		Query(func(rows pgx.Rows) error {
			for rows.Next() {
				var r RumahRow
				if err := rows.Scan(&r.Alamat, &r.Cluster, &r.Blok, &r.Nomor, &r.Nama, &r.Telp); err != nil {
					return err
				}
				d.Rumah = append(d.Rumah, r)
			}
			return rows.Err()
		})
	b.Queue(`select alamat, berlaku, luas::float8, tipe::text from rumah_riwayat
	          where $1 = '' or alamat = $1`, only).
		Query(func(rows pgx.Rows) error {
			for rows.Next() {
				var r tagihan.Riwayat
				if err := rows.Scan(&r.Alamat, &r.Berlaku, &r.Luas, &r.Tipe); err != nil {
					return err
				}
				d.Riwayat = append(d.Riwayat, r)
			}
			return rows.Err()
		})
	b.Queue(`select berlaku, komponen::text, luas_min::float8, nominal from tarif_versi`).
		Query(func(rows pgx.Rows) error {
			for rows.Next() {
				var r tagihan.TarifRow
				if err := rows.Scan(&r.Berlaku, &r.Komponen, &r.LuasMin, &r.Nominal); err != nil {
					return err
				}
				d.Tarif = append(d.Tarif, r)
			}
			return rows.Err()
		})
	b.Queue(`select distinct alamat, periode, keabsahan from pembayaran
	          where keabsahan in ('sah', 'pending') and ($1 = '' or alamat = $1)`, only).
		Query(func(rows pgx.Rows) error {
			for rows.Next() {
				var a, k string
				var p int
				if err := rows.Scan(&a, &p, &k); err != nil {
					return err
				}
				if k == "sah" {
					d.Lunas[a] = append(d.Lunas[a], p)
				} else {
					d.Pending[a] = append(d.Pending[a], p)
				}
			}
			return rows.Err()
		})
	b.Queue(`select kode, coalesce(warna, '') from blok`).Query(func(rows pgx.Rows) error {
		for rows.Next() {
			var k, w string
			if err := rows.Scan(&k, &w); err != nil {
				return err
			}
			if w != "" {
				d.Warna[k] = w
			}
		}
		return rows.Err()
	})
}

// Hitung resolves every house in d as of sekarang.
func (d *Dasar) Hitung(sekarang int) (*tagihan.Tarif, []*tagihan.Rumah) {
	t := tagihan.NewTarif(d.Riwayat, d.Tarif)
	out := make([]*tagihan.Rumah, len(d.Rumah))
	for i, r := range d.Rumah {
		out[i] = tagihan.Hitung(t, r.Alamat, d.Lunas[r.Alamat], d.Pending[r.Alamat], sekarang)
	}
	return t, out
}

func (s *Store) kirim(ctx context.Context, b *pgx.Batch) error {
	return s.DB.SendBatch(ctx, b).Close()
}

// DasarRumah loads one active house (ErrTidakAda if there is none).
func (s *Store) DasarRumah(ctx context.Context, alamat string) (*Dasar, error) {
	var d Dasar
	b := &pgx.Batch{}
	queueDasar(b, &d, alamat)
	if err := s.kirim(ctx, b); err != nil {
		return nil, err
	}
	if len(d.Rumah) == 0 {
		return nil, ErrTidakAda
	}
	return &d, nil
}

// ── Balances ───────────────────────────────────────────────────────────────

// Saldo: opening balances (M-SaldoAwal snapshot) + sah cash/transfers ± setoran
// − pengeluaran — the same sums D-API made. Impor never moves money: it is
// already inside the opening balances.
type Saldo struct {
	Kas      int64 `json:"kas"`
	Rekening int64 `json:"rekening"`
}

const saldoSQL = `
with awal as (
  select coalesce((select (data->>'kas')::bigint from sheet_snapshot where nama = 'M-SaldoAwal'), 0) as kas,
         coalesce((select (data->>'rekening')::bigint from sheet_snapshot where nama = 'M-SaldoAwal'), 0) as rek
), masuk as (
  select coalesce(sum(nominal) filter (where metode = 'tunai'), 0) as tunai,
         coalesce(sum(nominal) filter (where metode = 'transfer'), 0) as transfer
    from pembayaran where keabsahan = 'sah'
), setor as (select coalesce(sum(nominal), 0) as n from setoran),
keluar as (
  select coalesce(sum(nominal) filter (where sumber = 'kas'), 0) as kas,
         coalesce(sum(nominal) filter (where sumber = 'bank'), 0) as bank
    from pengeluaran
)
select (select kas from awal) + masuk.tunai - setor.n - keluar.kas,
       (select rek from awal) + masuk.transfer + setor.n - keluar.bank
  from masuk, setor, keluar`

func queueSaldo(b *pgx.Batch, out *Saldo) {
	b.Queue(saldoSQL).QueryRow(func(row pgx.Row) error { return row.Scan(&out.Kas, &out.Rekening) })
}

// ── Public page ────────────────────────────────────────────────────────────

// Publik loads everything /sum needs in one round trip.
func (s *Store) Publik(ctx context.Context, sekarang int) (tagihan.Publik, error) {
	var d Dasar
	var saldo Saldo
	var terkumpul []tagihan.Terkumpul
	var opex []tagihan.Opex
	b := &pgx.Batch{}
	queueDasar(b, &d, "")
	queueSaldo(b, &saldo)
	// 120 months back, like D-Riwayat; empty months are 0
	b.Queue(`with bulan as (
	           select (extract(year from m) * 100 + extract(month from m))::int as p
	             from generate_series(make_date($1 / 100, $1 % 100, 1) - interval '119 months',
	                                  make_date($1 / 100, $1 % 100, 1), interval '1 month') m)
	         select p, coalesce((select sum(nominal) from pembayaran where keabsahan = 'sah' and periode = p), 0)
	           from bulan order by p`, sekarang).
		Query(func(rows pgx.Rows) error {
			for rows.Next() {
				var p int
				var n int64
				if err := rows.Scan(&p, &n); err != nil {
					return err
				}
				terkumpul = append(terkumpul, tagihan.Terkumpul{Tahun: p / 100, Bulan: p % 100, Terkumpul: n})
			}
			return rows.Err()
		})
	queueOpex(b, &opex)
	if err := s.kirim(ctx, b); err != nil {
		return tagihan.Publik{}, err
	}
	t, houses := d.Hitung(sekarang)
	return tagihan.RingkasanPublik(t, houses, terkumpul, opex, saldo.Kas, saldo.Rekening, sekarang), nil
}

func queueOpex(b *pgx.Batch, out *[]tagihan.Opex) {
	b.Queue(`select coalesce(e->>'kategori', ''), coalesce(e->>'ikon', ''), coalesce((e->>'nominal')::bigint, 0),
	                coalesce(e->>'diperbarui', '')
	           from sheet_snapshot, jsonb_array_elements(data) e where nama = 'M-Opex'`).
		Query(func(rows pgx.Rows) error {
			for rows.Next() {
				var o tagihan.Opex
				if err := rows.Scan(&o.Kategori, &o.Ikon, &o.Nominal, &o.Diperbarui); err != nil {
					return err
				}
				*out = append(*out, o)
			}
			return rows.Err()
		})
}

// ── Kas screen ─────────────────────────────────────────────────────────────

// Bayar is one row of the pembayaran view.
type Bayar struct {
	KirimanID int64      `json:"kirimanId"`
	Alamat    string     `json:"alamat"`
	Periode   int        `json:"periode"`
	Nominal   int64      `json:"nominal"`
	Metode    string     `json:"metode"`
	Waktu     *time.Time `json:"waktu"`
	Petugas   string     `json:"petugas"`
	AdaBukti  bool       `json:"adaBukti"`
	Keabsahan string     `json:"keabsahan"`
}

func scanBayar(rows pgx.Rows, out *[]Bayar) error {
	for rows.Next() {
		var r Bayar
		var bukti *string
		if err := rows.Scan(&r.KirimanID, &r.Alamat, &r.Periode, &r.Nominal, &r.Metode, &r.Waktu,
			&r.Petugas, &bukti, &r.Keabsahan); err != nil {
			return err
		}
		r.AdaBukti = bukti != nil && *bukti != ""
		*out = append(*out, r)
	}
	return rows.Err()
}

const bayarCols = `kiriman_id, alamat, periode, nominal, metode::text, waktu, petugas, bukti_url, keabsahan`

type Kas struct {
	Dasar    *Dasar
	Saldo    Saldo
	Antrian  []Bayar // pending + cek, any year
	KasMasuk []Bayar // cash received this year and last, incl. tolak/dobel (audit)
}

func (s *Store) Kas(ctx context.Context, sekarang int) (*Kas, error) {
	k := &Kas{Dasar: &Dasar{}}
	b := &pgx.Batch{}
	queueDasar(b, k.Dasar, "")
	queueSaldo(b, &k.Saldo)
	b.Queue(`select ` + bayarCols + ` from pembayaran where keabsahan in ('pending', 'cek')
	          order by waktu desc nulls last`).Query(func(rows pgx.Rows) error { return scanBayar(rows, &k.Antrian) })
	b.Queue(`select `+bayarCols+` from pembayaran where metode = 'tunai'
	          and waktu >= make_date($1 / 100 - 1, 1, 1)::timestamp at time zone 'Asia/Jakarta'
	          order by waktu desc`, sekarang).Query(func(rows pgx.Rows) error { return scanBayar(rows, &k.KasMasuk) })
	if err := s.kirim(ctx, b); err != nil {
		return nil, err
	}
	return k, nil
}

// Tahun is one dues year's rows (sah only matter for the summary).
func (s *Store) Tahun(ctx context.Context, tahun int) ([]tagihan.Baris, error) {
	rows, err := s.DB.Query(ctx, `select alamat, bulan, nominal, metode::text, keabsahan
	                               from pembayaran where tahun = $1`, tahun)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (tagihan.Baris, error) {
		var b tagihan.Baris
		err := r.Scan(&b.Alamat, &b.Bulan, &b.Nominal, &b.Metode, &b.Keabsahan)
		return b, err
	})
}

// BuktiKiriman is the stored proof of a kiriman ("" when there is none).
func (s *Store) BuktiKiriman(ctx context.Context, id int64) (string, error) {
	var u *string
	err := s.DB.QueryRow(ctx, `select bukti_url from kiriman where id = $1`, id).Scan(&u)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (u == nil || *u == "")) {
		return "", ErrTidakAda
	}
	if err != nil {
		return "", err
	}
	return *u, nil
}

// ── Writes ─────────────────────────────────────────────────────────────────

// KirimanBaru records one payment: all its months at once.
type KirimanBaru struct {
	Alamat   string
	Metode   string // tunai | transfer
	Waktu    time.Time
	Petugas  string
	BuktiURL string
	Bulan    []tagihan.Bulanan
}

// CatatKiriman inserts a payment. The caller has already checked every month
// is billable, unpaid and at its tarif; the row lock on the house serializes
// two submissions for it, and the re-check inside the transaction makes sure
// neither double-pays a month.
func (s *Store) CatatKiriman(ctx context.Context, k KirimanBaru) (int64, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `select 1 from rumah where alamat = $1 for update`, k.Alamat); err != nil {
		return 0, err
	}
	periodes := make([]int, len(k.Bulan))
	var total int64
	for i, b := range k.Bulan {
		periodes[i] = b.Periode
		total += b.Tarif
	}
	var sudah []int
	rows, err := tx.Query(ctx, `select distinct periode from pembayaran
	                             where alamat = $1 and periode = any($2) and keabsahan in ('sah', 'pending')`,
		k.Alamat, periodes)
	if err != nil {
		return 0, err
	}
	if sudah, err = pgx.CollectRows(rows, pgx.RowTo[int]); err != nil {
		return 0, err
	}
	if len(sudah) > 0 {
		return 0, fmt.Errorf("%w: %v", ErrSudahDibayar, sudah)
	}
	var id int64
	var bukti *string
	if k.BuktiURL != "" {
		bukti = &k.BuktiURL
	}
	if err := tx.QueryRow(ctx, `insert into kiriman (alamat, metode, waktu, total, petugas, bukti_url)
	                            values ($1, $2, $3, $4, $5, $6) returning id`,
		k.Alamat, k.Metode, k.Waktu, total, k.Petugas, bukti).Scan(&id); err != nil {
		return 0, err
	}
	for _, b := range k.Bulan {
		if _, err := tx.Exec(ctx, `insert into kiriman_bulan values ($1, $2, $3)`, id, b.Periode, b.Tarif); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit(ctx)
}

var ErrSudahDibayar = errors.New("bulan sudah dibayar atau menunggu verifikasi")

// Putuskan records a verdict on a kiriman (latest wins).
func (s *Store) Putuskan(ctx context.Context, kirimanID int64, keputusan, oleh string) error {
	tag, err := s.DB.Exec(ctx, `insert into keputusan (kiriman_id, keputusan, oleh)
	                            select id, $2, $3 from kiriman where id = $1 and metode <> 'impor'`,
		kirimanID, keputusan, oleh)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrTidakAda
	}
	return err
}

func (s *Store) Setor(ctx context.Context, nominal int64, oleh string) error {
	_, err := s.DB.Exec(ctx, `insert into setoran (nominal, oleh) values ($1, $2)`, nominal, oleh)
	return err
}

type PengeluaranBaru struct {
	Keterangan string
	Nominal    int64
	Sumber     string // kas | bank
	Oleh       string
	NotaURL    string
}

func (s *Store) Keluar(ctx context.Context, p PengeluaranBaru) error {
	var nota *string
	if p.NotaURL != "" {
		nota = &p.NotaURL
	}
	_, err := s.DB.Exec(ctx, `insert into pengeluaran (keterangan, nominal, sumber, oleh, nota_url)
	                          values ($1, $2, $3, $4, $5)`, p.Keterangan, p.Nominal, p.Sumber, p.Oleh, nota)
	return err
}

type Pengeluaran struct {
	ID         int64     `json:"id"`
	Keterangan string    `json:"keterangan"`
	Nominal    int64     `json:"nominal"`
	Sumber     string    `json:"sumber"`
	Oleh       string    `json:"oleh"`
	AdaNota    bool      `json:"adaNota"`
	Waktu      time.Time `json:"waktu"`
}

func (s *Store) DaftarPengeluaran(ctx context.Context, batas int) ([]Pengeluaran, error) {
	rows, err := s.DB.Query(ctx, `select id, keterangan, nominal, sumber::text, oleh, coalesce(nota_url, '') <> '', waktu
	                               from pengeluaran order by waktu desc limit $1`, batas)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Pengeluaran, error) {
		var p Pengeluaran
		err := r.Scan(&p.ID, &p.Keterangan, &p.Nominal, &p.Sumber, &p.Oleh, &p.AdaNota, &p.Waktu)
		return p, err
	})
}

func (s *Store) NotaPengeluaran(ctx context.Context, id int64) (string, error) {
	var u *string
	err := s.DB.QueryRow(ctx, `select nota_url from pengeluaran where id = $1`, id).Scan(&u)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (u == nil || *u == "")) {
		return "", ErrTidakAda
	}
	if err != nil {
		return "", err
	}
	return *u, nil
}

// ── Sheet snapshots ────────────────────────────────────────────────────────

func (s *Store) SimpanSnapshot(ctx context.Context, nama string, data any) error {
	_, err := s.DB.Exec(ctx, `insert into sheet_snapshot (nama, data) values ($1, $2)
	                          on conflict (nama) do update set data = excluded.data, dimuat_at = now()`, nama, data)
	return err
}

func (s *Store) WaktuSnapshot(ctx context.Context, nama string) (time.Time, error) {
	var t time.Time
	err := s.DB.QueryRow(ctx, `select dimuat_at from sheet_snapshot where nama = $1`, nama).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	return t, err
}

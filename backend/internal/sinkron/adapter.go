package sinkron

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/asyafalni/hoacy/backend/internal/auth"
)

// Column positions follow docs/sheets-schema.md (position, not header text,
// is what the Sheet formulas relied on too).

// unik makes duplicate keys distinct ("…#2"): a row typed twice in the Sheet
// is two rows here too, and the pembayaran view marks the second `dobel`.
func unik(seen map[string]int, k string) string {
	seen[k]++
	if n := seen[k]; n > 1 {
		return k + "#" + strconv.Itoa(n)
	}
	return k
}

// ── M-Blok: A blok · B warna ───────────────────────────────────────────────

func adapterBlok() *adapter {
	return &adapter{master: true,
		dariSheet: func(rows [][]string, _ konteks) ([]rekam, []Galat) {
			var out []rekam
			var galat []Galat
			body, nomor := data(rows)
			for i, r := range body {
				kode, warna := sel(r, 0), sel(r, 1)
				if kode == "" {
					continue
				}
				if warna != "" && !strings.HasPrefix(warna, "#") {
					galat = append(galat, Galat{nomor[i], "warna harus #rrggbb"})
					continue
				}
				out = append(out, rekam{kode, "Blok " + kode + " " + warna, warna, warna})
			}
			return out, galat
		},
		dariDB: func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error) {
			return kumpul(ctx, tx, `select kode, coalesce(warna, '') from blok`, func(r pgx.Rows) (rekam, error) {
				var k, w string
				err := r.Scan(&k, &w)
				return rekam{k, "Blok " + k + " " + w, w, w}, err
			})
		},
		simpan: func(ctx context.Context, tx pgx.Tx, r rekam, _ bool) error {
			_, err := tx.Exec(ctx, `insert into blok values ($1, nullif($2, ''))
			                        on conflict (kode) do update set warna = excluded.warna`, r.kunci, r.data)
			return err
		},
		hapus: func(ctx context.Context, tx pgx.Tx, k string) error {
			_, err := tx.Exec(ctx, `delete from blok where kode = $1`, k)
			return err
		},
	}
}

// ── M-Rumah: A alamat · B cluster · C blok · D rumah · E nama · F telp · G pin
//    H konfirmasi_nonaktif · I dinonaktifkan_oleh · J tanggal_nonaktif · K aktif
// The old 3-digit PINs are not carried over: a new house gets the last 6
// digits of its phone (or a random PIN the admin must reset), and a re-sync
// never touches an existing house's PIN.

type rumahSheet struct {
	alamat, cluster, blok, nomor, nama, telp string
	nonaktif, oleh                           string // date yyyy-mm-dd, who
}

func (r rumahSheet) sidik() string {
	return strings.Join([]string{r.cluster, r.blok, r.nomor, r.nama, r.telp, r.nonaktif, r.oleh}, "|")
}

func (r rumahSheet) ringkas() string {
	s := r.alamat + " · " + r.nama
	if r.telp != "" {
		s += " · " + r.telp
	}
	if r.nonaktif != "" {
		s += " · nonaktif " + r.nonaktif + " (" + r.oleh + ")"
	}
	return s
}

func adapterRumah() *adapter {
	return &adapter{master: true,
		dariSheet: func(rows [][]string, _ konteks) ([]rekam, []Galat) {
			var out []rekam
			var galat []Galat
			body, nomor := data(rows)
			seen := map[string]bool{}
			for i, r := range body {
				if sel(r, 1) == "" && sel(r, 2) == "" {
					continue
				}
				h := rumahSheet{cluster: sel(r, 1), blok: sel(r, 2), nomor: sel(r, 3), nama: sel(r, 4), telp: sel(r, 5)}
				if len(h.nomor) == 1 {
					h.nomor = "0" + h.nomor
				}
				h.alamat = h.cluster + h.blok + "-" + h.nomor
				if a := sel(r, 0); a != "" && a != h.alamat {
					galat = append(galat, Galat{nomor[i], fmt.Sprintf("alamat %s tidak cocok dengan cluster/blok/rumah (%s)", a, h.alamat)})
					continue
				}
				if seen[h.alamat] {
					galat = append(galat, Galat{nomor[i], h.alamat + " muncul dua kali"})
					continue
				}
				seen[h.alamat] = true
				if h.telp != "" && !strings.HasPrefix(h.telp, "62") {
					galat = append(galat, Galat{nomor[i], h.alamat + ": telp harus berawalan 62"})
					continue
				}
				if strings.EqualFold(sel(r, 10), "false") {
					t, err := waktu(sel(r, 9))
					if err != nil || sel(r, 8) == "" {
						galat = append(galat, Galat{nomor[i], h.alamat + ": nonaktif tanpa tanggal/oleh yang lengkap"})
						continue
					}
					h.nonaktif, h.oleh = t.Format("2006-01-02"), sel(r, 8)
				}
				out = append(out, rekam{h.alamat, h.ringkas(), h.sidik(), h})
			}
			return out, galat
		},
		dariDB: func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error) {
			return kumpul(ctx, tx, `select alamat, cluster, blok, nomor, nama, coalesce(telp, ''),
			                               coalesce(to_char(nonaktif_at, 'YYYY-MM-DD'), ''), coalesce(nonaktif_oleh, '')
			                          from rumah`, func(r pgx.Rows) (rekam, error) {
				var h rumahSheet
				err := r.Scan(&h.alamat, &h.cluster, &h.blok, &h.nomor, &h.nama, &h.telp, &h.nonaktif, &h.oleh)
				return rekam{h.alamat, h.ringkas(), h.sidik(), h}, err
			})
		},
		simpan: func(ctx context.Context, tx pgx.Tx, r rekam, baru bool) error {
			h := r.data.(rumahSheet)
			if _, err := tx.Exec(ctx, `insert into blok (kode) values ($1) on conflict do nothing`, h.blok); err != nil {
				return err
			}
			var telp, tgl, oleh any
			if h.telp != "" {
				telp = h.telp
			}
			if h.nonaktif != "" {
				tgl, oleh = h.nonaktif, h.oleh
			}
			if !baru {
				_, err := tx.Exec(ctx, `update rumah set cluster = $2, blok = $3, nomor = $4, nama = $5, telp = $6,
				                        nonaktif_at = $7, nonaktif_oleh = $8 where alamat = $1`,
					h.alamat, h.cluster, h.blok, h.nomor, h.nama, telp, tgl, oleh)
				return err
			}
			pin, ok := auth.PINBawaan(h.telp)
			if !ok {
				pin = acakPIN()
			}
			hash, err := auth.HashPIN(pin)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `insert into rumah (alamat, cluster, blok, nomor, nama, telp, pin_hash, nonaktif_at, nonaktif_oleh)
			                       values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				h.alamat, h.cluster, h.blok, h.nomor, h.nama, telp, hash, tgl, oleh)
			return err
		},
		hapus: func(ctx context.Context, tx pgx.Tx, k string) error {
			_, err := tx.Exec(ctx, `delete from rumah where alamat = $1`, k) // refused by FKs once it has any history
			return err
		},
	}
}

func acakPIN() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return fmt.Sprintf("%06d", n.Int64())
}

// ── M-RumahRiwayat: A alamat · B tahun · C bulan · D luas · E tipe ─────────

type riwayatSheet struct {
	alamat  string
	berlaku int
	luas    float64
	tipe    string
}

func (r riwayatSheet) sidik() string { return strconv.FormatFloat(r.luas, 'f', 2, 64) + "|" + r.tipe }
func (r riwayatSheet) ringkas() string {
	return fmt.Sprintf("%s mulai %d: %s m² %s", r.alamat, r.berlaku, strconv.FormatFloat(r.luas, 'f', -1, 64), r.tipe)
}

func adapterRiwayat() *adapter {
	return &adapter{master: true,
		dariSheet: func(rows [][]string, k konteks) ([]rekam, []Galat) {
			var out []rekam
			var galat []Galat
			body, nomor := data(rows)
			for i, r := range body {
				if kosong(r) {
					continue
				}
				x := riwayatSheet{alamat: sel(r, 0), tipe: strings.ToLower(sel(r, 4))}
				th, e1 := bulat(sel(r, 1))
				bl, e2 := bulat(sel(r, 2))
				luas, e3 := angka(sel(r, 3))
				switch {
				case !k.alamat[x.alamat]:
					galat = append(galat, Galat{nomor[i], "alamat " + x.alamat + " belum ada (sinkron M-Rumah dulu)"})
				case e1 != nil || e2 != nil || bl < 1 || bl > 12:
					galat = append(galat, Galat{nomor[i], x.alamat + ": tahun/bulan tidak valid"})
				case e3 != nil || luas <= 0:
					galat = append(galat, Galat{nomor[i], x.alamat + ": luas tidak valid"})
				case x.tipe != "rumah" && x.tipe != "kavling":
					galat = append(galat, Galat{nomor[i], x.alamat + ": tipe harus rumah atau kavling"})
				default:
					x.berlaku, x.luas = th*100+bl, luas
					out = append(out, rekam{fmt.Sprintf("%s|%d", x.alamat, x.berlaku), x.ringkas(), x.sidik(), x})
				}
			}
			return out, galat
		},
		dariDB: func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error) {
			return kumpul(ctx, tx, `select alamat, berlaku, luas::float8, tipe::text from rumah_riwayat`, func(r pgx.Rows) (rekam, error) {
				var x riwayatSheet
				err := r.Scan(&x.alamat, &x.berlaku, &x.luas, &x.tipe)
				return rekam{fmt.Sprintf("%s|%d", x.alamat, x.berlaku), x.ringkas(), x.sidik(), x}, err
			})
		},
		simpan: func(ctx context.Context, tx pgx.Tx, r rekam, _ bool) error {
			x := r.data.(riwayatSheet)
			_, err := tx.Exec(ctx, `insert into rumah_riwayat values ($1, $2, $3, $4)
			                        on conflict (alamat, berlaku) do update set luas = excluded.luas, tipe = excluded.tipe`,
				x.alamat, x.berlaku, x.luas, x.tipe)
			return err
		},
		hapus: func(ctx context.Context, tx pgx.Tx, k string) error {
			a, b, _ := strings.Cut(k, "|")
			_, err := tx.Exec(ctx, `delete from rumah_riwayat where alamat = $1 and berlaku = $2::int`, a, b)
			return err
		},
	}
}

// ── M-TarifVersi: A tahun · B bulan · C komponen · D luas_min · E nominal ───

type tarifSheet struct {
	berlaku  int
	komponen string
	luasMin  float64
	nominal  int64
}

func (t tarifSheet) kunci() string {
	return fmt.Sprintf("%d|%s|%s", t.berlaku, t.komponen, strconv.FormatFloat(t.luasMin, 'f', -1, 64))
}
func (t tarifSheet) ringkas() string {
	s := fmt.Sprintf("%d %s", t.berlaku, t.komponen)
	if t.komponen == "islk_rumah" {
		s += fmt.Sprintf(" ≥%s m²", strconv.FormatFloat(t.luasMin, 'f', -1, 64))
	}
	return s + fmt.Sprintf(" = %d", t.nominal)
}

func adapterTarif() *adapter {
	return &adapter{master: true,
		dariSheet: func(rows [][]string, _ konteks) ([]rekam, []Galat) {
			var out []rekam
			var galat []Galat
			body, nomor := data(rows)
			for i, r := range body {
				if kosong(r) {
					continue
				}
				t := tarifSheet{komponen: strings.ToLower(sel(r, 2))}
				th, e1 := bulat(sel(r, 0))
				bl, e2 := bulat(sel(r, 1))
				n, e3 := rupiah(sel(r, 4))
				if s := sel(r, 3); s != "" {
					t.luasMin, _ = angka(s)
				}
				switch {
				case e1 != nil || e2 != nil || bl < 1 || bl > 12:
					galat = append(galat, Galat{nomor[i], "tahun/bulan tidak valid"})
				case t.komponen != "islk_rumah" && t.komponen != "islk_kavling" && t.komponen != "iuran_rt":
					galat = append(galat, Galat{nomor[i], "komponen tidak dikenal: " + t.komponen})
				case e3 != nil:
					galat = append(galat, Galat{nomor[i], "nominal tidak valid"})
				default:
					if t.komponen != "islk_rumah" {
						t.luasMin = 0
					}
					t.berlaku, t.nominal = th*100+bl, n
					out = append(out, rekam{t.kunci(), t.ringkas(), strconv.FormatInt(n, 10), t})
				}
			}
			return out, galat
		},
		dariDB: func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error) {
			return kumpul(ctx, tx, `select berlaku, komponen::text, luas_min::float8, nominal from tarif_versi`, func(r pgx.Rows) (rekam, error) {
				var t tarifSheet
				err := r.Scan(&t.berlaku, &t.komponen, &t.luasMin, &t.nominal)
				return rekam{t.kunci(), t.ringkas(), strconv.FormatInt(t.nominal, 10), t}, err
			})
		},
		simpan: func(ctx context.Context, tx pgx.Tx, r rekam, _ bool) error {
			t := r.data.(tarifSheet)
			_, err := tx.Exec(ctx, `insert into tarif_versi values ($1, $2, $3, $4)
			                        on conflict (berlaku, komponen, luas_min) do update set nominal = excluded.nominal`,
				t.berlaku, t.komponen, t.luasMin, t.nominal)
			return err
		},
		hapus: func(ctx context.Context, tx pgx.Tx, k string) error {
			p := strings.SplitN(k, "|", 3)
			if len(p) != 3 {
				return fmt.Errorf("kunci tarif tidak valid")
			}
			_, err := tx.Exec(ctx, `delete from tarif_versi where berlaku = $1::int and komponen = $2 and luas_min = $3::numeric`, p[0], p[1], p[2])
			return err
		},
	}
}

// ── Payments: M-Impor<tahun>, L-Tunai, L-Transfer → kiriman ────────────────

type kirimanSheet struct {
	ref, alamat, metode, petugas, bukti string
	waktu                               *time.Time
	total                               int64
	bulan                               [][2]int64
}

func (k kirimanSheet) teksRincian() string {
	b := append([][2]int64(nil), k.bulan...)
	sort.Slice(b, func(i, j int) bool { return b[i][0] < b[j][0] })
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("%d=%d", x[0], x[1])
	}
	return strings.Join(parts, ",")
}

func (k kirimanSheet) teksWaktu() string {
	if k.waktu == nil {
		return ""
	}
	return teksWaktu(*k.waktu)
}

func (k kirimanSheet) sidik() string {
	return strings.Join([]string{k.alamat, k.teksWaktu(), strconv.FormatInt(k.total, 10), k.petugas, k.bukti, k.teksRincian()}, "|")
}

func (k kirimanSheet) ringkas() string {
	s := k.alamat + " · " + k.teksRincian()
	if w := k.teksWaktu(); w != "" {
		s += " · " + w
	}
	return s
}

func adapterImpor(tab string, tahun int) *adapter {
	a := adapterKirimanDB(tab)
	a.dariSheet = func(rows [][]string, k konteks) ([]rekam, []Galat) {
		var out []rekam
		var galat []Galat
		seen := map[string]int{}
		body, nomor := data(rows)
		for i, r := range body {
			if kosong(r) {
				continue
			}
			x := kirimanSheet{alamat: sel(r, 0), metode: "impor", petugas: "Impor"}
			bl, e1 := bulat(sel(r, 1))
			n, e2 := rupiah(sel(r, 2))
			switch {
			case !k.alamat[x.alamat]:
				galat = append(galat, Galat{nomor[i], "alamat " + x.alamat + " belum ada"})
				continue
			case e1 != nil || bl < 1 || bl > 12:
				galat = append(galat, Galat{nomor[i], x.alamat + ": bulan harus 1–12"})
				continue
			case e2 != nil || n < 0:
				galat = append(galat, Galat{nomor[i], x.alamat + ": nominal tidak valid"})
				continue
			}
			if s := sel(r, 3); s != "" {
				t, err := waktu(s)
				if err != nil {
					galat = append(galat, Galat{nomor[i], x.alamat + ": " + err.Error()})
					continue
				}
				x.waktu = &t
			}
			x.total, x.bulan = n, [][2]int64{{int64(tahun*100 + bl), n}}
			x.ref = unik(seen, fmt.Sprintf("%s:%s:%d", tab, x.alamat, bl))
			out = append(out, rekam{x.ref, x.ringkas(), x.sidik(), x})
		}
		return out, galat
	}
	return a
}

func adapterKiriman(tab, metode string) *adapter {
	a := adapterKirimanDB(tab)
	a.dariSheet = func(rows [][]string, k konteks) ([]rekam, []Galat) {
		var out []rekam
		var galat []Galat
		seen := map[string]int{}
		body, nomor := data(rows)
		for i, r := range body {
			if kosong(r) {
				continue
			}
			t, err := waktu(sel(r, 0))
			if err != nil {
				galat = append(galat, Galat{nomor[i], err.Error()})
				continue
			}
			x := kirimanSheet{alamat: sel(r, 1), metode: metode, waktu: &t, bulan: rincian(sel(r, 2))}
			if !k.alamat[x.alamat] {
				galat = append(galat, Galat{nomor[i], "alamat " + x.alamat + " belum ada"})
				continue
			}
			if x.total, err = rupiah(sel(r, 3)); err != nil {
				galat = append(galat, Galat{nomor[i], x.alamat + ": total tidak valid"})
				continue
			}
			if metode == "tunai" {
				x.petugas = sel(r, 4)
			} else {
				x.petugas, x.bukti = "Warga", sel(r, 4)
			}
			x.ref = unik(seen, tab+":"+x.alamat+"|"+teksWaktu(t))
			out = append(out, rekam{x.ref, x.ringkas(), x.sidik(), x})
		}
		return out, galat
	}
	return a
}

// adapterKirimanDB is the database side shared by every payment tab: kiriman
// rows whose sumber_ref starts with "<tab>:".
func adapterKirimanDB(tab string) *adapter {
	return &adapter{
		dariDB: func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error) {
			return kumpul(ctx, tx, `
			  select k.sumber_ref, k.alamat, k.metode::text, k.petugas, coalesce(k.bukti_url, ''), k.waktu, k.total,
			         coalesce(array_agg(array[b.periode::bigint, b.nominal]) filter (where b.kiriman_id is not null), '{}')
			    from kiriman k left join kiriman_bulan b on b.kiriman_id = k.id
			   where k.sumber_ref like $1 || ':%'
			   group by k.id`, func(r pgx.Rows) (rekam, error) {
				var x kirimanSheet
				var bulan [][]int64
				err := r.Scan(&x.ref, &x.alamat, &x.metode, &x.petugas, &x.bukti, &x.waktu, &x.total, &bulan)
				for _, b := range bulan {
					x.bulan = append(x.bulan, [2]int64{b[0], b[1]})
				}
				return rekam{x.ref, x.ringkas(), x.sidik(), x}, err
			}, tab)
		},
		simpan: func(ctx context.Context, tx pgx.Tx, r rekam, baru bool) error {
			x := r.data.(kirimanSheet)
			var bukti any
			if x.bukti != "" {
				bukti = x.bukti
			}
			var id int64
			if baru {
				if err := tx.QueryRow(ctx, `insert into kiriman (alamat, metode, waktu, total, petugas, bukti_url, sumber_ref)
				                            values ($1, $2, $3, $4, $5, $6, $7) returning id`,
					x.alamat, x.metode, x.waktu, x.total, x.petugas, bukti, x.ref).Scan(&id); err != nil {
					return err
				}
			} else {
				if err := tx.QueryRow(ctx, `update kiriman set alamat = $2, waktu = $3, total = $4, petugas = $5, bukti_url = $6
				                            where sumber_ref = $1 returning id`,
					x.ref, x.alamat, x.waktu, x.total, x.petugas, bukti).Scan(&id); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `delete from kiriman_bulan where kiriman_id = $1`, id); err != nil {
					return err
				}
			}
			for _, b := range x.bulan {
				if _, err := tx.Exec(ctx, `insert into kiriman_bulan values ($1, $2, $3)`, id, b[0], b[1]); err != nil {
					return err
				}
			}
			return nil
		},
		hapus: func(ctx context.Context, tx pgx.Tx, ref string) error {
			if _, err := tx.Exec(ctx, `delete from keputusan where kiriman_id = (select id from kiriman where sumber_ref = $1)`, ref); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `delete from kiriman where sumber_ref = $1`, ref)
			return err
		},
	}
}

// ── L-Keputusan: A Timestamp · B alamat · C waktu · D keputusan · E oleh ───
// Points at a synced L-Tunai/L-Transfer submission by alamat + its timestamp.

type keputusanSheet struct {
	ref, target, keputusan, oleh string
	waktu                        time.Time
}

func (k keputusanSheet) sidik() string {
	return strings.Join([]string{k.target, k.keputusan, k.oleh, teksWaktu(k.waktu)}, "|")
}

func (k keputusanSheet) ringkas() string {
	return k.keputusan + " oleh " + k.oleh + " · " + strings.TrimPrefix(strings.TrimPrefix(k.target, "L-Tunai:"), "L-Transfer:")
}

func adapterKeputusan() *adapter {
	return &adapter{
		dariSheet: func(rows [][]string, k konteks) ([]rekam, []Galat) {
			var out []rekam
			var galat []Galat
			seen := map[string]int{}
			body, nomor := data(rows)
			for i, r := range body {
				if kosong(r) {
					continue
				}
				t, err1 := waktu(sel(r, 0))
				w, err2 := waktu(sel(r, 2))
				x := keputusanSheet{keputusan: strings.ToLower(sel(r, 3)), oleh: sel(r, 4), waktu: t}
				alamat := sel(r, 1)
				switch {
				case err1 != nil || err2 != nil:
					galat = append(galat, Galat{nomor[i], alamat + ": tanggal tidak terbaca"})
					continue
				case x.keputusan != "sah" && x.keputusan != "tolak":
					galat = append(galat, Galat{nomor[i], alamat + ": keputusan harus sah atau tolak"})
					continue
				}
				x.target = alamat + "|" + teksWaktu(w)
				x.ref = unik(seen, "L-Keputusan:"+alamat+"|"+teksWaktu(t))
				out = append(out, rekam{x.ref, x.ringkas(), x.sidik(), x})
			}
			return out, galat
		},
		dariDB: func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error) {
			return kumpul(ctx, tx, `
			  select d.sumber_ref, d.keputusan::text, d.oleh, d.waktu,
			         k.alamat || '|' || to_char(k.waktu at time zone 'Asia/Jakarta', 'YYYY-MM-DD HH24:MI:SS')
			    from keputusan d join kiriman k on k.id = d.kiriman_id
			   where d.sumber_ref like 'L-Keputusan:%'`, func(r pgx.Rows) (rekam, error) {
				var x keputusanSheet
				err := r.Scan(&x.ref, &x.keputusan, &x.oleh, &x.waktu, &x.target)
				return rekam{x.ref, x.ringkas(), x.sidik(), x}, err
			})
		},
		simpan: func(ctx context.Context, tx pgx.Tx, r rekam, baru bool) error {
			x := r.data.(keputusanSheet)
			alamat, w, _ := strings.Cut(x.target, "|")
			var id int64
			err := tx.QueryRow(ctx, `select id from kiriman where sumber_ref in ($1, $2)`,
				"L-Tunai:"+alamat+"|"+w, "L-Transfer:"+alamat+"|"+w).Scan(&id)
			if err != nil {
				return fmt.Errorf("kiriman %s %s belum disinkron — sinkron L-Tunai dan L-Transfer dulu", alamat, w)
			}
			if baru {
				_, err = tx.Exec(ctx, `insert into keputusan (kiriman_id, keputusan, oleh, waktu, sumber_ref)
				                       values ($1, $2, $3, $4, $5)`, id, x.keputusan, x.oleh, x.waktu, x.ref)
			} else {
				_, err = tx.Exec(ctx, `update keputusan set kiriman_id = $2, keputusan = $3, oleh = $4, waktu = $5
				                       where sumber_ref = $1`, x.ref, id, x.keputusan, x.oleh, x.waktu)
			}
			return err
		},
		hapus: func(ctx context.Context, tx pgx.Tx, ref string) error {
			_, err := tx.Exec(ctx, `delete from keputusan where sumber_ref = $1`, ref)
			return err
		},
	}
}

// ── L-Setoran: A Timestamp · B nominal · C oleh ────────────────────────────
// ── L-Pengeluaran: A Timestamp · B keterangan · C nominal · D sumber ──────

type uangSheet struct {
	ref, keterangan, sumber, oleh string
	nominal                       int64
	waktu                         time.Time
}

func (u uangSheet) sidik() string {
	return strings.Join([]string{u.keterangan, u.sumber, u.oleh, strconv.FormatInt(u.nominal, 10), teksWaktu(u.waktu)}, "|")
}

func (u uangSheet) ringkas() string {
	s := teksWaktu(u.waktu) + " · " + strconv.FormatInt(u.nominal, 10)
	if u.keterangan != "" {
		s += " · " + u.keterangan + " (" + u.sumber + ")"
	}
	return s + " · " + u.oleh
}

func adapterSetoran() *adapter {
	return &adapter{
		dariSheet: func(rows [][]string, _ konteks) ([]rekam, []Galat) {
			return bacaUang(rows, "L-Setoran", func(r []string, u *uangSheet) error {
				var err error
				u.nominal, err = rupiah(sel(r, 1))
				u.oleh = sel(r, 2)
				if err == nil && u.nominal == 0 {
					err = fmt.Errorf("nominal 0")
				}
				return err
			})
		},
		dariDB: func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error) {
			return kumpul(ctx, tx, `select sumber_ref, nominal, oleh, waktu from setoran where sumber_ref like 'L-Setoran:%'`,
				func(r pgx.Rows) (rekam, error) {
					var u uangSheet
					err := r.Scan(&u.ref, &u.nominal, &u.oleh, &u.waktu)
					return rekam{u.ref, u.ringkas(), u.sidik(), u}, err
				})
		},
		simpan: func(ctx context.Context, tx pgx.Tx, r rekam, baru bool) error {
			u := r.data.(uangSheet)
			var err error
			if baru {
				_, err = tx.Exec(ctx, `insert into setoran (nominal, oleh, waktu, sumber_ref) values ($1, $2, $3, $4)`,
					u.nominal, u.oleh, u.waktu, u.ref)
			} else {
				_, err = tx.Exec(ctx, `update setoran set nominal = $2, oleh = $3, waktu = $4 where sumber_ref = $1`,
					u.ref, u.nominal, u.oleh, u.waktu)
			}
			return err
		},
		hapus: func(ctx context.Context, tx pgx.Tx, ref string) error {
			_, err := tx.Exec(ctx, `delete from setoran where sumber_ref = $1`, ref)
			return err
		},
	}
}

func adapterPengeluaran() *adapter {
	return &adapter{
		dariSheet: func(rows [][]string, _ konteks) ([]rekam, []Galat) {
			return bacaUang(rows, "L-Pengeluaran", func(r []string, u *uangSheet) error {
				var err error
				u.keterangan, u.sumber, u.oleh = sel(r, 1), strings.ToLower(sel(r, 3)), "Sheet"
				if u.nominal, err = rupiah(sel(r, 2)); err != nil {
					return err
				}
				if u.nominal <= 0 {
					return fmt.Errorf("nominal harus > 0")
				}
				if u.sumber != "kas" && u.sumber != "bank" {
					return fmt.Errorf("sumber harus kas atau bank")
				}
				return nil
			})
		},
		dariDB: func(ctx context.Context, tx pgx.Tx) (map[string]rekam, error) {
			return kumpul(ctx, tx, `select sumber_ref, keterangan, sumber::text, oleh, nominal, waktu
			                          from pengeluaran where sumber_ref like 'L-Pengeluaran:%'`,
				func(r pgx.Rows) (rekam, error) {
					var u uangSheet
					err := r.Scan(&u.ref, &u.keterangan, &u.sumber, &u.oleh, &u.nominal, &u.waktu)
					return rekam{u.ref, u.ringkas(), u.sidik(), u}, err
				})
		},
		simpan: func(ctx context.Context, tx pgx.Tx, r rekam, baru bool) error {
			u := r.data.(uangSheet)
			var err error
			if baru {
				_, err = tx.Exec(ctx, `insert into pengeluaran (keterangan, nominal, sumber, oleh, waktu, sumber_ref)
				                       values ($1, $2, $3, $4, $5, $6)`, u.keterangan, u.nominal, u.sumber, u.oleh, u.waktu, u.ref)
			} else {
				_, err = tx.Exec(ctx, `update pengeluaran set keterangan = $2, nominal = $3, sumber = $4, waktu = $5
				                       where sumber_ref = $1`, u.ref, u.keterangan, u.nominal, u.sumber, u.waktu)
			}
			return err
		},
		hapus: func(ctx context.Context, tx pgx.Tx, ref string) error {
			_, err := tx.Exec(ctx, `delete from pengeluaran where sumber_ref = $1`, ref)
			return err
		},
	}
}

func bacaUang(rows [][]string, tab string, isi func([]string, *uangSheet) error) ([]rekam, []Galat) {
	var out []rekam
	var galat []Galat
	seen := map[string]int{}
	body, nomor := data(rows)
	for i, r := range body {
		if kosong(r) {
			continue
		}
		t, err := waktu(sel(r, 0))
		if err != nil {
			galat = append(galat, Galat{nomor[i], err.Error()})
			continue
		}
		u := uangSheet{waktu: t}
		if err := isi(r, &u); err != nil {
			galat = append(galat, Galat{nomor[i], err.Error()})
			continue
		}
		u.ref = unik(seen, tab+":"+teksWaktu(t))
		out = append(out, rekam{u.ref, u.ringkas(), u.sidik(), u})
	}
	return out, galat
}

func kumpul(ctx context.Context, tx pgx.Tx, sql string, scan func(pgx.Rows) (rekam, error), args ...any) (map[string]rekam, error) {
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]rekam{}
	for rows.Next() {
		r, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out[r.kunci] = r
	}
	return out, rows.Err()
}

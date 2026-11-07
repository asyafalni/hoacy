package store

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/asyafalni/hoacy/backend/internal/auth"
)

// Every admin write goes through ubah: one transaction plus an audit row with
// the record before and after.
func (s *Store) ubah(ctx context.Context, tabel, kunci, oleh string, sebelumSQL string, fn func(pgx.Tx) error) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	snap := func() []byte {
		if sebelumSQL == "" {
			return nil
		}
		var b []byte
		if err := tx.QueryRow(ctx, sebelumSQL, kunci).Scan(&b); err != nil {
			return nil
		}
		return b
	}
	lama := snap()
	if err := fn(tx); err != nil {
		return err
	}
	baru := snap()
	if _, err := tx.Exec(ctx, `insert into audit (tabel, kunci, sebelum, sesudah, oleh) values ($1, $2, $3, $4, $5)`,
		tabel, kunci, lama, baru, oleh); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// snapshot queries: the row as JSON, without the PIN hash
const (
	snapRumah    = `select to_jsonb(r) - 'pin_hash' from rumah r where alamat = $1`
	snapRiwayat  = `select coalesce(jsonb_agg(to_jsonb(x) order by berlaku), '[]') from rumah_riwayat x where alamat = $1`
	snapTarif    = `select coalesce(jsonb_agg(to_jsonb(x) order by komponen, luas_min), '[]') from tarif_versi x where berlaku = $1::int`
	snapPengurus = `select to_jsonb(p) from pengurus p where email = $1`
)

// ── Rumah ──────────────────────────────────────────────────────────────────

type RumahAdmin struct {
	Alamat       string     `json:"alamat"`
	Cluster      string     `json:"cluster"`
	Blok         string     `json:"blok"`
	Nomor        string     `json:"rumah"`
	Nama         string     `json:"nama"`
	Telp         *string    `json:"telp"`
	PINDiganti   *time.Time `json:"pinDiganti"`
	NonaktifAt   *time.Time `json:"nonaktifAt"`
	NonaktifOleh *string    `json:"nonaktifOleh"`
	Riwayat      []struct {
		Berlaku int     `json:"berlaku"`
		Luas    float64 `json:"luas"`
		Tipe    string  `json:"tipe"`
	} `json:"riwayat"`
}

func (s *Store) DaftarRumahAdmin(ctx context.Context) ([]RumahAdmin, error) {
	rows, err := s.DB.Query(ctx, `
	  select r.alamat, r.cluster, r.blok, r.nomor, r.nama, r.telp, r.pin_diganti_at, r.nonaktif_at, r.nonaktif_oleh,
	         coalesce((select jsonb_agg(jsonb_build_object('berlaku', x.berlaku, 'luas', x.luas, 'tipe', x.tipe) order by x.berlaku)
	                     from rumah_riwayat x where x.alamat = r.alamat), '[]')
	    from rumah r order by r.blok, r.nomor`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (RumahAdmin, error) {
		var x RumahAdmin
		var riw []byte
		var nonaktif *time.Time
		err := r.Scan(&x.Alamat, &x.Cluster, &x.Blok, &x.Nomor, &x.Nama, &x.Telp, &x.PINDiganti, &nonaktif, &x.NonaktifOleh, &riw)
		x.NonaktifAt = nonaktif
		json.Unmarshal(riw, &x.Riwayat)
		return x, err
	})
}

type RumahBaru struct {
	Cluster, Blok, Nomor, Nama, Telp string
}

// TambahRumah adds a house with the default PIN (last 6 digits of its phone),
// or a random one returned so the admin can hand it over.
func (s *Store) TambahRumah(ctx context.Context, r RumahBaru, oleh string) (alamat, pinAcak string, err error) {
	if r.Cluster == "" {
		r.Cluster = "N"
	}
	if len(r.Nomor) == 1 {
		r.Nomor = "0" + r.Nomor
	}
	alamat = r.Cluster + r.Blok + "-" + r.Nomor
	pin, ok := auth.PINBawaan(r.Telp)
	if !ok {
		pin = PINAcak()
		pinAcak = pin
	}
	hash, err := auth.HashPIN(pin)
	if err != nil {
		return "", "", err
	}
	err = s.ubah(ctx, "rumah", alamat, oleh, snapRumah, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `insert into blok (kode) values ($1) on conflict do nothing`, r.Blok); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `insert into rumah (alamat, cluster, blok, nomor, nama, telp, pin_hash)
		                        values ($1, $2, $3, $4, $5, nullif($6, ''), $7)`,
			alamat, r.Cluster, r.Blok, r.Nomor, r.Nama, r.Telp, hash)
		return err
	})
	return alamat, pinAcak, err
}

func (s *Store) UbahRumah(ctx context.Context, alamat, nama, telp, oleh string) error {
	return s.ubah(ctx, "rumah", alamat, oleh, snapRumah, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `update rumah set nama = $2, telp = nullif($3, '') where alamat = $1`, alamat, nama, telp)
		if err == nil && tag.RowsAffected() == 0 {
			return ErrTidakAda
		}
		return err
	})
}

var ErrKonfirmasi = errors.New("ketik alamat rumah persis untuk konfirmasi")

// Nonaktifkan: a merger or an uncollectable house. Deliberately not one
// click — the admin must type the exact alamat. The reason is never stored.
func (s *Store) Nonaktifkan(ctx context.Context, alamat, konfirmasi string, oleh string) error {
	if konfirmasi != alamat {
		return ErrKonfirmasi
	}
	return s.ubah(ctx, "rumah", alamat, oleh, snapRumah, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `update rumah set nonaktif_at = (now() at time zone 'Asia/Jakarta')::date, nonaktif_oleh = $2
		                          where alamat = $1 and nonaktif_at is null`, alamat, oleh)
		if err == nil && tag.RowsAffected() == 0 {
			return ErrTidakAda
		}
		return err
	})
}

func (s *Store) Aktifkan(ctx context.Context, alamat, oleh string) error {
	return s.ubah(ctx, "rumah", alamat, oleh, snapRumah, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `update rumah set nonaktif_at = null, nonaktif_oleh = null where alamat = $1`, alamat)
		return err
	})
}

// ResetPIN puts a house back on its default PIN (or a random one, returned,
// when it has no phone). Every signed-in phone of that house is signed out.
func (s *Store) ResetPIN(ctx context.Context, alamat, oleh string) (pinAcak string, err error) {
	var telp *string
	if err := s.DB.QueryRow(ctx, `select telp from rumah where alamat = $1`, alamat).Scan(&telp); err != nil {
		return "", ErrTidakAda
	}
	pin, ok := "", false
	if telp != nil {
		pin, ok = auth.PINBawaan(*telp)
	}
	if !ok {
		pin = PINAcak()
		pinAcak = pin
	}
	hash, err := auth.HashPIN(pin)
	if err != nil {
		return "", err
	}
	err = s.ubah(ctx, "rumah.pin", alamat, oleh, "", func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `update rumah set pin_hash = $2, pin_diganti_at = null where alamat = $1`, alamat, hash)
		return err
	})
	return pinAcak, err
}

func PINAcak() string {
	var b [4]byte
	randRead(b[:])
	n := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 1_000_000
	return fmt.Sprintf("%06d", n)
}

// ── Riwayat & tarif ────────────────────────────────────────────────────────

func (s *Store) SimpanRiwayat(ctx context.Context, alamat string, berlaku int, luas float64, tipe, oleh string) error {
	return s.ubah(ctx, "rumah_riwayat", alamat, oleh, snapRiwayat, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `insert into rumah_riwayat values ($1, $2, $3, $4)
		                        on conflict (alamat, berlaku) do update set luas = excluded.luas, tipe = excluded.tipe`,
			alamat, berlaku, luas, tipe)
		return err
	})
}

func (s *Store) HapusRiwayat(ctx context.Context, alamat string, berlaku int, oleh string) error {
	return s.ubah(ctx, "rumah_riwayat", alamat, oleh, snapRiwayat, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `delete from rumah_riwayat where alamat = $1 and berlaku = $2`, alamat, berlaku)
		return err
	})
}

type TarifBaris struct {
	Komponen string  `json:"komponen"`
	LuasMin  float64 `json:"luasMin"`
	Nominal  int64   `json:"nominal"`
}

type RateCard struct {
	Berlaku int          `json:"berlaku"`
	Baris   []TarifBaris `json:"baris"`
}

func (s *Store) DaftarTarif(ctx context.Context) ([]RateCard, error) {
	rows, err := s.DB.Query(ctx, `select berlaku, komponen::text, luas_min::float8, nominal from tarif_versi
	                               order by berlaku desc, komponen, luas_min`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RateCard
	for rows.Next() {
		var b int
		var t TarifBaris
		if err := rows.Scan(&b, &t.Komponen, &t.LuasMin, &t.Nominal); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Berlaku != b {
			out = append(out, RateCard{Berlaku: b})
		}
		out[len(out)-1].Baris = append(out[len(out)-1].Baris, t)
	}
	return out, rows.Err()
}

// SimpanRateCard replaces the whole card for one `berlaku` — a price change is
// always a complete new set, including the unchanged rules.
func (s *Store) SimpanRateCard(ctx context.Context, c RateCard, oleh string) error {
	return s.ubah(ctx, "tarif_versi", fmt.Sprint(c.Berlaku), oleh, snapTarif, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `delete from tarif_versi where berlaku = $1`, c.Berlaku); err != nil {
			return err
		}
		for _, b := range c.Baris {
			if b.Komponen != "islk_rumah" {
				b.LuasMin = 0
			}
			if _, err := tx.Exec(ctx, `insert into tarif_versi values ($1, $2, $3, $4)`,
				c.Berlaku, b.Komponen, b.LuasMin, b.Nominal); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) HapusRateCard(ctx context.Context, berlaku int, oleh string) error {
	return s.ubah(ctx, "tarif_versi", fmt.Sprint(berlaku), oleh, snapTarif, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `delete from tarif_versi where berlaku = $1`, berlaku)
		return err
	})
}

// WarnaBlok is every block's stored color (blocks without one are absent).
func (s *Store) WarnaBlok(ctx context.Context) (map[string]string, error) {
	rows, err := s.DB.Query(ctx, `select kode, warna from blok where warna is not null`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, w string
		if err := rows.Scan(&k, &w); err != nil {
			return nil, err
		}
		out[k] = w
	}
	return out, rows.Err()
}

func (s *Store) SimpanWarnaBlok(ctx context.Context, kode, warna, oleh string) error {
	return s.ubah(ctx, "blok", kode, oleh, `select to_jsonb(b) from blok b where kode = $1`, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `insert into blok values ($1, nullif($2, '')) on conflict (kode) do update set warna = excluded.warna`, kode, warna)
		return err
	})
}

// ── Pengurus ───────────────────────────────────────────────────────────────

type PengurusRow struct {
	Email       string `json:"email"`
	Nama        string `json:"nama"`
	Peran       string `json:"peran"`
	Aktif       bool   `json:"aktif"`
	PernahMasuk bool   `json:"pernahMasuk"`
}

func (s *Store) DaftarPengurus(ctx context.Context) ([]PengurusRow, error) {
	rows, err := s.DB.Query(ctx, `select email, nama, peran::text, aktif, clerk_user_id is not null from pengurus order by aktif desc, nama`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (PengurusRow, error) {
		var p PengurusRow
		err := r.Scan(&p.Email, &p.Nama, &p.Peran, &p.Aktif, &p.PernahMasuk)
		return p, err
	})
}

func (s *Store) SimpanPengurus(ctx context.Context, p PengurusRow, oleh string) error {
	return s.ubah(ctx, "pengurus", p.Email, oleh, snapPengurus, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `insert into pengurus (email, nama, peran, aktif) values ($1, $2, $3, $4)
		                        on conflict (email) do update set nama = excluded.nama, peran = excluded.peran, aktif = excluded.aktif`,
			p.Email, p.Nama, p.Peran, p.Aktif)
		return err
	})
}

// JumlahAdmin is the number of active admins (the last one can't remove itself).
func (s *Store) JumlahAdmin(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRow(ctx, `select count(*) from pengurus where aktif and peran = 'admin'`).Scan(&n)
	return n, err
}

// ── Keamanan ───────────────────────────────────────────────────────────────

type Keamanan struct {
	Gagal24j int               `json:"gagal24j"`
	Gagal7h  int               `json:"gagal7h"`
	PerRumah []json.RawMessage `json:"perRumah"`
	PerIP    []json.RawMessage `json:"perIP"`
	Terbaru  []json.RawMessage `json:"terbaru"`
}

func (s *Store) Keamanan(ctx context.Context) (*Keamanan, error) {
	k := &Keamanan{}
	b := &pgx.Batch{}
	b.Queue(`select count(*) filter (where waktu > now() - interval '24 hours'), count(*)
	           from login_warga where not berhasil and waktu > now() - interval '7 days'`).
		QueryRow(func(r pgx.Row) error { return r.Scan(&k.Gagal24j, &k.Gagal7h) })
	jsonRows := func(out *[]json.RawMessage) func(pgx.Rows) error {
		return func(rows pgx.Rows) error {
			for rows.Next() {
				var b []byte
				if err := rows.Scan(&b); err != nil {
					return err
				}
				*out = append(*out, b)
			}
			return rows.Err()
		}
	}
	// houses with failures in the last 7 days, worst first; `ada` = a real house
	b.Queue(`select jsonb_build_object('alamat', l.alamat, 'gagal', count(*),
	                  'gagal24j', count(*) filter (where waktu > now() - interval '24 hours'),
	                  'terakhir', max(waktu), 'ip', count(distinct ip), 'ada', exists (select 1 from rumah r where r.alamat = l.alamat))
	           from login_warga l where not berhasil and waktu > now() - interval '7 days'
	          group by l.alamat order by count(*) desc limit 50`).Query(jsonRows(&k.PerRumah))
	// IPs trying many different houses — someone walking the address list
	b.Queue(`select jsonb_build_object('ip', host(ip), 'gagal', count(*), 'rumah', count(distinct alamat),
	                  'terakhir', max(waktu), 'mencurigakan', count(distinct alamat) >= 5)
	           from login_warga where not berhasil and waktu > now() - interval '7 days' and ip is not null
	          group by ip order by count(distinct alamat) desc, count(*) desc limit 50`).Query(jsonRows(&k.PerIP))
	b.Queue(`select jsonb_build_object('alamat', alamat, 'berhasil', berhasil, 'ip', host(ip), 'perangkat', user_agent, 'waktu', waktu)
	           from login_warga order by waktu desc limit 100`).Query(jsonRows(&k.Terbaru))
	if err := s.kirim(ctx, b); err != nil {
		return nil, err
	}
	return k, nil
}

// ── Logs ───────────────────────────────────────────────────────────────────

func (s *Store) Log(ctx context.Context) (sinkron, audit []json.RawMessage, err error) {
	b := &pgx.Batch{}
	jsonRows := func(out *[]json.RawMessage) func(pgx.Rows) error {
		return func(rows pgx.Rows) error {
			for rows.Next() {
				var x []byte
				if err := rows.Scan(&x); err != nil {
					return err
				}
				*out = append(*out, x)
			}
			return rows.Err()
		}
	}
	b.Queue(`select to_jsonb(x) from sinkron_log x order by waktu desc limit 50`).Query(jsonRows(&sinkron))
	b.Queue(`select to_jsonb(x) from audit x order by waktu desc limit 100`).Query(jsonRows(&audit))
	err = s.kirim(ctx, b)
	return
}

func randRead(b []byte) { crand.Read(b) }

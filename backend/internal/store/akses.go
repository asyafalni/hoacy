package store

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/asyafalni/hoacy/backend/internal/auth"
)

// ── Warga login ────────────────────────────────────────────────────────────

// PINRumah is an active house's PIN hash (ErrTidakAda for unknown or inactive).
func (s *Store) PINRumah(ctx context.Context, alamat string) (string, error) {
	var h string
	err := s.DB.QueryRow(ctx, `select pin_hash from rumah where alamat = $1 and nonaktif_at is null`, alamat).Scan(&h)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrTidakAda
	}
	return h, err
}

// Percobaan counts a house's failures since its last success, and the IP's
// failures across every house in the last hour.
func (s *Store) Percobaan(ctx context.Context, alamat string, ip *netip.Addr) (auth.Percobaan, error) {
	var p auth.Percobaan
	var terakhir, ipTerakhir *time.Time
	err := s.DB.QueryRow(ctx, `
	  with sejak as (select coalesce(max(waktu), '-infinity') as t from login_warga where alamat = $1 and berhasil),
	       gagal as (select waktu from login_warga, sejak
	                  where alamat = $1 and not berhasil and waktu > sejak.t and waktu > now() - interval '24 hours')
	  select (select count(*) from gagal where waktu > now() - interval '15 minutes'),
	         (select count(*) from gagal),
	         (select max(waktu) from gagal),
	         (select count(*) from login_warga where ip = $2 and not berhasil and waktu > now() - interval '1 hour'),
	         (select max(waktu) from login_warga where ip = $2 and not berhasil and waktu > now() - interval '1 hour')`,
		alamat, ip).Scan(&p.Gagal15m, &p.Gagal24j, &terakhir, &p.IPGagal1j, &ipTerakhir)
	if terakhir != nil {
		p.GagalTerakhir = *terakhir
	}
	if ipTerakhir != nil {
		p.IPTerakhir = *ipTerakhir
	}
	return p, err
}

func (s *Store) CatatLogin(ctx context.Context, alamat string, berhasil bool, ip *netip.Addr, ua string) error {
	if len(alamat) > 40 {
		alamat = alamat[:40]
	}
	if len(ua) > 300 {
		ua = ua[:300]
	}
	_, err := s.DB.Exec(ctx, `insert into login_warga (alamat, berhasil, ip, user_agent) values ($1, $2, $3, $4)`,
		alamat, berhasil, ip, ua)
	return err
}

// GantiPIN sets a new PIN hash; diganti=false marks it as the default again
// (an admin reset).
func (s *Store) GantiPIN(ctx context.Context, alamat, hash string, diganti bool) error {
	tag, err := s.DB.Exec(ctx, `update rumah set pin_hash = $2,
	                              pin_diganti_at = case when $3 then now() end where alamat = $1`, alamat, hash, diganti)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrTidakAda
	}
	return err
}

// ── Pengurus ───────────────────────────────────────────────────────────────

// CariPengurus is an active pengurus by email. On a first Clerk sign-in it
// records the Clerk user id; after that the id must match (an email moved to
// another Clerk account doesn't inherit the rights).
func (s *Store) CariPengurus(ctx context.Context, email, clerkID string) (*auth.Pengurus, error) {
	var p auth.Pengurus
	var id *string
	err := s.DB.QueryRow(ctx, `select email, nama, peran::text, clerk_user_id from pengurus where email = $1 and aktif`,
		email).Scan(&p.Email, &p.Nama, &p.Peran, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrBukanPengurus
	}
	if err != nil {
		return nil, err
	}
	if clerkID != "" {
		if id == nil {
			if _, err := s.DB.Exec(ctx, `update pengurus set clerk_user_id = $2 where email = $1`, email, clerkID); err != nil {
				return nil, err
			}
		} else if *id != clerkID {
			return nil, auth.ErrBukanPengurus
		}
	}
	return &p, nil
}

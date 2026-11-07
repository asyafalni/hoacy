package sinkron

import (
	"context"
	"fmt"

	"github.com/asyafalni/hoacy/backend/internal/tagihan"
)

// M-Opex and M-SaldoAwal stay edited in the Sheet. The server copies them at
// start-up and daily (cmd/server) and keeps the last good copy in Postgres.

// BacaOpex: A kategori · B ikon · C nominal · D diperbarui (yyyy-mm-dd).
func BacaOpex(ctx context.Context, s Sumber) ([]tagihan.Opex, error) {
	rows, err := s.Baca(ctx, "M-Opex")
	if err != nil {
		return nil, err
	}
	out := []tagihan.Opex{}
	body, nomor := data(rows)
	for i, r := range body {
		if sel(r, 0) == "" {
			continue
		}
		n, err := rupiah(sel(r, 2))
		if err != nil {
			return nil, fmt.Errorf("M-Opex baris %d: nominal tidak valid", nomor[i])
		}
		o := tagihan.Opex{Kategori: sel(r, 0), Ikon: sel(r, 1), Nominal: n}
		if d := sel(r, 3); d != "" {
			t, err := waktu(d)
			if err != nil {
				return nil, fmt.Errorf("M-Opex baris %d: %w", nomor[i], err)
			}
			o.Diperbarui = t.Format("2006-01-02")
		}
		out = append(out, o)
	}
	return out, nil
}

type SaldoAwal struct {
	Tanggal  string `json:"tanggal"`
	Kas      int64  `json:"kas"`
	Rekening int64  `json:"rekening"`
}

// BacaSaldoAwal: row 2 — A tanggal · B kas · C rekening.
func BacaSaldoAwal(ctx context.Context, s Sumber) (SaldoAwal, error) {
	rows, err := s.Baca(ctx, "M-SaldoAwal")
	if err != nil {
		return SaldoAwal{}, err
	}
	if len(rows) < 2 {
		return SaldoAwal{}, nil
	}
	r := rows[1]
	var sa SaldoAwal
	if t, err := waktu(sel(r, 0)); err == nil {
		sa.Tanggal = t.Format("2006-01-02")
	}
	if sa.Kas, err = rupiah(sel(r, 1)); err != nil && sel(r, 1) != "" {
		return SaldoAwal{}, fmt.Errorf("M-SaldoAwal: kas tidak valid")
	}
	if sa.Rekening, err = rupiah(sel(r, 2)); err != nil && sel(r, 2) != "" {
		return SaldoAwal{}, fmt.Errorf("M-SaldoAwal: rekening tidak valid")
	}
	return sa, nil
}

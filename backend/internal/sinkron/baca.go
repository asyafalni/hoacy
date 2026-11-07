// Package sinkron brings Sheet rows into Postgres on demand: an admin picks a
// tab, sees what would change (Pratinjau), and applies it (Terapkan). Always
// Sheet → database. Every source row has a natural key stored as sumber_ref
// (ledgers) or the table's own primary key (master tabs), so syncing again
// updates instead of duplicating. A row that disappeared from the Sheet is only
// listed — it is deleted only when the admin ticks it.
package sinkron

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Sumber is where tabs are read from: Google Sheets in production
// (google.Klien), CSV files in tests and local development.
type Sumber interface {
	Baca(ctx context.Context, tab string) ([][]string, error)
	DaftarTab(ctx context.Context) ([]string, error)
}

var wib = func() *time.Location {
	l, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return l
}()

func sel(row []string, i int) string {
	if i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}

var bukanAngka = regexp.MustCompile(`[^0-9-]`)

// rupiah reads a whole-rupiah cell ("300000"; also "300,000" or "Rp300.000").
func rupiah(s string) (int64, error) {
	s = bukanAngka.ReplaceAllString(s, "")
	if s == "" {
		return 0, fmt.Errorf("nominal kosong")
	}
	return strconv.ParseInt(s, 10, 64)
}

// angka reads a decimal (luas). The spreadsheet locale is United States, so
// "." is the decimal point and "," a thousands separator.
func angka(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", ""), 64)
}

func bulat(s string) (int, error) {
	n, err := rupiah(s)
	return int(n), err
}

// waktu reads a timestamp or date as the Sheet shows it (US locale Form
// timestamps like 9/16/2026 7:55:00, or ISO dates the komite typed), in WIB.
func waktu(s string) (time.Time, error) {
	for _, f := range []string{"1/2/2006 15:04:05", "1/2/2006 15:04", "1/2/2006",
		"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(f, strings.TrimSpace(s), wib); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("tanggal tidak terbaca: %q", s)
}

// teksWaktu is the canonical text of a timestamp — the id a Keputusan points
// at (yyyy-mm-dd hh:mm:ss, as 'D-Pembayaran'!A showed it).
func teksWaktu(t time.Time) string { return t.In(wib).Format("2006-01-02 15:04:05") }

// rincian splits "202607=360000,202609=360000". A malformed part becomes
// periode 0, which the pembayaran view marks `cek` (as D-Pembayaran did).
func rincian(s string) [][2]int64 {
	var out [][2]int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		p, n, ok := strings.Cut(part, "=")
		pp, err1 := strconv.Atoi(strings.TrimSpace(p))
		nn, err2 := strconv.ParseInt(strings.TrimSpace(n), 10, 64)
		if !ok || err1 != nil || err2 != nil {
			pp, nn = 0, 0
		}
		merged := false
		for i := range out { // the same month twice in one rincian: one row, summed
			if out[i][0] == int64(pp) {
				out[i][1] += nn
				merged = true
			}
		}
		if !merged {
			out = append(out, [2]int64{int64(pp), nn})
		}
	}
	return out
}

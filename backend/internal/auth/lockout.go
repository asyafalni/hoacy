package auth

import "time"

// Lock-out is computed from the login_warga log, never kept in memory: it
// survives restarts and the admin "Keamanan" page reads the same rows.
//
// Per house, counting only failures since its last success:
//   - 5 or more in the last 15 minutes  → locked 15 minutes
//   - 10 or more in the last 24 hours   → locked 1 hour (from the latest failure)
//   - 20 or more in the last 24 hours   → locked 24 hours
//
// Per IP, across all houses (someone walking the address list):
//   - 30 or more failures in the last hour → that IP is locked 1 hour
type Percobaan struct {
	Gagal15m      int
	Gagal24j      int
	GagalTerakhir time.Time
	IPGagal1j     int
	IPTerakhir    time.Time
}

// Terkunci reports whether a login must be refused now, and until when.
func (p Percobaan) Terkunci(now time.Time) (bool, time.Time) {
	var sampai time.Time
	pakai := func(dari time.Time, d time.Duration) {
		if t := dari.Add(d); t.After(sampai) {
			sampai = t
		}
	}
	if p.Gagal24j >= 20 {
		pakai(p.GagalTerakhir, 24*time.Hour)
	} else if p.Gagal24j >= 10 {
		pakai(p.GagalTerakhir, time.Hour)
	} else if p.Gagal15m >= 5 {
		pakai(p.GagalTerakhir, 15*time.Minute)
	}
	if p.IPGagal1j >= 30 {
		pakai(p.IPTerakhir, time.Hour)
	}
	return sampai.After(now), sampai
}

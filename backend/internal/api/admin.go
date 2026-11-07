package api

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	fibhttp "github.com/lesismal/fib/http"

	"github.com/asyafalni/hoacy/backend/internal/auth"
	"github.com/asyafalni/hoacy/backend/internal/sinkron"
	"github.com/asyafalni/hoacy/backend/internal/store"
)

var (
	polaBlok  = regexp.MustCompile(`^(Blvd|[0-9]{1,2})$`)
	polaNomor = regexp.MustCompile(`^[0-9]{1,3}$`)
	polaTelp  = regexp.MustCompile(`^62[0-9]{8,13}$`)
	polaWarna = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	polaEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// dbGagal turns a constraint violation into a 400 the admin can act on.
func (a *App) dbGagal(c *fibhttp.Context, err error) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			a.gagal(c, 409, "sudah-ada", "Data itu sudah ada.")
			return
		case "23503":
			a.gagal(c, 409, "dipakai", "Data ini masih dipakai catatan lain, jadi tidak bisa dihapus.")
			return
		case "23514", "22P02":
			a.gagal(c, 400, "tidak-valid", "Isian tidak valid.")
			return
		}
	}
	if errors.Is(err, store.ErrTidakAda) {
		a.gagal(c, 404, "tidak-ada", "Tidak ditemukan.")
		return
	}
	a.galat(c, err)
}

func (a *App) adminRumah(c *fibhttp.Context, _ *auth.Pengurus) {
	list, err := a.Store.DaftarRumahAdmin(c.Context())
	if err != nil {
		a.galat(c, err)
		return
	}
	warna, err := a.Store.WarnaBlok(c.Context())
	if err != nil {
		a.galat(c, err)
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "rumah": list, "warna": warna})
}

func (a *App) adminTambahRumah(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct{ Cluster, Blok, Rumah, Nama, Telp string }
	if !a.baca(c, &req) {
		return
	}
	req.Nama, req.Telp = strings.TrimSpace(req.Nama), strings.TrimSpace(req.Telp)
	switch {
	case !polaBlok.MatchString(req.Blok) || !polaNomor.MatchString(req.Rumah):
		a.gagal(c, 400, "alamat", "Blok atau nomor rumah tidak valid.")
		return
	case req.Nama == "":
		a.gagal(c, 400, "nama", "Isi nama kepala keluarga.")
		return
	case req.Telp != "" && !polaTelp.MatchString(req.Telp):
		a.gagal(c, 400, "telp", "No. HP harus berawalan 62, tanpa + atau spasi.")
		return
	}
	alamat, pin, err := a.Store.TambahRumah(c.Context(), store.RumahBaru{Cluster: req.Cluster, Blok: req.Blok,
		Nomor: req.Rumah, Nama: req.Nama, Telp: req.Telp}, p.Nama)
	if err != nil {
		a.dbGagal(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]any{"ok": true, "alamat": alamat, "pinAcak": pin})
}

func (a *App) adminUbahRumah(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct{ Nama, Telp string }
	if !a.baca(c, &req) {
		return
	}
	req.Nama, req.Telp = strings.TrimSpace(req.Nama), strings.TrimSpace(req.Telp)
	if req.Nama == "" || (req.Telp != "" && !polaTelp.MatchString(req.Telp)) {
		a.gagal(c, 400, "tidak-valid", "Nama wajib; no. HP berawalan 62.")
		return
	}
	if err := a.Store.UbahRumah(c.Context(), c.Param("alamat"), req.Nama, req.Telp, p.Nama); err != nil {
		a.dbGagal(c, err)
		return
	}
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) adminNonaktif(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct{ Konfirmasi string }
	if !a.baca(c, &req) {
		return
	}
	err := a.Store.Nonaktifkan(c.Context(), c.Param("alamat"), strings.TrimSpace(req.Konfirmasi), p.Nama)
	if errors.Is(err, store.ErrKonfirmasi) {
		a.gagal(c, 400, "konfirmasi", err.Error())
		return
	} else if err != nil {
		a.dbGagal(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) adminAktif(c *fibhttp.Context, p *auth.Pengurus) {
	if err := a.Store.Aktifkan(c.Context(), c.Param("alamat"), p.Nama); err != nil {
		a.dbGagal(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) adminResetPIN(c *fibhttp.Context, p *auth.Pengurus) {
	pin, err := a.Store.ResetPIN(c.Context(), c.Param("alamat"), p.Nama)
	if err != nil {
		a.dbGagal(c, err)
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "pinAcak": pin})
}

func (a *App) adminSimpanRiwayat(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct {
		Berlaku int     `json:"berlaku"`
		Luas    float64 `json:"luas"`
		Tipe    string  `json:"tipe"`
	}
	if !a.baca(c, &req) {
		return
	}
	if req.Berlaku%100 < 1 || req.Berlaku%100 > 12 || req.Luas <= 0 || (req.Tipe != "rumah" && req.Tipe != "kavling") {
		a.gagal(c, 400, "tidak-valid", "Bulan berlaku, luas (> 0) dan tipe (rumah/kavling) wajib benar.")
		return
	}
	if err := a.Store.SimpanRiwayat(c.Context(), c.Param("alamat"), req.Berlaku, req.Luas, req.Tipe, p.Nama); err != nil {
		a.dbGagal(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) adminHapusRiwayat(c *fibhttp.Context, p *auth.Pengurus) {
	berlaku, _ := strconv.Atoi(c.Param("berlaku"))
	if err := a.Store.HapusRiwayat(c.Context(), c.Param("alamat"), berlaku, p.Nama); err != nil {
		a.dbGagal(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) adminTarif(c *fibhttp.Context, _ *auth.Pengurus) {
	list, err := a.Store.DaftarTarif(c.Context())
	if err != nil {
		a.galat(c, err)
		return
	}
	if list == nil {
		list = []store.RateCard{}
	}
	a.json(c, 200, map[string]any{"ok": true, "tarif": list})
}

// PUT /api/admin/tarif/{berlaku} {baris: [...]} — the complete card.
func (a *App) adminSimpanTarif(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct {
		Baris []store.TarifBaris `json:"baris"`
	}
	if !a.baca(c, &req) {
		return
	}
	berlaku, _ := strconv.Atoi(c.Param("berlaku"))
	if berlaku%100 < 1 || berlaku%100 > 12 {
		a.gagal(c, 400, "berlaku", "Bulan berlaku tidak valid.")
		return
	}
	adaRT, adaRumah := false, false
	for _, b := range req.Baris {
		switch b.Komponen {
		case "iuran_rt":
			adaRT = true
		case "islk_rumah":
			adaRumah = true
		case "islk_kavling":
		default:
			a.gagal(c, 400, "komponen", "Komponen tidak dikenal: "+b.Komponen)
			return
		}
		if b.Nominal < 0 {
			a.gagal(c, 400, "nominal", "Nominal tidak boleh negatif.")
			return
		}
	}
	if !adaRT || !adaRumah {
		a.gagal(c, 400, "lengkap", "Rate card harus lengkap: minimal satu ISLK rumah dan Iuran RT.")
		return
	}
	if err := a.Store.SimpanRateCard(c.Context(), store.RateCard{Berlaku: berlaku, Baris: req.Baris}, p.Nama); err != nil {
		a.dbGagal(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) adminHapusTarif(c *fibhttp.Context, p *auth.Pengurus) {
	berlaku, _ := strconv.Atoi(c.Param("berlaku"))
	if err := a.Store.HapusRateCard(c.Context(), berlaku, p.Nama); err != nil {
		a.dbGagal(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) adminWarnaBlok(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct{ Warna string }
	if !a.baca(c, &req) {
		return
	}
	if req.Warna != "" && !polaWarna.MatchString(req.Warna) {
		a.gagal(c, 400, "warna", "Warna harus #rrggbb.")
		return
	}
	if err := a.Store.SimpanWarnaBlok(c.Context(), c.Param("kode"), req.Warna, p.Nama); err != nil {
		a.dbGagal(c, err)
		return
	}
	a.json(c, 200, map[string]bool{"ok": true})
}

func (a *App) adminPengurus(c *fibhttp.Context, _ *auth.Pengurus) {
	list, err := a.Store.DaftarPengurus(c.Context())
	if err != nil {
		a.galat(c, err)
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "pengurus": list})
}

// PUT /api/admin/pengurus/{email} {nama, peran, aktif, undang?} — add or
// change a pengurus; `undang` also sends the Clerk invitation email.
func (a *App) adminSimpanPengurus(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct {
		Nama   string `json:"nama"`
		Peran  string `json:"peran"`
		Aktif  bool   `json:"aktif"`
		Undang bool   `json:"undang"`
	}
	if !a.baca(c, &req) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(c.Param("email")))
	switch {
	case !polaEmail.MatchString(email):
		a.gagal(c, 400, "email", "Email tidak valid.")
		return
	case strings.TrimSpace(req.Nama) == "":
		a.gagal(c, 400, "nama", "Isi nama — tercatat di setiap verifikasi.")
		return
	case req.Peran != "admin" && req.Peran != "bendahara" && req.Peran != "komite":
		a.gagal(c, 400, "peran", "Peran harus admin, bendahara atau komite.")
		return
	}
	if email == p.Email && (req.Peran != "admin" || !req.Aktif) {
		if n, _ := a.Store.JumlahAdmin(c.Context()); n <= 1 {
			a.gagal(c, 400, "admin-terakhir", "Anda admin terakhir — tambahkan admin lain dulu.")
			return
		}
	}
	if err := a.Store.SimpanPengurus(c.Context(), store.PengurusRow{Email: email, Nama: strings.TrimSpace(req.Nama),
		Peran: req.Peran, Aktif: req.Aktif}, p.Nama); err != nil {
		a.dbGagal(c, err)
		return
	}
	terkirim := false
	if req.Undang && req.Aktif && a.Undang != nil {
		if err := a.Undang(c.Request, email); err != nil {
			a.Log.Warn("undangan clerk", "email", email, "err", err)
			a.gagal(c, 502, "undangan", "Pengurus tersimpan, tapi undangan Clerk gagal dikirim: "+err.Error())
			return
		}
		terkirim = true
	}
	a.json(c, 200, map[string]any{"ok": true, "undanganTerkirim": terkirim})
}

func (a *App) adminKeamanan(c *fibhttp.Context, _ *auth.Pengurus) {
	k, err := a.Store.Keamanan(c.Context())
	if err != nil {
		a.galat(c, err)
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "keamanan": k})
}

func (a *App) adminLog(c *fibhttp.Context, _ *auth.Pengurus) {
	s, au, err := a.Store.Log(c.Context())
	if err != nil {
		a.galat(c, err)
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "sinkron": s, "audit": au})
}

func (a *App) adaSinkron(c *fibhttp.Context) bool {
	if a.Sinkron == nil {
		a.gagal(c, 503, "sheet", "Sumber Sheet belum diatur di server (GOOGLE_* / SHEET_ID).")
		return false
	}
	return true
}

func (a *App) adminSinkronTab(c *fibhttp.Context, _ *auth.Pengurus) {
	if !a.adaSinkron(c) {
		return
	}
	tabs, err := a.Sinkron.Sumber.DaftarTab(c.Context())
	if err != nil {
		a.gagal(c, 502, "sheet", "Sheet tidak bisa dibaca: "+err.Error())
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "tab": sinkron.BisaDisinkron(tabs), "urutan": sinkron.TabTetap})
}

func (a *App) adminPratinjau(c *fibhttp.Context, _ *auth.Pengurus) {
	var req struct{ Tab string }
	if !a.baca(c, &req) || !a.adaSinkron(c) {
		return
	}
	p, err := a.Sinkron.Pratinjau(c.Context(), req.Tab)
	if err != nil {
		a.gagal(c, 400, "sinkron", err.Error())
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "pratinjau": p})
}

func (a *App) adminTerapkan(c *fibhttp.Context, p *auth.Pengurus) {
	var req struct {
		Tab   string   `json:"tab"`
		Hapus []string `json:"hapus"`
	}
	if !a.baca(c, &req) || !a.adaSinkron(c) {
		return
	}
	hasil, err := a.Sinkron.Terapkan(c.Context(), req.Tab, req.Hapus, p.Nama)
	if err != nil {
		a.gagal(c, 400, "sinkron", err.Error())
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]any{"ok": true, "hasil": hasil})
}

func (a *App) adminMuatOpex(c *fibhttp.Context, _ *auth.Pengurus) {
	if a.MuatSnapshot == nil {
		a.gagal(c, 503, "sheet", "Sumber Sheet belum diatur di server.")
		return
	}
	if err := a.MuatSnapshot(); err != nil {
		a.gagal(c, 502, "sheet", "Gagal memuat dari Sheet: "+err.Error())
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]bool{"ok": true})
}

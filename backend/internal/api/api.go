// Package api is the HTTP surface: JSON endpoints under /api and the built
// frontend for everything else. Routing and request plumbing are fib's; the
// decisions (who may see what, what a payment must satisfy) live in store,
// auth and tagihan.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	fibhttp "github.com/lesismal/fib/http"

	"github.com/asyafalni/hoacy/backend/internal/auth"
	"github.com/asyafalni/hoacy/backend/internal/berkas"
	"github.com/asyafalni/hoacy/backend/internal/sinkron"
	"github.com/asyafalni/hoacy/backend/internal/store"
)

type App struct {
	Store   *store.Store
	Sesi    *auth.Sesi
	Penjaga auth.Penjaga
	Berkas  berkas.Berkas
	Sinkron *sinkron.Sinkron // nil when no Sheet source is configured
	// MuatSnapshot reloads M-Opex/M-SaldoAwal now (admin button); nil without a Sheet.
	MuatSnapshot func() error
	// Undang sends a Clerk invitation (nil in development).
	Undang func(r *http.Request, email string) error
	// IPHeader is the proxy header carrying the client IP (DigitalOcean:
	// "do-connecting-ip"); empty = the TCP peer.
	IPHeader string
	Now      func() time.Time
	Log      *slog.Logger

	publikMu     sync.Mutex
	publik       []byte
	publikSampai time.Time
}

// Router builds every route. web serves the built frontend (nil in tests).
func (a *App) Router(web fibhttp.HandlerFunc) *fibhttp.Router {
	r := fibhttp.NewRouter()
	r.Get("/api/sehat", func(c *fibhttp.Context) { a.json(c, 200, map[string]bool{"ok": true}) })
	r.Get("/api/publik", a.getPublik)

	r.Post("/api/warga/masuk", a.wargaMasuk)
	r.Get("/api/warga", a.wargaData)
	r.Post("/api/warga/pin", a.wargaGantiPIN)
	r.Post("/api/warga/transfer", a.wargaTransfer)

	r.Get("/api/saya", a.pengurus(a.getSaya))
	r.Get("/api/kas", a.pengurus(a.getKas))
	r.Get("/api/kas/rumah/{alamat}", a.pengurus(a.getKasRumah))
	r.Get("/api/kas/tahun/{tahun:[0-9]+}", a.pengurus(a.getKasTahun))
	r.Get("/api/kas/bukti/{id:[0-9]+}", a.pengurus(a.getBukti))
	r.Get("/api/kas/nota/{id:[0-9]+}", a.pengurus(a.getNota))
	r.Get("/api/kas/pengeluaran", a.pengurus(a.getPengeluaran))
	r.Post("/api/kas/keputusan", a.pengurus(a.postKeputusan, "bendahara"))
	r.Post("/api/kas/tunai", a.pengurus(a.postTunai, "bendahara"))
	r.Post("/api/kas/setoran", a.pengurus(a.postSetoran, "bendahara"))
	r.Post("/api/kas/pengeluaran", a.pengurus(a.postPengeluaran, "bendahara"))

	r.Get("/api/admin/rumah", a.pengurus(a.adminRumah, "admin"))
	r.Post("/api/admin/rumah", a.pengurus(a.adminTambahRumah, "admin"))
	r.Put("/api/admin/rumah/{alamat}", a.pengurus(a.adminUbahRumah, "admin"))
	r.Post("/api/admin/rumah/{alamat}/nonaktif", a.pengurus(a.adminNonaktif, "admin"))
	r.Post("/api/admin/rumah/{alamat}/aktif", a.pengurus(a.adminAktif, "admin"))
	r.Post("/api/admin/rumah/{alamat}/reset-pin", a.pengurus(a.adminResetPIN, "admin"))
	r.Post("/api/admin/rumah/{alamat}/riwayat", a.pengurus(a.adminSimpanRiwayat, "admin"))
	r.Delete("/api/admin/rumah/{alamat}/riwayat/{berlaku:[0-9]+}", a.pengurus(a.adminHapusRiwayat, "admin"))
	r.Get("/api/admin/tarif", a.pengurus(a.adminTarif, "admin"))
	r.Put("/api/admin/tarif/{berlaku:[0-9]+}", a.pengurus(a.adminSimpanTarif, "admin"))
	r.Delete("/api/admin/tarif/{berlaku:[0-9]+}", a.pengurus(a.adminHapusTarif, "admin"))
	r.Put("/api/admin/blok/{kode}", a.pengurus(a.adminWarnaBlok, "admin"))
	r.Get("/api/admin/pengurus", a.pengurus(a.adminPengurus, "admin"))
	r.Put("/api/admin/pengurus/{email}", a.pengurus(a.adminSimpanPengurus, "admin"))
	r.Get("/api/admin/keamanan", a.pengurus(a.adminKeamanan, "admin"))
	r.Get("/api/admin/log", a.pengurus(a.adminLog, "admin"))
	r.Get("/api/admin/sinkron", a.pengurus(a.adminSinkronTab, "admin"))
	r.Post("/api/admin/sinkron/pratinjau", a.pengurus(a.adminPratinjau, "admin"))
	r.Post("/api/admin/sinkron/terapkan", a.pengurus(a.adminTerapkan, "admin"))
	r.Post("/api/admin/opex/muat-ulang", a.pengurus(a.adminMuatOpex, "admin"))

	r.NotFound(func(c *fibhttp.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || web == nil {
			a.gagal(c, 404, "tidak-ada", "Tidak ditemukan")
			return
		}
		web(c)
	})
	return r
}

// ── Plumbing ───────────────────────────────────────────────────────────────

func (a *App) json(c *fibhttp.Context, status int, v any) {
	c.Header().Set("Cache-Control", "no-store")
	_ = c.JSON(status, v)
}

// gagal answers {ok:false, kode, pesan}; pesan is shown to the user as is.
func (a *App) gagal(c *fibhttp.Context, status int, kode, pesan string) {
	a.json(c, status, map[string]any{"ok": false, "kode": kode, "pesan": pesan})
}

// galat logs an unexpected error and answers a generic 500.
func (a *App) galat(c *fibhttp.Context, err error) {
	a.Log.Error("api", "path", c.Request.URL.Path, "err", err)
	a.gagal(c, 500, "server", "Server bermasalah — coba lagi sebentar lagi.")
}

func (a *App) baca(c *fibhttp.Context, v any) bool {
	body := c.Body()
	if body == nil {
		b, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			a.gagal(c, 400, "request", "Permintaan tidak terbaca")
			return false
		}
		body = b
	}
	if err := json.Unmarshal(body, v); err != nil {
		a.gagal(c, 400, "request", "Permintaan tidak terbaca")
		return false
	}
	return true
}

func (a *App) ip(c *fibhttp.Context) *netip.Addr {
	s := ""
	if a.IPHeader != "" {
		s = strings.TrimSpace(strings.Split(c.Request.Header.Get(a.IPHeader), ",")[0])
	}
	if s == "" {
		s, _, _ = net.SplitHostPort(c.Request.RemoteAddr)
	}
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return nil
	}
	return &ip
}

func bearer(c *fibhttp.Context) string {
	t, _ := strings.CutPrefix(c.Request.Header.Get("Authorization"), "Bearer ")
	return strings.TrimSpace(t)
}

// pengurus wraps a handler that needs a signed-in pengurus, optionally with
// one of the given roles (admin always passes).
func (a *App) pengurus(h func(*fibhttp.Context, *auth.Pengurus), peran ...string) fibhttp.HandlerFunc {
	return func(c *fibhttp.Context) {
		p, err := a.Penjaga.Pengurus(c.Context(), c.Request)
		switch {
		case errors.Is(err, auth.ErrBelumMasuk):
			a.gagal(c, 401, "masuk", "Silakan masuk dulu.")
			return
		case errors.Is(err, auth.ErrBukanPengurus):
			a.gagal(c, 403, "bukan-pengurus", "Akun ini tidak terdaftar sebagai pengurus. Hubungi admin.")
			return
		case err != nil:
			a.galat(c, err)
			return
		}
		if len(peran) > 0 && !p.Boleh(peran...) {
			a.gagal(c, 403, "peran", "Peran Anda ("+p.Peran+") tidak boleh melakukan ini.")
			return
		}
		h(c, p)
	}
}

func (a *App) sekarang() int { return store.Sekarang(a.Now()) }

func (a *App) hari() map[string]int {
	p := a.sekarang()
	return map[string]int{"tahun": p / 100, "bulan": p % 100}
}

// ── Public ─────────────────────────────────────────────────────────────────

// getPublik: aggregates only, cached a minute — this page is open to anyone.
func (a *App) getPublik(c *fibhttp.Context) {
	a.publikMu.Lock()
	defer a.publikMu.Unlock()
	if a.publik == nil || a.Now().After(a.publikSampai) {
		p, err := a.Store.Publik(c.Context(), a.sekarang())
		if err != nil {
			a.galat(c, err)
			return
		}
		b, _ := json.Marshal(map[string]any{"ok": true, "hari": a.hari(), "data": p})
		a.publik, a.publikSampai = b, a.Now().Add(time.Minute)
	}
	c.Header().Set("Cache-Control", "public, max-age=60")
	_ = c.Respond(200, "application/json", a.publik)
}

func (a *App) lupakanPublik() {
	a.publikMu.Lock()
	a.publik = nil
	a.publikMu.Unlock()
}

// kirimBerkas serves a stored proof; a pengurus' browser shows it inline.
func (a *App) kirimBerkas(c *fibhttp.Context, ref string) {
	data, mime, err := a.Berkas.Buka(c.Context(), ref)
	if err != nil {
		a.Log.Warn("berkas", "ref", ref, "err", err)
		a.gagal(c, 404, "berkas", "Berkas tidak bisa dibuka.")
		return
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	c.Header().Set("Cache-Control", "private, max-age=3600")
	c.Header().Set("Content-Disposition", "inline")
	_ = c.RespondCopy(http.StatusOK, mime, data)
}

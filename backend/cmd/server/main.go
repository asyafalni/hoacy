// server is the whole app in one process: the JSON API and the built Vue
// frontend, on fib. Configuration is environment variables (backend/.env
// locally, encrypted app env vars on DigitalOcean) — see backend/.env.example.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	fib "github.com/lesismal/fib"
	fibhttp "github.com/lesismal/fib/http"
	"github.com/lesismal/fib/middleware"
	"github.com/lesismal/fib/middleware/recover"

	"github.com/clerk/clerk-sdk-go/v2/invitation"

	"github.com/asyafalni/hoacy/backend/internal/api"
	"github.com/asyafalni/hoacy/backend/internal/auth"
	"github.com/asyafalni/hoacy/backend/internal/berkas"
	"github.com/asyafalni/hoacy/backend/internal/config"
	"github.com/asyafalni/hoacy/backend/internal/db"
	"github.com/asyafalni/hoacy/backend/internal/google"
	"github.com/asyafalni/hoacy/backend/internal/sinkron"
	"github.com/asyafalni/hoacy/backend/internal/store"
)

func main() {
	config.LoadDotEnv(".env")
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := jalan(log); err != nil {
		log.Error("server berhenti", "err", err)
		os.Exit(1)
	}
}

func env(k, bawaan string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return bawaan
}

func jalan(log *slog.Logger) error {
	produksi := env("APP_ENV", "production") != "development"
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── settings that must be right before anything starts ──
	secret := os.Getenv("SESSION_SECRET")
	if len(secret) < 32 {
		if produksi {
			return errors.New("SESSION_SECRET wajib, minimal 32 karakter acak")
		}
		secret = "dev-only-session-secret-not-for-production!"
	}
	clerkKey := os.Getenv("CLERK_SECRET_KEY")
	if produksi && clerkKey == "" {
		return errors.New("CLERK_SECRET_KEY wajib di produksi")
	}

	// ── database ──
	dbCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	pool, err := db.Open(dbCtx, os.Getenv("DATABASE_URL"))
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	migCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	res, err := db.Migrate(migCtx, pool)
	cancel()
	if err != nil {
		return fmt.Errorf("migrasi: %w", err)
	}
	for _, r := range res {
		log.Info("migrasi", "file", r.Source.Path, "durasi", r.Duration.String())
	}
	st := &store.Store{DB: pool}
	if err := bootstrapAdmin(ctx, st, log); err != nil {
		return err
	}

	// ── Google (Drive + Sheets) or local stand-ins ──
	var simpanan berkas.Berkas
	var sumber sinkron.Sumber
	gcfg := google.Config{ClientID: os.Getenv("GOOGLE_CLIENT_ID"), ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RefreshToken: os.Getenv("GOOGLE_REFRESH_TOKEN"), FolderID: os.Getenv("DRIVE_FOLDER_ID"), SheetID: os.Getenv("SHEET_ID")}
	if gcfg.Lengkap() {
		k, err := google.New(ctx, gcfg)
		if err != nil {
			return err
		}
		simpanan = k
		if gcfg.SheetID != "" {
			sumber = k
		}
	} else if produksi {
		return errors.New("GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET dan GOOGLE_REFRESH_TOKEN wajib di produksi (bukti disimpan di Drive)")
	} else {
		simpanan = berkas.Disk{Dir: env("BERKAS_DIR", "berkas-lokal")}
		if dir := os.Getenv("SHEET_CSV_DIR"); dir != "" {
			sumber = sinkron.CSV{Dir: dir}
		}
		log.Warn("mode pengembangan: bukti disimpan di folder lokal", "dir", env("BERKAS_DIR", "berkas-lokal"))
	}

	// ── who is a pengurus ──
	var penjaga auth.Penjaga
	var undang func(*http.Request, string) error
	if clerkKey != "" {
		penjaga = auth.NewClerk(clerkKey, st.CariPengurus)
		undang = func(r *http.Request, email string) error {
			redirect := asal(r) + "/#/kas"
			notify := true
			_, err := invitation.Create(r.Context(), &invitation.CreateParams{EmailAddress: email,
				RedirectURL: &redirect, Notify: &notify})
			return err
		}
	} else {
		penjaga = auth.Dev{Cari: st.CariPengurus}
		log.Warn("mode pengembangan: pengurus dikenali dari header X-Dev-Email (tanpa Clerk)")
	}

	now := time.Now
	if h := os.Getenv("HARI_TETAP"); h != "" && !produksi {
		// development only: pin "today" (the mock data is built around 2026-09-22)
		t, err := time.ParseInLocation("2006-01-02", h, store.WIB)
		if err != nil {
			return fmt.Errorf("HARI_TETAP: %w", err)
		}
		mulai := time.Now()
		now = func() time.Time { return t.Add(12*time.Hour + time.Since(mulai)) }
		log.Warn("mode pengembangan: hari ini dipatok", "hari", h)
	}
	app := &api.App{Store: st, Sesi: auth.NewSesi(secret), Penjaga: penjaga, Berkas: simpanan, Undang: undang,
		IPHeader: os.Getenv("IP_HEADER"), Now: now, Log: log}
	if sumber != nil {
		app.Sinkron = &sinkron.Sinkron{DB: pool, Sumber: sumber}
		app.MuatSnapshot = func() error {
			c, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			return muatSnapshot(c, st, sumber)
		}
		go snapshotHarian(ctx, app.MuatSnapshot, log)
	}

	web, err := frontend(env("WEB_DIR", "web"))
	if err != nil {
		return err
	}
	handler := middleware.Chain(app.Router(web), recover.New())
	addr := ":" + env("PORT", "8080")
	engine, err := fib.New(fib.DefaultConfig(), fib.Binding{Addr: addr, Handler: fibhttp.NewHandler(handler)})
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		engine.Shutdown(c)
		engine.Close()
	}()
	log.Info("siap", "addr", addr, "produksi", produksi)
	return engine.Run()
}

// asal is the public origin of a request (behind DigitalOcean's proxy).
func asal(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "https"
	}
	return proto + "://" + r.Host
}

// bootstrapAdmin: the very first admin comes from BOOTSTRAP_ADMIN_EMAIL; after
// that, admins add everyone else in the app.
func bootstrapAdmin(ctx context.Context, st *store.Store, log *slog.Logger) error {
	email := strings.ToLower(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))
	if email == "" {
		return nil
	}
	if n, err := st.JumlahAdmin(ctx); err != nil || n > 0 {
		return err
	}
	nama := env("BOOTSTRAP_ADMIN_NAMA", "Admin")
	log.Info("admin pertama dibuat", "email", email)
	return st.SimpanPengurus(ctx, store.PengurusRow{Email: email, Nama: nama, Peran: "admin", Aktif: true}, "bootstrap")
}

// ── M-Opex / M-SaldoAwal: loaded at start-up and once a day ────────────────

func snapshotHarian(ctx context.Context, muat func() error, log *slog.Logger) {
	for {
		if err := muat(); err != nil {
			log.Warn("snapshot sheet gagal — memakai salinan terakhir", "err", err)
		} else {
			log.Info("snapshot sheet dimuat")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
		}
	}
}

func muatSnapshot(ctx context.Context, st *store.Store, s sinkron.Sumber) error {
	opex, err := sinkron.BacaOpex(ctx, s)
	if err != nil {
		return err
	}
	saldo, err := sinkron.BacaSaldoAwal(ctx, s)
	if err != nil {
		return err
	}
	if err := st.SimpanSnapshot(ctx, "M-Opex", opex); err != nil {
		return err
	}
	return st.SimpanSnapshot(ctx, "M-SaldoAwal", saldo)
}

// ── Frontend ───────────────────────────────────────────────────────────────

// frontend serves the built app (npm run build → dist, copied to WEB_DIR):
// hashed /assets for a year, the shell and service worker never cached, and
// index.html for any other path (hash routing keeps real paths few).
func frontend(dir string) (fibhttp.HandlerFunc, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	index := filepath.Join(abs, "index.html")
	return func(c *fibhttp.Context) {
		h := c.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		p := filepath.Clean("/" + c.Request.URL.Path)
		file := filepath.Join(abs, p)
		if st, err := os.Stat(file); err != nil || st.IsDir() || !strings.HasPrefix(file, abs) {
			file = index
		}
		switch {
		case strings.HasPrefix(p, "/assets/"):
			h.Set("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasSuffix(p, ".webmanifest"):
			h.Set("Content-Type", "application/manifest+json")
			h.Set("Cache-Control", "no-cache")
		default:
			h.Set("Cache-Control", "no-cache")
		}
		http.ServeFile(c, c.Request, file)
	}, nil
}

// Package dbtest gives each test its own freshly migrated database on the
// local development Postgres (TEST_DATABASE_URL, default the hoacy-dev-db
// container). Tests that need it are skipped when no server answers.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/asyafalni/hoacy/backend/internal/db"
)

const bawaan = "postgres://postgres:dev@127.0.0.1:55432/hoacy?sslmode=disable"

func Baru(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = bawaan
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Skipf("tidak ada Postgres untuk tes (%v) — jalankan container hoacy-dev-db", err)
	}
	b := make([]byte, 6)
	rand.Read(b)
	nama := "uji_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "create database "+nama); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(base)
	u.Path = "/" + nama
	pool, err := db.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		admin.Exec(ctx, "drop database "+nama+" with (force)")
		admin.Close(ctx)
	})
	return pool
}

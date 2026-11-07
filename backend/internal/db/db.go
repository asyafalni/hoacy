// Package db owns the Postgres connection and the schema migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open connects a pool and checks it answers. One pool per process: the
// database is a TLS hop away, so connections are kept, never opened per request.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// Migrate brings the schema up to the newest migration in migrations/.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]*goose.MigrationResult, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	return provider(sqlDB).Up(ctx)
}

// Down rolls back the newest migration (development only).
func Down(ctx context.Context, pool *pgxpool.Pool) (*goose.MigrationResult, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	return provider(sqlDB).Down(ctx)
}

func provider(sqlDB *sql.DB) *goose.Provider {
	sub, _ := fs.Sub(migrations, "migrations")
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, sub)
	if err != nil {
		panic(err) // only fails on a malformed embedded migration — a build-time bug
	}
	return p
}

// migrate applies the schema migrations: `go run ./cmd/migrate` (up) or
// `go run ./cmd/migrate down` (roll back the newest one, development only).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/asyafalni/hoacy/backend/internal/config"
	"github.com/asyafalni/hoacy/backend/internal/db"
)

func main() {
	config.LoadDotEnv(".env")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if len(os.Args) > 1 && os.Args[1] == "down" {
		r, err := db.Down(ctx, pool)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("rolled back:", r)
		return
	}
	res, err := db.Migrate(ctx, pool)
	if err != nil {
		log.Fatal(err)
	}
	if len(res) == 0 {
		fmt.Println("schema already up to date")
	}
	for _, r := range res {
		fmt.Println("applied:", r)
	}
}

// sinkron runs the Sheet → Postgres sync from the command line — the same code
// as the admin page. Without -terapkan it only previews. Used to seed a local
// database from CSV exports and to rehearse the cutover:
//
//	go run ./cmd/sinkron -csv dev/sheet               # preview every tab
//	go run ./cmd/sinkron -csv dev/sheet -terapkan     # apply every tab, in order
//	go run ./cmd/sinkron -tab M-Impor2025 -terapkan   # one tab, from Google Sheets
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/asyafalni/hoacy/backend/internal/config"
	"github.com/asyafalni/hoacy/backend/internal/db"
	"github.com/asyafalni/hoacy/backend/internal/google"
	"github.com/asyafalni/hoacy/backend/internal/sinkron"
	"github.com/asyafalni/hoacy/backend/internal/store"
)

func main() {
	csvDir := flag.String("csv", "", "baca tab dari folder CSV ini (bukan Google Sheets)")
	tab := flag.String("tab", "", "satu tab saja (kosong = semua, berurutan)")
	terapkan := flag.Bool("terapkan", false, "terapkan perubahan (tanpa ini: pratinjau saja)")
	snapshot := flag.Bool("snapshot", true, "juga muat M-Opex dan M-SaldoAwal")
	flag.Parse()
	config.LoadDotEnv(".env")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool, err := db.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if _, err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}

	var sumber sinkron.Sumber
	if *csvDir != "" {
		sumber = sinkron.CSV{Dir: *csvDir}
	} else {
		k, err := google.New(ctx, google.Config{ClientID: os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"), RefreshToken: os.Getenv("GOOGLE_REFRESH_TOKEN"),
			SheetID: os.Getenv("SHEET_ID")})
		if err != nil {
			log.Fatal(err)
		}
		sumber = k
	}
	s := &sinkron.Sinkron{DB: pool, Sumber: sumber}

	tabs := []string{*tab}
	if *tab == "" {
		semua, err := sumber.DaftarTab(ctx)
		if err != nil {
			log.Fatal(err)
		}
		tabs = sinkron.BisaDisinkron(semua)
	}
	for _, t := range tabs {
		var p *sinkron.Pratinjau
		if *terapkan {
			p, err = s.Terapkan(ctx, t, nil, "cmd/sinkron")
		} else {
			p, err = s.Pratinjau(ctx, t)
		}
		if err != nil {
			log.Fatalf("%s: %v", t, err)
		}
		fmt.Printf("%-16s tambah %4d · ubah %4d · sama %4d · hilang %3d · galat %3d\n",
			t, len(p.Tambah), len(p.Ubah), p.Sama, len(p.Hilang), len(p.Galat))
		for _, g := range p.Galat {
			fmt.Printf("    baris %d: %s\n", g.Baris, g.Pesan)
		}
	}
	if *snapshot && *terapkan {
		st := &store.Store{DB: pool}
		opex, err := sinkron.BacaOpex(ctx, sumber)
		if err == nil {
			err = st.SimpanSnapshot(ctx, "M-Opex", opex)
		}
		if err == nil {
			var sa sinkron.SaldoAwal
			if sa, err = sinkron.BacaSaldoAwal(ctx, sumber); err == nil {
				err = st.SimpanSnapshot(ctx, "M-SaldoAwal", sa)
			}
		}
		if err != nil {
			log.Fatal("snapshot: ", err)
		}
		fmt.Println("M-Opex, M-SaldoAwal dimuat")
	}
}

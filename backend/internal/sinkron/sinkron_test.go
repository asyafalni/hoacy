package sinkron

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asyafalni/hoacy/backend/internal/auth"
	"github.com/asyafalni/hoacy/backend/internal/dbtest"
)

func TestSinkronDariCSV(t *testing.T) {
	pool := dbtest.Baru(t)
	ctx := context.Background()
	s := &Sinkron{DB: pool, Sumber: CSV{Dir: "testdata/sheet"}}

	terap := func(tab string, hapus ...string) *Pratinjau {
		t.Helper()
		p, err := s.Terapkan(ctx, tab, hapus, "Admin Uji")
		if err != nil {
			t.Fatalf("%s: %v", tab, err)
		}
		return p
	}

	// first import, in the documented order
	p := terap("M-Blok")
	if len(p.Tambah) != 2 {
		t.Errorf("M-Blok tambah = %d", len(p.Tambah))
	}
	p = terap("M-Rumah")
	if len(p.Tambah) != 3 || len(p.Galat) != 1 || !strings.Contains(p.Galat[0].Pesan, "N7-03") {
		t.Errorf("M-Rumah: tambah %d galat %+v", len(p.Tambah), p.Galat)
	}
	p = terap("M-RumahRiwayat")
	if len(p.Tambah) != 2 || len(p.Galat) != 1 { // N9-99 doesn't exist
		t.Errorf("M-RumahRiwayat: %+v", p)
	}
	terap("M-TarifVersi")
	p = terap("M-Impor2025")
	if len(p.Tambah) != 3 || len(p.Galat) != 1 { // bulan 13 refused; the duplicate Feb row is kept
		t.Errorf("M-Impor2025: tambah %d galat %+v", len(p.Tambah), p.Galat)
	}
	terap("L-Tunai")
	terap("L-Transfer")
	terap("L-Keputusan")
	terap("L-Setoran")
	terap("L-Pengeluaran")

	// the house PIN is the last 6 digits of the phone; the 3-digit Sheet PIN is gone
	var hash string
	pool.QueryRow(ctx, `select pin_hash from rumah where alamat = 'N7-09'`).Scan(&hash)
	if !auth.CocokPIN(hash, "900314") || auth.CocokPIN(hash, "314") {
		t.Error("PIN N7-09 bukan 6 digit terakhir telp")
	}
	var nonaktif *string
	pool.QueryRow(ctx, `select nonaktif_oleh from rumah where alamat = 'NBlvd-02'`).Scan(&nonaktif)
	if nonaktif == nil || *nonaktif != "Ibu Siti" {
		t.Error("NBlvd-02 harus nonaktif oleh Ibu Siti")
	}

	// the pembayaran view sees the synced rows exactly as D-Pembayaran did
	want := map[string]string{
		"N7-09 202501 impor": "sah", "N7-09 202502 impor": "sah", // + one dobel
		"N7-01 202509 tunai": "sah", "N7-01 202510 tunai": "cek",
		"N7-09 202503 transfer": "sah", "N7-09 202504 transfer": "pending",
	}
	rows, _ := pool.Query(ctx, `select alamat || ' ' || periode || ' ' || metode, keabsahan from pembayaran`)
	got := map[string][]string{}
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		got[k] = append(got[k], v)
	}
	for k, v := range want {
		if len(got[k]) == 0 || got[k][0] != v {
			t.Errorf("%s = %v, want %s", k, got[k], v)
		}
	}
	if g := got["N7-09 202502 impor"]; len(g) != 2 || g[1] != "dobel" {
		t.Errorf("baris impor ganda harus dobel: %v", g)
	}

	// syncing again changes nothing
	for _, tab := range []string{"M-Rumah", "M-Impor2025", "L-Tunai", "L-Keputusan"} {
		p, err := s.Pratinjau(ctx, tab)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Tambah)+len(p.Ubah)+len(p.Hilang) != 0 {
			t.Errorf("%s disinkron ulang tidak kosong: %+v", tab, p)
		}
	}

	// edit a row and drop another in the Sheet → ubah + hilang; hilang only goes if ticked
	dir := t.TempDir()
	for _, f := range []string{"M-Impor2025.csv"} {
		b, _ := os.ReadFile(filepath.Join("testdata/sheet", f))
		b = []byte(strings.Replace(string(b), "N7-09,1,250000,2025-01-05", "N7-09,1,250000,2025-01-06", 1))
		b = []byte(strings.Replace(string(b), "N7-09,2,250000,\nN7-09,2,250000,\n", "N7-09,2,250000,\n", 1))
		os.WriteFile(filepath.Join(dir, f), b, 0o600)
	}
	s.Sumber = CSV{Dir: dir}
	p, _ = s.Pratinjau(ctx, "M-Impor2025")
	if len(p.Ubah) != 1 || len(p.Hilang) != 1 || p.Hilang[0].Kunci != "M-Impor2025:N7-09:2#2" {
		t.Fatalf("pratinjau edit: %+v", p)
	}
	terap("M-Impor2025") // nothing ticked
	var n int
	pool.QueryRow(ctx, `select count(*) from kiriman where sumber_ref like 'M-Impor2025:%'`).Scan(&n)
	if n != 3 {
		t.Errorf("tanpa centang, baris hilang tetap ada: %d", n)
	}
	terap("M-Impor2025", "M-Impor2025:N7-09:2#2", "bukan-kunci-yang-ada")
	pool.QueryRow(ctx, `select count(*) from kiriman where sumber_ref like 'M-Impor2025:%'`).Scan(&n)
	if n != 2 {
		t.Errorf("setelah dicentang: %d", n)
	}
	pool.QueryRow(ctx, `select count(*) from sinkron_log`).Scan(&n)
	if n != 12 {
		t.Errorf("sinkron_log = %d", n)
	}
	pool.QueryRow(ctx, `select count(*) from audit where tabel = 'M-Rumah'`).Scan(&n)
	if n != 3 {
		t.Errorf("audit M-Rumah = %d", n)
	}
}

func TestBisaDisinkron(t *testing.T) {
	got := BisaDisinkron([]string{"D-API", "M-Impor2026", "M-Rumah", "L-Tunai", "M-Impor2023", "M-Opex"})
	if strings.Join(got, ",") != "M-Rumah,L-Tunai,M-Impor2023,M-Impor2026" {
		t.Error(got)
	}
}

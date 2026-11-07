package tagihan

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// testdata/golden.json is what src/lib/*.js computed over the mock dataset
// (scripts/golden-tagihan.mjs). The Go port must agree on every number.
type golden struct {
	Masukan struct {
		Rumah []struct {
			Alamat  string `json:"alamat"`
			Aktif   bool   `json:"aktif"`
			Lunas   []int  `json:"lunas"`
			Pending []int  `json:"pending"`
		} `json:"rumah"`
		Riwayat    []Riwayat   `json:"riwayat"`
		Tarif      []TarifRow  `json:"tarif"`
		Terkumpul  []Terkumpul `json:"terkumpul"`
		Opex       []Opex      `json:"opex"`
		Kas        int64       `json:"kas"`
		Rekening   int64       `json:"rekening"`
		Pembayaran [][]any     `json:"pembayaran"`
	} `json:"masukan"`
	Kasus []struct {
		Hari struct {
			Tahun int `json:"tahun"`
			Bulan int `json:"bulan"`
		} `json:"hari"`
		Rumah []struct {
			Alamat        string    `json:"alamat"`
			Mulai         *int      `json:"mulai"`
			Tarif         *int64    `json:"tarif"`
			TarifDiatur   bool      `json:"tarifDiatur"`
			Luas          *float64  `json:"luas"`
			Tipe          *string   `json:"tipe"`
			Tunggakan     int64     `json:"tunggakan"`
			Tertua        *int      `json:"tertua"`
			TunggakanList []Bulanan `json:"tunggakanList"`
			Kartu         []Kartu   `json:"kartu"`
		} `json:"rumah"`
		Ringkasan json.RawMessage `json:"ringkasan"`
	} `json:"kasus"`
	RingkasTahun []struct {
		Tahun int             `json:"tahun"`
		Hasil json.RawMessage `json:"hasil"`
	} `json:"ringkasTahun"`
}

func muat(t *testing.T) golden {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func nol(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// sama compares v with a JSON value after a JSON round-trip of v, so field
// names and number shapes are checked too.
func sama(t *testing.T, apa string, v any, want json.RawMessage) {
	t.Helper()
	b, _ := json.Marshal(v)
	var got, exp any
	json.Unmarshal(b, &got)
	json.Unmarshal(want, &exp)
	if !reflect.DeepEqual(got, exp) {
		t.Errorf("%s berbeda:\n go: %s\n js: %s", apa, b, want)
	}
}

func TestSamaDenganJavaScript(t *testing.T) {
	g := muat(t)
	tr := NewTarif(g.Masukan.Riwayat, g.Masukan.Tarif)
	for _, k := range g.Kasus {
		sekarang := k.Hari.Tahun*100 + k.Hari.Bulan
		var houses []*Rumah
		for _, r := range g.Masukan.Rumah {
			if r.Aktif {
				houses = append(houses, Hitung(tr, r.Alamat, r.Lunas, r.Pending, sekarang))
			}
		}
		if len(houses) != len(k.Rumah) {
			t.Fatalf("%d: %d rumah, js %d", sekarang, len(houses), len(k.Rumah))
		}
		for i, want := range k.Rumah {
			h := houses[i]
			if h.Alamat != want.Alamat || h.Mulai != nol(want.Mulai) || h.Tertua != nol(want.Tertua) ||
				h.Tunggakan != want.Tunggakan || h.TarifDiatur != want.TarifDiatur ||
				!reflect.DeepEqual(h.Tarif, want.Tarif) || !reflect.DeepEqual(h.Luas, want.Luas) ||
				!reflect.DeepEqual(h.Tipe, want.Tipe) || !reflect.DeepEqual(h.TunggakanList, want.TunggakanList) {
				t.Errorf("%d %s:\n go %+v\n js %+v", sekarang, want.Alamat, *h, want)
				continue
			}
			if got := h.SemuaKartu(); !reflect.DeepEqual(got, want.Kartu) && !(len(got) == 0 && len(want.Kartu) == 0) {
				t.Errorf("%d %s kartu berbeda:\n go %v\n js %v", sekarang, want.Alamat, got, want.Kartu)
			}
		}
		sama(t, "ringkasan "+string(rune('0'+k.Hari.Bulan)), RingkasanPublik(tr, houses, g.Masukan.Terkumpul,
			g.Masukan.Opex, g.Masukan.Kas, g.Masukan.Rekening, sekarang), k.Ringkasan)
	}
}

func TestRingkasTahunSamaDenganJavaScript(t *testing.T) {
	g := muat(t)
	for _, rt := range g.RingkasTahun {
		var rows []Baris
		for _, r := range g.Masukan.Pembayaran { // D-Pembayaran column order
			if int(r[3].(float64)) != rt.Tahun {
				continue
			}
			rows = append(rows, Baris{Alamat: r[1].(string), Bulan: int(r[2].(float64)),
				Nominal: int64(r[4].(float64)), Metode: r[5].(string), Keabsahan: r[8].(string)})
		}
		sama(t, "ringkasTahun", RingkasTahun(rows), rt.Hasil)
	}
}

func TestGeser(t *testing.T) {
	for _, c := range [][3]int{{202601, -1, 202512}, {202612, 1, 202701}, {202609, -35, 202310}, {202001, -13, 201812}} {
		if got := Geser(c[0], c[1]); got != c[2] {
			t.Errorf("Geser(%d,%d) = %d, want %d", c[0], c[1], got, c[2])
		}
	}
}

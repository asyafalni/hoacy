package tagihan

import (
	"math"
	"sort"
)

// Terkumpul is the sah money for one dues month (what D-Riwayat held).
type Terkumpul struct {
	Tahun     int   `json:"tahun"`
	Bulan     int   `json:"bulan"`
	Terkumpul int64 `json:"terkumpul"`
}

// Opex is one M-Opex row (category totals only — never a person's wage).
type Opex struct {
	Kategori   string `json:"kategori"`
	Ikon       string `json:"ikon"`
	Nominal    int64  `json:"nominal"`
	Diperbarui string `json:"diperbarui"` // yyyy-mm-dd
}

type RiwayatBulan struct {
	Tahun     int   `json:"tahun"`
	Bulan     int   `json:"bulan"`
	Terkumpul int64 `json:"terkumpul"`
	Target    int64 `json:"target"`
}

type DibayarDimuka struct {
	Rumah             int   `json:"rumah"`
	BulanTahunIni     int   `json:"bulanTahunIni"`
	NominalTahunIni   int64 `json:"nominalTahunIni"`
	BulanTahunDepan   int   `json:"bulanTahunDepan"`
	NominalTahunDepan int64 `json:"nominalTahunDepan"`
	Total             int64 `json:"total"`
}

type AgingBucket struct {
	Label         string `json:"label"`
	Jumlah        int    `json:"jumlah"`
	Nominal       int64  `json:"nominal"`
	PersenRumah   int    `json:"persenRumah"`
	PersenNominal int    `json:"persenNominal"`
}

type Aging struct {
	JumlahRumah   int           `json:"jumlahRumah"`
	BelumDianggap int           `json:"belumDianggap"`
	JumlahAging   int           `json:"jumlahAging"`
	NominalAging  int64         `json:"nominalAging"`
	Distribusi    []AgingBucket `json:"distribusi"`
}

// Publik is everything /sum shows: cluster-wide aggregates, no house rows (PDP).
type Publik struct {
	JumlahRumah       int            `json:"jumlahRumah"`
	Tunggakan         int64          `json:"tunggakan"`
	Target            int64          `json:"target"`
	LunasBulanIni     int            `json:"lunasBulanIni"`
	TerkumpulBulanIni int64          `json:"terkumpulBulanIni"`
	Kas               int64          `json:"kas"`
	Rekening          int64          `json:"rekening"`
	Opex              int64          `json:"opex"`
	OpexDiperbarui    string         `json:"opexDiperbarui"`
	OpexList          []Opex         `json:"opexList"`
	Riwayat           []RiwayatBulan `json:"riwayat"`
	DibayarDimuka     DibayarDimuka  `json:"dibayarDimuka"`
	Aging             Aging          `json:"aging"`
}

// Aging buckets, counted only for houses already ≥ 3 months behind.
var buckets = []struct {
	label    string
	min, max int
}{
	{"3–6 bulan", 3, 6}, {"6–12 bulan", 6, 12}, {"1–3 tahun", 12, 36}, {"> 3 tahun", 36, math.MaxInt},
}

func persen(bagian, total float64) int {
	if total == 0 {
		return 0
	}
	return int(math.Floor(bagian/total*100 + 0.5)) // JS Math.round
}

// RingkasanPublik builds /sum from the active houses (already resolved with
// Hitung), the monthly collected totals, opex and the two balances.
func RingkasanPublik(t *Tarif, houses []*Rumah, terkumpul []Terkumpul, opex []Opex, kas, rekening int64, sekarang int) Publik {
	tahunIni := Tahun(sekarang)
	out := Publik{JumlahRumah: len(houses), Kas: kas, Rekening: rekening, OpexList: []Opex{}, Riwayat: []RiwayatBulan{}}
	for _, h := range houses {
		out.Tunggakan += h.Tunggakan
		if h.Tarif != nil {
			out.Target += *h.Tarif
		}
		if h.Lunas(sekarang) {
			out.LunasBulanIni++
		}
	}

	// Oldest month first; leading all-zero months trimmed. Each month's target
	// is every house's tarif in that month.
	bulanan := append([]Terkumpul(nil), terkumpul...)
	sort.SliceStable(bulanan, func(i, j int) bool {
		return bulanan[i].Tahun*100+bulanan[i].Bulan < bulanan[j].Tahun*100+bulanan[j].Bulan
	})
	awal := 0
	for awal < len(bulanan) && bulanan[awal].Terkumpul <= 0 {
		awal++
	}
	if awal == len(bulanan) {
		awal = 0
	}
	for _, b := range bulanan[awal:] {
		p := b.Tahun*100 + b.Bulan
		var target int64
		for _, h := range houses {
			if tr, ok := t.Pada(h.Alamat, p); ok {
				target += tr
			}
		}
		out.Riwayat = append(out.Riwayat, RiwayatBulan{b.Tahun, b.Bulan, b.Terkumpul, target})
		if p == sekarang {
			out.TerkumpulBulanIni = b.Terkumpul
		}
	}

	for _, o := range opex {
		if o.Kategori == "" {
			continue
		}
		if o.Ikon == "" {
			o.Ikon = "📋"
		}
		out.OpexList = append(out.OpexList, o)
		out.Opex += o.Nominal
		if o.Diperbarui > out.OpexDiperbarui {
			out.OpexDiperbarui = o.Diperbarui
		}
	}

	// Paid ahead: every sah month not yet due, at the tarif in force that month.
	m := &out.DibayarDimuka
	for _, h := range houses {
		depan := false
		for _, p := range h.LunasList() {
			if p <= sekarang {
				continue
			}
			depan = true
			tr, _ := h.TarifPer(p)
			if Tahun(p) == tahunIni {
				m.BulanTahunIni++
				m.NominalTahunIni += tr
			} else {
				m.BulanTahunDepan++
				m.NominalTahunDepan += tr
			}
		}
		if depan {
			m.Rumah++
		}
	}
	m.Total = m.NominalTahunIni + m.NominalTahunDepan

	// Aging: age runs from the oldest unpaid month to this month.
	type nunggak struct {
		umur    int
		nominal int64
	}
	var semua, aging []nunggak
	for _, h := range houses {
		if h.Tertua == 0 {
			continue
		}
		n := nunggak{Selisih(h.Tertua, sekarang) + 1, h.Tunggakan}
		semua = append(semua, n)
		if n.umur >= 3 {
			aging = append(aging, n)
			out.Aging.NominalAging += n.nominal
		}
	}
	out.Aging.JumlahRumah = len(semua)
	out.Aging.JumlahAging = len(aging)
	out.Aging.BelumDianggap = len(semua) - len(aging)
	for _, b := range buckets {
		bk := AgingBucket{Label: b.label}
		for _, n := range aging {
			if n.umur >= b.min && n.umur < b.max {
				bk.Jumlah++
				bk.Nominal += n.nominal
			}
		}
		bk.PersenRumah = persen(float64(bk.Jumlah), float64(len(aging)))
		bk.PersenNominal = persen(float64(bk.Nominal), float64(out.Aging.NominalAging))
		out.Aging.Distribusi = append(out.Aging.Distribusi, bk)
	}
	return out
}

// Baris is one row of the pembayaran view, as far as yearly summaries need it.
type Baris struct {
	Alamat    string `json:"alamat"`
	Bulan     int    `json:"bulan"`
	Nominal   int64  `json:"nominal"`
	Metode    string `json:"metode"`
	Keabsahan string `json:"keabsahan"`
}

type TahunBulan struct {
	Bulan int   `json:"bulan"`
	Total int64 `json:"total"`
	Rumah int   `json:"rumah"`
}

type Tahunan struct {
	Bulan     []TahunBulan     `json:"bulan"`
	PerMetode map[string]int64 `json:"perMetode"`
	Total     int64            `json:"total"`
}

// RingkasTahun is one dues year's summary from its pembayaran rows (sah only).
func RingkasTahun(rows []Baris) Tahunan {
	out := Tahunan{PerMetode: map[string]int64{}}
	rumah := make([]map[string]bool, 12)
	for i := range rumah {
		rumah[i] = map[string]bool{}
	}
	tot := make([]int64, 12)
	for _, r := range rows {
		if r.Keabsahan != "sah" || r.Bulan < 1 || r.Bulan > 12 {
			continue
		}
		out.PerMetode[r.Metode] += r.Nominal
		out.Total += r.Nominal
		tot[r.Bulan-1] += r.Nominal
		rumah[r.Bulan-1][r.Alamat] = true
	}
	for i := range 12 {
		out.Bulan = append(out.Bulan, TahunBulan{i + 1, tot[i], len(rumah[i])})
	}
	return out
}

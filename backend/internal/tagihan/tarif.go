// Package tagihan is the billing logic: which tarif applies to a house in a
// month, which months it owes, and the cluster-wide numbers for /sum. Pure
// functions over plain data — no database, no HTTP — so it is tested on its own
// against the JavaScript it replaces (testdata/golden.json, made by
// scripts/golden-tagihan.mjs from src/lib/*.js).
//
// A periode is yyyymm (202609). Money is whole rupiah.
package tagihan

import (
	"math"
	"sort"
)

func Tahun(p int) int { return p / 100 }
func Bulan(p int) int { return p % 100 }

// Geser is the periode n months after p (n may be negative).
func Geser(p, n int) int {
	idx := Tahun(p)*12 + (Bulan(p) - 1) + n
	y := idx / 12
	m := idx % 12
	if m < 0 { // floor division for negative idx
		y--
		m += 12
	}
	return y*100 + m + 1
}

// Selisih is the number of whole months from a to b (b - a).
func Selisih(a, b int) int { return (Tahun(b)-Tahun(a))*12 + (Bulan(b) - Bulan(a)) }

// Riwayat is one rumah_riwayat row: from `Berlaku` on, the house is this size.
type Riwayat struct {
	Alamat  string  `json:"alamat"`
	Berlaku int     `json:"berlaku"`
	Luas    float64 `json:"luas"`
	Tipe    string  `json:"tipe"` // "rumah" | "kavling"
}

// TarifRow is one tarif_versi row. Every row sharing a Berlaku is one rate card.
type TarifRow struct {
	Berlaku  int     `json:"berlaku"`
	Komponen string  `json:"komponen"` // islk_rumah | islk_kavling | iuran_rt
	LuasMin  float64 `json:"luasMin"`
	Nominal  int64   `json:"nominal"`
}

type rateCard struct {
	berlaku      int
	tiers        [][2]float64 // [luasMin, nominal], ascending luasMin
	kavlingPerM2 *int64
	iuranRt      int64
}

// Tarif answers "what did house X owe in month P", from the append-only history.
// Build it once per request from the rows; lookups are cheap.
type Tarif struct {
	riwayat map[string][]Riwayat // per alamat, ascending Berlaku
	cards   []rateCard           // ascending berlaku
}

func NewTarif(riwayat []Riwayat, tarif []TarifRow) *Tarif {
	t := &Tarif{riwayat: map[string][]Riwayat{}}
	for _, r := range riwayat {
		t.riwayat[r.Alamat] = append(t.riwayat[r.Alamat], r)
	}
	for _, rs := range t.riwayat {
		sort.Slice(rs, func(i, j int) bool { return rs[i].Berlaku < rs[j].Berlaku })
	}
	byVersi := map[int]*rateCard{}
	for _, r := range tarif {
		c := byVersi[r.Berlaku]
		if c == nil {
			c = &rateCard{berlaku: r.Berlaku}
			byVersi[r.Berlaku] = c
		}
		switch r.Komponen {
		case "islk_rumah":
			c.tiers = append(c.tiers, [2]float64{r.LuasMin, float64(r.Nominal)})
		case "islk_kavling":
			if c.kavlingPerM2 == nil { // first row wins, as in the JS find()
				n := r.Nominal
				c.kavlingPerM2 = &n
			}
		case "iuran_rt":
			c.iuranRt = r.Nominal
		}
	}
	for _, c := range byVersi {
		sort.SliceStable(c.tiers, func(i, j int) bool { return c.tiers[i][0] < c.tiers[j][0] })
		t.cards = append(t.cards, *c)
	}
	sort.Slice(t.cards, func(i, j int) bool { return t.cards[i].berlaku < t.cards[j].berlaku })
	return t
}

// LuasTipe is the house's size and kind in month p, or ok=false before its first row.
func (t *Tarif) LuasTipe(alamat string, p int) (luas float64, tipe string, ok bool) {
	rs := t.riwayat[alamat]
	for i := len(rs) - 1; i >= 0; i-- {
		if rs[i].Berlaku <= p {
			return rs[i].Luas, rs[i].Tipe, true
		}
	}
	return 0, "", false
}

// Mulai is the house's first billed periode (its first riwayat row), or 0.
func (t *Tarif) Mulai(alamat string) int {
	if rs := t.riwayat[alamat]; len(rs) > 0 {
		return rs[0].Berlaku
	}
	return 0
}

func (t *Tarif) card(p int) *rateCard {
	for i := len(t.cards) - 1; i >= 0; i-- {
		if t.cards[i].berlaku <= p {
			return &t.cards[i]
		}
	}
	return nil
}

// RateCard is the ISLK/iuran RT split in force in month p, for the warga card.
type RateCard struct {
	Rumah        [][2]float64 `json:"rumah"`
	KavlingPerM2 *int64       `json:"kavlingPerM2"`
	IuranRt      int64        `json:"iuranRt"`
}

func (t *Tarif) RateCard(p int) *RateCard {
	c := t.card(p)
	if c == nil {
		return nil
	}
	return &RateCard{Rumah: c.tiers, KavlingPerM2: c.kavlingPerM2, IuranRt: c.iuranRt}
}

// Pada is the full monthly tarif (ISLK + iuran RT) of a house in month p, or
// ok=false when it has no riwayat yet, no rate card is in force, or the card has
// no rule for it — such a month is never billed (never as Rp0).
func (t *Tarif) Pada(alamat string, p int) (int64, bool) {
	luas, tipe, ok := t.LuasTipe(alamat, p)
	if !ok {
		return 0, false
	}
	c := t.card(p)
	if c == nil {
		return 0, false
	}
	var islk float64
	if tipe == "kavling" {
		if c.kavlingPerM2 == nil {
			return 0, false
		}
		islk = float64(*c.kavlingPerM2) * luas
	} else {
		found := false
		for _, tier := range c.tiers {
			if luas >= tier[0] {
				islk, found = tier[1], true
			}
		}
		if !found {
			return 0, false
		}
	}
	return int64(math.Round(islk)) + c.iuranRt, true
}

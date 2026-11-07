package api

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"

	fibhttp "github.com/lesismal/fib/http"

	"github.com/asyafalni/hoacy/backend/internal/auth"
	"github.com/asyafalni/hoacy/backend/internal/berkas"
	"github.com/asyafalni/hoacy/backend/internal/store"
	"github.com/asyafalni/hoacy/backend/internal/tagihan"
)

// POST /api/warga/masuk {alamat, pin} → {token}. Unknown house, inactive
// house and wrong PIN all answer the same; every attempt is logged.
func (a *App) wargaMasuk(c *fibhttp.Context) {
	var req struct{ Alamat, PIN string }
	if !a.baca(c, &req) {
		return
	}
	ctx := c.Context()
	ip := a.ip(c)
	p, err := a.Store.Percobaan(ctx, req.Alamat, ip)
	if err != nil {
		a.galat(c, err)
		return
	}
	if kunci, sampai := p.Terkunci(a.Now()); kunci {
		a.json(c, 429, map[string]any{"ok": false, "kode": "terkunci", "sampai": sampai,
			"pesan": "Terlalu banyak percobaan. Coba lagi pukul " + sampai.In(store.WIB).Format("15:04") + "."})
		return
	}
	hash, err := a.Store.PINRumah(ctx, req.Alamat)
	ada := err == nil
	if errors.Is(err, store.ErrTidakAda) {
		hash = auth.HashPalsu // same work as a real check: no enumeration by timing
	} else if err != nil {
		a.galat(c, err)
		return
	}
	cocok := auth.CekFormatPIN(req.PIN) == nil && auth.CocokPIN(hash, req.PIN) && ada
	if err := a.Store.CatatLogin(ctx, req.Alamat, cocok, ip, c.Request.UserAgent()); err != nil {
		a.galat(c, err)
		return
	}
	if !cocok {
		a.gagal(c, 401, "pin", "Alamat atau PIN salah.")
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "token": a.Sesi.Buat(req.Alamat, hash, a.Now())})
}

// warga resolves the house behind the request's session token.
func (a *App) warga(c *fibhttp.Context) (alamat, hash string, ok bool) {
	alamat, sidik, err := a.Sesi.Baca(bearer(c), a.Now())
	if err == nil {
		hash, err = a.Store.PINRumah(c.Context(), alamat)
	}
	if err != nil || auth.Sidik(hash) != sidik {
		a.gagal(c, 401, "sesi", "Silakan masuk lagi dengan PIN rumah.")
		return "", "", false
	}
	return alamat, hash, true
}

type wargaRumah struct {
	*tagihan.Rumah
	Nama     string            `json:"nama"`
	Cluster  string            `json:"cluster"`
	Blok     string            `json:"blok"`
	Nomor    string            `json:"rumah"`
	Kartu    []tagihan.Kartu   `json:"kartu"`
	Muka     []tagihan.Bulanan `json:"muka"` // months that may be paid ahead (to December next year)
	Lunas    []int             `json:"lunas"`
	Pending  []int             `json:"pending"`
	RateCard *rateCardWarga    `json:"rateCard"`
}

type rateCardWarga struct {
	ISLK    int64 `json:"islk"`
	IuranRt int64 `json:"iuranRt"`
}

func bangunWarga(d *store.Dasar, sekarang int) wargaRumah {
	t, hs := d.Hitung(sekarang)
	h, r := hs[0], d.Rumah[0]
	w := wargaRumah{Rumah: h, Nama: r.Nama, Cluster: r.Cluster, Blok: r.Blok, Nomor: r.Nomor,
		Kartu: h.SemuaKartu(), Muka: bisaMuka(h, sekarang), Lunas: h.LunasList(), Pending: h.PendingList()}
	if w.Lunas == nil {
		w.Lunas = []int{}
	}
	if w.Pending == nil {
		w.Pending = []int{}
	}
	if h.Tarif != nil {
		if rc := t.RateCard(sekarang); rc != nil {
			w.RateCard = &rateCardWarga{ISLK: *h.Tarif - rc.IuranRt, IuranRt: rc.IuranRt}
		}
	}
	return w
}

// bisaMuka: future months up to December next year that have a tarif and
// aren't paid or waiting already.
func bisaMuka(h *tagihan.Rumah, sekarang int) []tagihan.Bulanan {
	out := []tagihan.Bulanan{}
	akhir := (tagihan.Tahun(sekarang)+1)*100 + 12
	for p := tagihan.Geser(sekarang, 1); p <= akhir; p = tagihan.Geser(p, 1) {
		if s := h.Status(p); s == tagihan.Lunas || s == tagihan.Pending {
			continue
		}
		if tr, ok := h.TarifPer(p); ok {
			out = append(out, tagihan.Bulanan{Periode: p, Tarif: tr})
		}
	}
	return out
}

// GET /api/warga — the signed-in house's own card. Nothing about any other house.
func (a *App) wargaData(c *fibhttp.Context) {
	alamat, _, ok := a.warga(c)
	if !ok {
		return
	}
	d, err := a.Store.DasarRumah(c.Context(), alamat)
	if errors.Is(err, store.ErrTidakAda) {
		a.gagal(c, 401, "sesi", "Rumah ini tidak lagi terdaftar.")
		return
	} else if err != nil {
		a.galat(c, err)
		return
	}
	var bawaan bool
	a.Store.DB.QueryRow(c.Context(), `select pin_diganti_at is null from rumah where alamat = $1`, alamat).Scan(&bawaan)
	a.json(c, 200, map[string]any{"ok": true, "hari": a.hari(), "rumah": bangunWarga(d, a.sekarang()),
		"warna": d.Warna[d.Rumah[0].Blok], "pinBawaan": bawaan})
}

// POST /api/warga/pin {lama, baru} → a new token; every other phone of this
// house is signed out.
func (a *App) wargaGantiPIN(c *fibhttp.Context) {
	alamat, hash, ok := a.warga(c)
	if !ok {
		return
	}
	var req struct{ Lama, Baru string }
	if !a.baca(c, &req) {
		return
	}
	switch {
	case !auth.CocokPIN(hash, req.Lama):
		a.gagal(c, 400, "pin-lama", "PIN lama salah.")
		return
	case auth.CekFormatPIN(req.Baru) != nil:
		a.gagal(c, 400, "pin-format", "PIN baru harus 6 angka.")
		return
	case req.Baru == req.Lama:
		a.gagal(c, 400, "pin-sama", "PIN baru harus berbeda dari yang lama.")
		return
	}
	baru, err := auth.HashPIN(req.Baru)
	if err == nil {
		err = a.Store.GantiPIN(c.Context(), alamat, baru, true)
	}
	if err != nil {
		a.galat(c, err)
		return
	}
	a.json(c, 200, map[string]any{"ok": true, "token": a.Sesi.Buat(alamat, baru, a.Now())})
}

// validasiBulan turns the chosen periodes into owed months at their tarif, or
// says why not. Past and current months must be owed ("Belum"); future ones
// must have a tarif, not be paid or pending, and fall before next January+12.
func validasiBulan(h *tagihan.Rumah, periodes []int, sekarang int) ([]tagihan.Bulanan, string) {
	if len(periodes) == 0 {
		return nil, "Pilih minimal satu bulan."
	}
	if len(periodes) > 240 {
		return nil, "Terlalu banyak bulan dalam satu kiriman."
	}
	sort.Ints(periodes)
	akhir := (tagihan.Tahun(sekarang)+1)*100 + 12
	out := make([]tagihan.Bulanan, 0, len(periodes))
	for i, p := range periodes {
		if i > 0 && p == periodes[i-1] {
			return nil, "Bulan yang sama dipilih dua kali."
		}
		label := fmt.Sprintf("%02d/%d", tagihan.Bulan(p), tagihan.Tahun(p))
		tr, ok := h.TarifPer(p)
		switch st := h.Status(p); {
		case st == tagihan.Lunas:
			return nil, label + " sudah lunas."
		case st == tagihan.Pending:
			return nil, label + " sedang menunggu verifikasi."
		case !ok || (h.Mulai != 0 && p < h.Mulai):
			return nil, label + " tidak ditagih."
		case p <= sekarang && st != tagihan.Belum:
			return nil, label + " tidak ditagih."
		case p > akhir:
			return nil, label + " terlalu jauh ke depan."
		}
		out = append(out, tagihan.Bulanan{Periode: p, Tarif: tr})
	}
	return out, ""
}

func periodesDari(vals []string) ([]int, bool) {
	out := make([]int, 0, len(vals))
	for _, v := range vals {
		p, err := strconv.Atoi(v)
		if err != nil || p%100 < 1 || p%100 > 12 {
			return nil, false
		}
		out = append(out, p)
	}
	return out, true
}

// POST /api/warga/transfer (multipart: periode…, bukti) — one confirmation
// with its proof photo; it waits as pending until the bendahara verifies it.
func (a *App) wargaTransfer(c *fibhttp.Context) {
	alamat, _, ok := a.warga(c)
	if !ok {
		return
	}
	ctx := c.Context()
	if err := c.Request.ParseMultipartForm(berkas.MaksUkuran + (1 << 20)); err != nil {
		a.gagal(c, 400, "request", "Unggahan tidak terbaca (maks. 10 MB).")
		return
	}
	periodes, ok := periodesDari(c.Request.MultipartForm.Value["periode"])
	if !ok {
		a.gagal(c, 400, "bulan", "Bulan tidak valid.")
		return
	}
	f, _, err := c.Request.FormFile("bukti")
	if err != nil {
		a.gagal(c, 400, "bukti", "Lampirkan foto bukti transfer.")
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, berkas.MaksUkuran+1))
	f.Close()
	if err != nil {
		a.galat(c, err)
		return
	}
	mime, err := berkas.Periksa(data)
	if err != nil {
		a.gagal(c, 400, "bukti", err.Error())
		return
	}
	d, err := a.Store.DasarRumah(ctx, alamat)
	if err != nil {
		a.galat(c, err)
		return
	}
	sekarang := a.sekarang()
	_, hs := d.Hitung(sekarang)
	bulan, alasan := validasiBulan(hs[0], periodes, sekarang)
	if alasan != "" {
		a.gagal(c, 400, "bulan", alasan)
		return
	}
	now := a.Now()
	ref, err := a.Berkas.Simpan(ctx, fmt.Sprintf("bukti-%s-%s", alamat, now.In(store.WIB).Format("20060102-150405")), mime, data)
	if err != nil {
		a.galat(c, err)
		return
	}
	id, err := a.Store.CatatKiriman(ctx, store.KirimanBaru{Alamat: alamat, Metode: "transfer", Waktu: now,
		Petugas: "Warga", BuktiURL: ref, Bulan: bulan})
	if errors.Is(err, store.ErrSudahDibayar) {
		a.gagal(c, 409, "bulan", "Sebagian bulan sudah dibayar atau sedang diverifikasi — muat ulang kartu.")
		return
	} else if err != nil {
		a.galat(c, err)
		return
	}
	a.lupakanPublik()
	a.json(c, 200, map[string]any{"ok": true, "kirimanId": id, "total": total(bulan)})
}

func total(b []tagihan.Bulanan) int64 {
	var n int64
	for _, x := range b {
		n += x.Tarif
	}
	return n
}

package tagihan

// Status of one month on a house's card.
const (
	Lunas   = "Lunas"
	Pending = "Pending"
	Belum   = "Belum"
	Kosong  = "-" // not billed: before billing began, not due yet, or no rate
)

// Bulanan is one owed month.
type Bulanan struct {
	Periode int   `json:"periode"`
	Tarif   int64 `json:"tarif"`
}

// Kartu is a house's 12-month card for one year. Tarif[i] is nil for a month
// without a tarif.
type Kartu struct {
	Tahun      int      `json:"tahun"`
	Status     []string `json:"status"`
	TarifBulan []*int64 `json:"tarifBulan"`
}

// Rumah is everything the screens show about one house's dues as of Sekarang.
type Rumah struct {
	Alamat        string    `json:"alamat"`
	Mulai         int       `json:"mulai,omitempty"` // 0 = no riwayat
	Tarif         *int64    `json:"tarif"`           // this month's; nil = "Tarif belum diatur"
	TarifDiatur   bool      `json:"tarifDiatur"`
	Luas          *float64  `json:"luas"`
	Tipe          *string   `json:"tipe"`
	Tunggakan     int64     `json:"tunggakan"`
	Tertua        int       `json:"tertua,omitempty"` // oldest owed periode
	TunggakanList []Bulanan `json:"tunggakanList"`

	lunas, pending map[int]bool
	t              *Tarif
	sekarang       int
}

// Hitung resolves one house. lunas/pending are its periodes with a sah/pending
// payment (the `pembayaran` view); sekarang is this month.
func Hitung(t *Tarif, alamat string, lunas, pending []int, sekarang int) *Rumah {
	h := &Rumah{Alamat: alamat, Mulai: t.Mulai(alamat), t: t, sekarang: sekarang,
		lunas: set(lunas), pending: set(pending), TunggakanList: []Bulanan{}}
	if tr, ok := t.Pada(alamat, sekarang); ok {
		h.Tarif, h.TarifDiatur = &tr, true
	}
	if luas, tipe, ok := t.LuasTipe(alamat, sekarang); ok {
		h.Luas, h.Tipe = &luas, &tipe
	}
	if h.Mulai != 0 {
		for p := h.Mulai; p <= sekarang; p = Geser(p, 1) {
			if h.Status(p) == Belum {
				tr, _ := t.Pada(alamat, p)
				h.TunggakanList = append(h.TunggakanList, Bulanan{p, tr})
				h.Tunggakan += tr
			}
		}
	}
	if len(h.TunggakanList) > 0 {
		h.Tertua = h.TunggakanList[0].Periode
	}
	return h
}

// Status of month p. Pending transfers aren't owed from the resident's side.
func (h *Rumah) Status(p int) string {
	switch {
	case h.lunas[p]:
		return Lunas
	case h.pending[p]:
		return Pending
	}
	if h.Mulai == 0 || p < h.Mulai || p > h.sekarang {
		return Kosong
	}
	if _, ok := h.t.Pada(h.Alamat, p); !ok {
		return Kosong
	}
	return Belum
}

// TarifPer is the house's tarif in month p (any month, also ahead).
func (h *Rumah) TarifPer(p int) (int64, bool) { return h.t.Pada(h.Alamat, p) }

func (h *Rumah) Lunas(p int) bool { return h.lunas[p] }

// LunasList / PendingList, ascending.
func (h *Rumah) LunasList() []int   { return sorted(h.lunas) }
func (h *Rumah) PendingList() []int { return sorted(h.pending) }

// KartuTahun is the 12-month card for any year.
func (h *Rumah) KartuTahun(tahun int) Kartu {
	k := Kartu{Tahun: tahun, Status: make([]string, 12), TarifBulan: make([]*int64, 12)}
	for i := range 12 {
		p := tahun*100 + i + 1
		k.Status[i] = h.Status(p)
		if tr, ok := h.TarifPer(p); ok {
			k.TarifBulan[i] = &tr
		}
	}
	return k
}

// SemuaKartu is every card from the year billing began to next year (paying
// ahead reaches into next year). Empty when the house has no riwayat.
func (h *Rumah) SemuaKartu() []Kartu {
	out := []Kartu{}
	if h.Mulai == 0 {
		return out
	}
	for y := Tahun(h.Mulai); y <= Tahun(h.sekarang)+1; y++ {
		out = append(out, h.KartuTahun(y))
	}
	return out
}

func set(ps []int) map[int]bool {
	m := make(map[int]bool, len(ps))
	for _, p := range ps {
		m[p] = true
	}
	return m
}

func sorted(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	for i := 1; i < len(out); i++ { // small sets; insertion sort keeps deps nil
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

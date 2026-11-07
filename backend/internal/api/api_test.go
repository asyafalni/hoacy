package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	fib "github.com/lesismal/fib"
	fibhttp "github.com/lesismal/fib/http"

	"github.com/asyafalni/hoacy/backend/internal/auth"
	"github.com/asyafalni/hoacy/backend/internal/berkas"
	"github.com/asyafalni/hoacy/backend/internal/dbtest"
	"github.com/asyafalni/hoacy/backend/internal/sinkron"
	"github.com/asyafalni/hoacy/backend/internal/store"
)

// server seeds a fresh database from the sync fixtures and serves the API on
// a random port through fib — the real stack, minus Google and Clerk.
func server(t *testing.T) (base string, st *store.Store) {
	t.Helper()
	pool := dbtest.Baru(t)
	ctx := context.Background()
	s := &sinkron.Sinkron{DB: pool, Sumber: sinkron.CSV{Dir: "../sinkron/testdata/sheet"}}
	for _, tab := range []string{"M-Blok", "M-Rumah", "M-RumahRiwayat", "M-TarifVersi", "M-Impor2025"} {
		if _, err := s.Terapkan(ctx, tab, nil, "uji"); err != nil {
			t.Fatal(tab, err)
		}
	}
	st = &store.Store{DB: pool}
	for _, p := range []store.PengurusRow{{Email: "admin@x.id", Nama: "Admin", Peran: "admin", Aktif: true},
		{Email: "bendahara@x.id", Nama: "Ibu Siti", Peran: "bendahara", Aktif: true},
		{Email: "komite@x.id", Nama: "Pak Komite", Peran: "komite", Aktif: true}} {
		if err := st.SimpanPengurus(ctx, p, "uji"); err != nil {
			t.Fatal(err)
		}
	}
	hari := time.Date(2025, 6, 15, 10, 0, 0, 0, store.WIB)
	app := &App{Store: st, Sesi: auth.NewSesi(strings.Repeat("k", 32)), Penjaga: auth.Dev{Cari: st.CariPengurus},
		Berkas: berkas.Disk{Dir: t.TempDir()}, Now: func() time.Time { return hari }, Log: slog.New(slog.DiscardHandler)}
	engine, err := fib.New(fib.DefaultConfig(), fib.Binding{Addr: "127.0.0.1:0", Handler: fibhttp.NewHandler(app.Router(nil))})
	if err != nil {
		t.Fatal(err)
	}
	go engine.Run()
	t.Cleanup(func() { engine.Close() })
	var addr string
	for range 50 {
		if as, err := engine.ListenAddrs(); err == nil && len(as) > 0 {
			addr = as[0].String()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "http://" + addr, st
}

type jawaban struct {
	status int
	body   map[string]any
}

func kirim(t *testing.T, method, url string, hdr map[string]string, body any) jawaban {
	t.Helper()
	var r io.Reader
	ct := "application/json"
	switch b := body.(type) {
	case nil:
	case *multipartBody:
		r, ct = b.buf, b.ct
	default:
		j, _ := json.Marshal(b)
		r = bytes.NewReader(j)
	}
	req, _ := http.NewRequest(method, url, r)
	req.Header.Set("Content-Type", ct)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var m map[string]any
	json.NewDecoder(res.Body).Decode(&m)
	return jawaban{res.StatusCode, m}
}

type multipartBody struct {
	buf *bytes.Buffer
	ct  string
}

func formTransfer(periode []int, file []byte) *multipartBody {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	for _, p := range periode {
		w.WriteField("periode", fmt.Sprint(p))
	}
	if file != nil {
		f, _ := w.CreateFormFile("bukti", "struk.jpg")
		f.Write(file)
	}
	w.Close()
	return &multipartBody{buf, w.FormDataContentType()}
}

// a minimal valid JPEG header is enough for the sniffing check
var jpeg = append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0x10, 'J', 'F', 'I', 'F', 0}, bytes.Repeat([]byte{0}, 64)...)

func TestWarga(t *testing.T) {
	base, _ := server(t)
	masuk := func(alamat, pin string) jawaban {
		return kirim(t, "POST", base+"/api/warga/masuk", nil, map[string]string{"alamat": alamat, "pin": pin})
	}

	// unknown house and wrong PIN look the same
	if a, b := masuk("N9-99", "123456"), masuk("N7-09", "123456"); a.status != 401 || b.status != 401 ||
		a.body["pesan"] != b.body["pesan"] {
		t.Fatalf("alamat asing / PIN salah harus sama: %v %v", a, b)
	}
	if r := masuk("NBlvd-02", "000000"); r.status != 401 {
		t.Errorf("rumah nonaktif tidak boleh masuk: %v", r)
	}
	ok := masuk("N7-09", "900314") // last 6 digits of 6281299900314
	if ok.status != 200 {
		t.Fatalf("masuk: %v", ok)
	}
	tok := map[string]string{"Authorization": "Bearer " + ok.body["token"].(string)}

	d := kirim(t, "GET", base+"/api/warga", tok, nil)
	rumah := d.body["rumah"].(map[string]any)
	if rumah["alamat"] != "N7-09" || rumah["tarif"].(float64) != 250000 || d.body["pinBawaan"] != true {
		t.Fatalf("data warga: %v", d.body)
	}
	if _, bocor := rumah["telp"]; bocor {
		t.Error("telp tidak boleh dikirim ke warga")
	}
	// Jan + Feb 2025 are imported (sah), Mar–Jun owed
	if n := len(rumah["tunggakanList"].([]any)); n != 4 {
		t.Errorf("tunggakan = %d bulan, want 4", n)
	}

	// paying a month already paid, or with no proof / a fake file, is refused
	if r := kirim(t, "POST", base+"/api/warga/transfer", tok, formTransfer([]int{202501}, jpeg)); r.status != 400 {
		t.Errorf("bulan lunas harus ditolak: %v", r)
	}
	if r := kirim(t, "POST", base+"/api/warga/transfer", tok, formTransfer([]int{202503}, []byte("bukan gambar"))); r.status != 400 {
		t.Errorf("berkas palsu harus ditolak: %v", r)
	}
	if r := kirim(t, "POST", base+"/api/warga/transfer", tok, formTransfer([]int{202503}, nil)); r.status != 400 {
		t.Errorf("tanpa bukti harus ditolak: %v", r)
	}
	if r := kirim(t, "POST", base+"/api/warga/transfer", tok, formTransfer([]int{202812}, jpeg)); r.status != 400 {
		t.Errorf("terlalu jauh ke depan harus ditolak: %v", r)
	}
	tr := kirim(t, "POST", base+"/api/warga/transfer", tok, formTransfer([]int{202503, 202504}, jpeg))
	if tr.status != 200 || tr.body["total"].(float64) != 500000 {
		t.Fatalf("transfer: %v", tr)
	}
	// the same months again while pending → refused
	if r := kirim(t, "POST", base+"/api/warga/transfer", tok, formTransfer([]int{202504}, jpeg)); r.status != 400 {
		t.Errorf("bulan pending harus ditolak: %v", r)
	}

	// changing the PIN signs every old token out
	if r := kirim(t, "POST", base+"/api/warga/pin", tok, map[string]string{"lama": "900314", "baru": "12345"}); r.status != 400 {
		t.Errorf("PIN 5 digit harus ditolak: %v", r)
	}
	g := kirim(t, "POST", base+"/api/warga/pin", tok, map[string]string{"lama": "900314", "baru": "481516"})
	if g.status != 200 {
		t.Fatalf("ganti PIN: %v", g)
	}
	if r := kirim(t, "GET", base+"/api/warga", tok, nil); r.status != 401 {
		t.Errorf("token lama harus mati setelah ganti PIN: %v", r)
	}
	if r := masuk("N7-09", "481516"); r.status != 200 {
		t.Errorf("PIN baru: %v", r)
	}
}

func TestLockout(t *testing.T) {
	base, st := server(t)
	for i := range 5 {
		r := kirim(t, "POST", base+"/api/warga/masuk", nil, map[string]string{"alamat": "N7-01", "pin": fmt.Sprintf("00000%d", i)})
		if r.status != 401 {
			t.Fatalf("percobaan %d: %v", i, r)
		}
	}
	r := kirim(t, "POST", base+"/api/warga/masuk", nil, map[string]string{"alamat": "N7-01", "pin": "500001"})
	if r.status != 429 || r.body["kode"] != "terkunci" {
		t.Fatalf("PIN benar pun ditolak saat terkunci: %v", r)
	}
	// another house is unaffected
	if r := kirim(t, "POST", base+"/api/warga/masuk", nil, map[string]string{"alamat": "N7-09", "pin": "900314"}); r.status != 200 {
		t.Errorf("rumah lain ikut terkunci: %v", r)
	}
	var n int
	st.DB.QueryRow(context.Background(), `select count(*) from login_warga where not berhasil`).Scan(&n)
	if n != 5 {
		t.Errorf("percobaan gagal tercatat %d, want 5", n)
	}
	k := kirim(t, "GET", base+"/api/admin/keamanan", map[string]string{"X-Dev-Email": "admin@x.id"}, nil)
	if k.status != 200 || k.body["keamanan"].(map[string]any)["gagal24j"].(float64) != 5 {
		t.Errorf("halaman keamanan: %v", k)
	}
}

func TestKas(t *testing.T) {
	base, _ := server(t)
	admin := map[string]string{"X-Dev-Email": "admin@x.id"}
	bend := map[string]string{"X-Dev-Email": "bendahara@x.id"}
	komite := map[string]string{"X-Dev-Email": "komite@x.id"}

	if r := kirim(t, "GET", base+"/api/kas", nil, nil); r.status != 401 {
		t.Errorf("tanpa login: %v", r)
	}
	if r := kirim(t, "GET", base+"/api/kas", map[string]string{"X-Dev-Email": "orang@lain.id"}, nil); r.status != 403 {
		t.Errorf("bukan pengurus: %v", r)
	}
	k := kirim(t, "GET", base+"/api/kas", komite, nil)
	if k.status != 200 || len(k.body["rumah"].([]any)) != 2 { // NBlvd-02 is inactive
		t.Fatalf("kas komite: %v", k.status)
	}
	if r := kirim(t, "POST", base+"/api/kas/tunai", komite, map[string]any{"alamat": "N7-01", "periode": []int{202503}}); r.status != 403 {
		t.Errorf("komite tidak boleh mencatat: %v", r)
	}

	// cash: recorded at its tarif, counted at once; the same month twice is refused
	tn := kirim(t, "POST", base+"/api/kas/tunai", bend, map[string]any{"alamat": "N7-01", "periode": []int{202503}})
	if tn.status != 200 || tn.body["total"].(float64) != 250000 {
		t.Fatalf("tunai: %v", tn)
	}
	if r := kirim(t, "POST", base+"/api/kas/tunai", bend, map[string]any{"alamat": "N7-01", "periode": []int{202503}}); r.status != 400 {
		t.Errorf("bulan yang sama dua kali: %v", r)
	}
	k = kirim(t, "GET", base+"/api/kas", bend, nil)
	if k.body["saldo"].(map[string]any)["kas"].(float64) != 250000 || len(k.body["tunai"].([]any)) != 1 {
		t.Errorf("saldo/riwayat tunai: %v / %v", k.body["saldo"], k.body["tunai"])
	}
	// void it → the month is owed again
	id := tn.body["kirimanId"].(float64)
	if r := kirim(t, "POST", base+"/api/kas/keputusan", bend, map[string]any{"kirimanId": id, "keputusan": "tolak"}); r.status != 200 {
		t.Fatal(r)
	}
	if r := kirim(t, "POST", base+"/api/kas/tunai", bend, map[string]any{"alamat": "N7-01", "periode": []int{202503}}); r.status != 200 {
		t.Errorf("setelah dibatalkan bulan bisa dicatat lagi: %v", r)
	}
	if r := kirim(t, "POST", base+"/api/kas/setoran", bend, map[string]any{"nominal": 250000}); r.status != 200 {
		t.Fatal(r)
	}
	k = kirim(t, "GET", base+"/api/kas", bend, nil)
	if s := k.body["saldo"].(map[string]any); s["kas"].(float64) != 0 || s["rekening"].(float64) != 250000 {
		t.Errorf("setelah setor: %v", s)
	}

	// deactivating needs the exact alamat typed; admin only
	if r := kirim(t, "POST", base+"/api/admin/rumah/N7-01/nonaktif", bend, map[string]string{"konfirmasi": "N7-01"}); r.status != 403 {
		t.Errorf("bendahara tidak boleh menonaktifkan: %v", r)
	}
	if r := kirim(t, "POST", base+"/api/admin/rumah/N7-01/nonaktif", admin, map[string]string{"konfirmasi": "N7-1"}); r.status != 400 {
		t.Errorf("konfirmasi salah ketik harus ditolak: %v", r)
	}
	if r := kirim(t, "POST", base+"/api/admin/rumah/N7-01/nonaktif", admin, map[string]string{"konfirmasi": "N7-01"}); r.status != 200 {
		t.Fatal(r)
	}
	k = kirim(t, "GET", base+"/api/kas", bend, nil)
	if n := len(k.body["rumah"].([]any)); n != 1 {
		t.Errorf("rumah nonaktif masih tampil: %d", n)
	}

	// the last admin can't demote themselves
	if r := kirim(t, "PUT", base+"/api/admin/pengurus/admin@x.id", admin, map[string]any{"nama": "Admin", "peran": "komite", "aktif": true}); r.status != 400 {
		t.Errorf("admin terakhir: %v", r)
	}

	// /sum carries no house rows at all
	p := kirim(t, "GET", base+"/api/publik", nil, nil)
	b, _ := json.Marshal(p.body)
	for _, rahasia := range []string{"N7-09", "Alfin", "6281299900314"} {
		if strings.Contains(string(b), rahasia) {
			t.Errorf("/api/publik membocorkan %q", rahasia)
		}
	}
}

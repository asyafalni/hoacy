// google-login gets the refresh token the server uses to act as the RT's
// Google account (Drive for proofs, Sheets read-only). Run it once on your
// own computer, signed in to that account in the browser:
//
//	go run ./cmd/google-login
//
// It needs GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET in backend/.env (an OAuth
// client of type "Desktop app"), opens a local page on 127.0.0.1:8085, and
// prints GOOGLE_REFRESH_TOKEN for you to put in backend/.env and in the
// DigitalOcean app's encrypted env vars. Nothing is sent anywhere else.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"

	"github.com/asyafalni/hoacy/backend/internal/config"
	"github.com/asyafalni/hoacy/backend/internal/google"
)

func main() {
	config.LoadDotEnv(".env")
	cfg := google.OAuth(google.Config{ClientID: os.Getenv("GOOGLE_CLIENT_ID"), ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET")})
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		log.Fatal("isi GOOGLE_CLIENT_ID dan GOOGLE_CLIENT_SECRET di backend/.env dulu")
	}
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)
	hasil := make(chan *oauth2.Token, 1)
	srv := &http.Server{Addr: "127.0.0.1:8085", ReadHeaderTimeout: 10 * time.Second}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state tidak cocok", 400)
			return
		}
		tok, err := cfg.Exchange(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "gagal menukar kode: "+err.Error(), 400)
			return
		}
		fmt.Fprintln(w, "Berhasil. Tutup tab ini dan kembali ke terminal.")
		hasil <- tok
	})
	go srv.ListenAndServe()
	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	fmt.Println("Buka link ini di browser yang login dengan akun Google RT:\n\n" + url + "\n")
	tok := <-hasil
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	srv.Shutdown(ctx)
	cancel()
	if tok.RefreshToken == "" {
		log.Fatal("Google tidak mengirim refresh token — cabut akses app ini di myaccount.google.com/permissions lalu ulangi")
	}
	fmt.Println("Tambahkan ke backend/.env (dan env var terenkripsi di DigitalOcean):\n\nGOOGLE_REFRESH_TOKEN=" + tok.RefreshToken)
}

// Package berkas stores proof photos (bukti transfer, nota pengeluaran).
// Production keeps them in Google Drive under the RT's own account; local
// development keeps them in a folder. Callers only see an opaque reference,
// stored on the row, and the bytes are only ever served back through the
// server to a signed-in pengurus.
package berkas

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Berkas interface {
	// Simpan stores data and returns its reference.
	Simpan(ctx context.Context, nama, mime string, data []byte) (string, error)
	// Buka returns the bytes and media type behind a reference.
	Buka(ctx context.Context, ref string) ([]byte, string, error)
}

const MaksUkuran = 10 << 20 // 10 MB — a phone photo of a struk is far smaller

var ErrJenis = errors.New("berkas harus foto (JPG, PNG, WebP, HEIC) atau PDF")

// Periksa sniffs the bytes — never trusting the browser's Content-Type — and
// returns the media type to store it as.
func Periksa(data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaksUkuran {
		return "", fmt.Errorf("ukuran berkas harus 1 byte – 10 MB")
	}
	switch t := http.DetectContentType(data); t {
	case "image/jpeg", "image/png", "image/webp", "application/pdf":
		return t, nil
	}
	if len(data) > 12 && string(data[4:8]) == "ftyp" {
		switch string(data[8:12]) {
		case "heic", "heix", "mif1", "msf1":
			return "image/heic", nil
		}
	}
	return "", ErrJenis
}

// IDDrive pulls a Drive file id out of a reference: "drive:<id>" (ours) or
// a share link the old Google Form stored ("https://drive.google.com/open?id=…",
// ".../file/d/<id>/view").
func IDDrive(ref string) (string, bool) {
	if id, ok := strings.CutPrefix(ref, "drive:"); ok {
		return id, id != ""
	}
	u, err := url.Parse(ref)
	if err != nil || !strings.HasSuffix(u.Host, "google.com") {
		return "", false
	}
	if id := u.Query().Get("id"); id != "" {
		return id, true
	}
	parts := strings.Split(u.Path, "/")
	for i, p := range parts {
		if p == "d" && i+1 < len(parts) && parts[i+1] != "" {
			return parts[i+1], true
		}
	}
	return "", false
}

// ── Local folder (development) ─────────────────────────────────────────────

type Disk struct{ Dir string }

func (d Disk) Simpan(_ context.Context, nama, mime string, data []byte) (string, error) {
	if err := os.MkdirAll(d.Dir, 0o700); err != nil {
		return "", err
	}
	b := make([]byte, 12)
	rand.Read(b)
	name := hex.EncodeToString(b) + ekstensi(mime)
	return "disk:" + name, os.WriteFile(filepath.Join(d.Dir, name), data, 0o600)
}

func (d Disk) Buka(_ context.Context, ref string) ([]byte, string, error) {
	name, ok := strings.CutPrefix(ref, "disk:")
	if !ok || name != filepath.Base(name) {
		return nil, "", os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(d.Dir, name))
	if err != nil {
		return nil, "", err
	}
	t, _ := Periksa(data)
	return data, t, nil
}

func ekstensi(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/heic":
		return ".heic"
	case "application/pdf":
		return ".pdf"
	}
	return ""
}

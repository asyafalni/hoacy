package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// A warga session: after alamat + PIN, the phone keeps a signed token instead
// of the PIN itself. It names the house and a fingerprint of the PIN hash, so
// changing the PIN (by the resident or an admin) signs every old token out.
type Sesi struct{ kunci []byte }

func NewSesi(secret string) *Sesi { return &Sesi{kunci: []byte(secret)} }

const umurSesi = 90 * 24 * time.Hour

var ErrSesi = errors.New("sesi tidak berlaku")

// Sidik is the PIN fingerprint carried in a token.
func Sidik(pinHash string) string {
	h := sha256.Sum256([]byte(pinHash))
	return base64.RawURLEncoding.EncodeToString(h[:6])
}

func (s *Sesi) Buat(alamat, pinHash string, now time.Time) string {
	isi := alamat + "|" + Sidik(pinHash) + "|" + strconv.FormatInt(now.Add(umurSesi).Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(isi)) + "." + s.tanda(isi)
}

// Baca checks a token's signature and expiry and returns its alamat and PIN
// fingerprint; the caller compares the fingerprint with the current hash.
func (s *Sesi) Baca(token string, now time.Time) (alamat, sidik string, err error) {
	a, tanda, ok := strings.Cut(token, ".")
	if !ok {
		return "", "", ErrSesi
	}
	raw, err := base64.RawURLEncoding.DecodeString(a)
	if err != nil {
		return "", "", ErrSesi
	}
	isi := string(raw)
	if !hmac.Equal([]byte(tanda), []byte(s.tanda(isi))) {
		return "", "", ErrSesi
	}
	f := strings.Split(isi, "|")
	if len(f) != 3 {
		return "", "", ErrSesi
	}
	exp, err := strconv.ParseInt(f[2], 10, 64)
	if err != nil || now.Unix() > exp {
		return "", "", ErrSesi
	}
	return f[0], f[1], nil
}

func (s *Sesi) tanda(isi string) string {
	m := hmac.New(sha256.New, s.kunci)
	m.Write([]byte(isi))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

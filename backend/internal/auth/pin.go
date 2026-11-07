// Package auth is who may see what: warga by alamat + PIN, pengurus by Clerk.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/argon2"
)

// PINPanjang is the warga PIN length. 6 digits = a million combinations; with
// the lock-out (lockout.go) guessing one house takes years, not days.
const PINPanjang = 6

var polaPIN = regexp.MustCompile(`^[0-9]{6}$`)

var ErrPINFormat = errors.New("PIN harus 6 angka")

// PINBawaan is a house's first PIN: the last 6 digits of its phone number.
func PINBawaan(telp string) (string, bool) {
	if len(telp) < PINPanjang {
		return "", false
	}
	return telp[len(telp)-PINPanjang:], true
}

func CekFormatPIN(pin string) error {
	if !polaPIN.MatchString(pin) {
		return ErrPINFormat
	}
	return nil
}

// argon2id parameters (RFC 9106 second recommendation, scaled for a small VM).
const (
	argonTime    = 2
	argonMemory  = 19 * 1024 // KiB
	argonThreads = 1
	argonKeyLen  = 32
)

// HashPIN returns a PHC-style argon2id string.
func HashPIN(pin string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pin), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	enc := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads, enc(salt), enc(key)), nil
}

// CocokPIN reports whether pin matches hash, in constant time.
func CocokPIN(hash, pin string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, t, m, p, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// HashPalsu is checked against when the alamat doesn't exist, so a wrong
// address costs the same time as a wrong PIN (no address enumeration by timing).
var HashPalsu = func() string { h, _ := HashPIN("000000"); return h }()

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
)

// Pengurus is a signed-in admin/bendahara/komite. Their rights come from the
// pengurus table (by email), never from anything the browser says.
type Pengurus struct {
	Email string `json:"email"`
	Nama  string `json:"nama"`
	Peran string `json:"peran"` // admin | bendahara | komite
}

// Boleh: admin may do everything; bendahara the money actions; komite only reads.
func (p *Pengurus) Boleh(peran ...string) bool {
	if p.Peran == "admin" {
		return true
	}
	for _, x := range peran {
		if p.Peran == x {
			return true
		}
	}
	return false
}

var (
	ErrBelumMasuk    = errors.New("belum masuk")
	ErrBukanPengurus = errors.New("akun ini bukan pengurus")
)

// CariPengurus maps a verified email (and Clerk user id) to a pengurus row.
type CariPengurus func(ctx context.Context, email, clerkID string) (*Pengurus, error)

// Penjaga identifies the pengurus behind a request.
type Penjaga interface {
	Pengurus(ctx context.Context, r *http.Request) (*Pengurus, error)
}

// ── Clerk ──────────────────────────────────────────────────────────────────

type Clerk struct {
	cari CariPengurus

	mu    sync.Mutex
	jwk   map[string]*clerk.JSONWebKey // by kid
	email map[string]emailCache        // by Clerk user id
}

type emailCache struct {
	email  string
	sampai time.Time
}

func NewClerk(secretKey string, cari CariPengurus) *Clerk {
	clerk.SetKey(secretKey)
	return &Clerk{cari: cari, jwk: map[string]*clerk.JSONWebKey{}, email: map[string]emailCache{}}
}

func (c *Clerk) Pengurus(ctx context.Context, r *http.Request) (*Pengurus, error) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return nil, ErrBelumMasuk
	}
	unverified, err := jwt.Decode(ctx, &jwt.DecodeParams{Token: token})
	if err != nil {
		return nil, ErrBelumMasuk
	}
	key, err := c.kunci(ctx, unverified.KeyID)
	if err != nil {
		return nil, err
	}
	claims, err := jwt.Verify(ctx, &jwt.VerifyParams{Token: token, JWK: key})
	if err != nil {
		return nil, ErrBelumMasuk
	}
	email, err := c.emailDari(ctx, claims.Subject)
	if err != nil {
		return nil, err
	}
	return c.cari(ctx, email, claims.Subject)
}

func (c *Clerk) kunci(ctx context.Context, kid string) (*clerk.JSONWebKey, error) {
	c.mu.Lock()
	k := c.jwk[kid]
	c.mu.Unlock()
	if k != nil {
		return k, nil
	}
	k, err := jwt.GetJSONWebKey(ctx, &jwt.GetJSONWebKeyParams{KeyID: kid})
	if err != nil {
		return nil, ErrBelumMasuk
	}
	c.mu.Lock()
	c.jwk[kid] = k
	c.mu.Unlock()
	return k, nil
}

// emailDari: the session token doesn't carry the email, so ask Clerk once per
// user per 10 minutes. Only a verified primary address counts.
func (c *Clerk) emailDari(ctx context.Context, userID string) (string, error) {
	c.mu.Lock()
	e, ok := c.email[userID]
	c.mu.Unlock()
	if ok && time.Now().Before(e.sampai) {
		return e.email, nil
	}
	u, err := user.Get(ctx, userID)
	if err != nil {
		return "", ErrBelumMasuk
	}
	var email string
	for _, a := range u.EmailAddresses {
		if u.PrimaryEmailAddressID != nil && a.ID == *u.PrimaryEmailAddressID &&
			a.Verification != nil && a.Verification.Status == "verified" {
			email = strings.ToLower(a.EmailAddress)
		}
	}
	if email == "" {
		return "", ErrBukanPengurus
	}
	c.mu.Lock()
	c.email[userID] = emailCache{email, time.Now().Add(10 * time.Minute)}
	c.mu.Unlock()
	return email, nil
}

// ── Development only ───────────────────────────────────────────────────────

// Dev trusts an X-Dev-Email header. main refuses to use it unless
// APP_ENV=development and no Clerk key is set — never in production.
type Dev struct{ Cari CariPengurus }

func (d Dev) Pengurus(ctx context.Context, r *http.Request) (*Pengurus, error) {
	email := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Dev-Email")))
	if email == "" {
		return nil, ErrBelumMasuk
	}
	return d.Cari(ctx, email, "")
}

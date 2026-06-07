// Package auth issues stub JWTs for the mock server's /auth/login
// endpoint so the CLI's API-key→JWT exchange flow exercises the same
// code path it does against the real server. The mock does NOT
// validate identity — Authenticate accepts any (username, password)
// pair, and Verify is provided for symmetry but isn't wired into any
// required middleware. The SDK is the right place to refuse to talk
// to the mock; see internal/mock/middleware.MockHeader.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultIssuer = "tavora-mock"
	defaultTTL    = 24 * time.Hour
)

// Manager signs and verifies tokens using a single HMAC secret. The
// mock generates the secret per-process by default — tokens don't
// outlive the binary, which is fine because the mock keeps no
// persistent state either.
type Manager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// Claims is the JWT payload the mock issues.
type Claims struct {
	Subject string   `json:"sub"`
	Roles   []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

// NewManager constructs a Manager. Pass an empty secret to use a
// random per-process key; pass an empty issuer to use "tavora-mock".
func NewManager(secret, issuer string, ttl time.Duration) *Manager {
	if issuer == "" {
		issuer = defaultIssuer
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if secret == "" {
		secret = randomSecret()
	}
	return &Manager{
		secret: []byte(secret),
		issuer: issuer,
		ttl:    ttl,
	}
}

// Issue signs a JWT for the given subject. The mock never rejects a
// login, so the subject is whatever the caller supplied on POST
// /auth/login.
func (m *Manager) Issue(subject string, roles []string) (string, time.Time, error) {
	exp := time.Now().Add(m.ttl)
	claims := Claims{
		Subject: subject,
		Roles:   roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// Verify parses + validates a bearer token. Not currently wired into a
// required middleware, but kept available so tests can assert that a
// token issued by the mock round-trips.
func (m *Manager) Verify(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// BearerFrom extracts the bearer token from an Authorization header
// value of the form "Bearer <tok>". Returns "" if the header is
// missing or malformed.
func BearerFrom(authorization string) string {
	if authorization == "" {
		return ""
	}
	parts := strings.SplitN(authorization, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func randomSecret() string {
	// 32 bytes of HS256 secret is plenty for the lifetime of a single
	// `tavora-mock` process. If crypto/rand fails (it shouldn't), fall
	// back to a fixed but obviously-mock string so the binary still
	// starts — the mock is dev-only and doesn't make security claims.
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "tavora-mock-fallback-secret"
	}
	return hex.EncodeToString(b[:])
}

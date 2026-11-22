package middleware

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwksKeyID = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

const httpsScheme = "https"

type jwksCache struct {
	mu                   sync.Mutex
	url                  string
	client               *http.Client
	now                  func() time.Time
	keys                 map[string]ed25519.PublicKey
	expires, timeAttempt time.Time
}

func NewJWKSAuthenticator(address, issuer, audience, legacySecret string, legacyUntil time.Time) (*JWTAuthenticator, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid JWKS URL")
	}
	loopback := parsed.Hostname() == "localhost"
	if ip := net.ParseIP(parsed.Hostname()); ip != nil {
		loopback = ip.IsLoopback()
	}
	if parsed.Scheme != httpsScheme && (parsed.Scheme != "http" || !loopback) {
		return nil, errors.New("JWKS requires HTTPS except loopback")
	}
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(audience) == "" || len(issuer) > 256 || len(audience) > 256 ||
		(legacySecret == "") != legacyUntil.IsZero() || legacyUntil.After(time.Now().Add(24*time.Hour)) {
		return nil, errors.New("invalid JWKS authentication configuration")
	}
	a := NewJWTAuthenticator(legacySecret, issuer)
	a.audience, a.legacyUntil = audience, legacyUntil
	a.jwks = &jwksCache{url: address, now: func() time.Time { return a.now() }, client: &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}

	return a, nil
}

func (c *jwksCache) key(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	if !jwksKeyID.MatchString(kid) {
		return nil, errors.New("invalid signing key id")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if key, ok := c.keys[kid]; ok && now.Before(c.expires) {
		return key, nil
	}
	if !c.timeAttempt.IsZero() && now.Sub(c.timeAttempt) < 5*time.Second {
		return nil, errors.New("JWKS refresh limited")
	}
	c.timeAttempt = now
	keys, err := c.fetch(ctx)
	if err != nil {
		return nil, err
	}
	c.keys, c.expires = keys, now.Add(time.Minute)
	key, ok := keys[kid]
	if !ok {
		return nil, errors.New("unknown signing key")
	}

	return key, nil
}

func (c *jwksCache) fetch(ctx context.Context) (map[string]ed25519.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("JWKS unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 32769))
	if err != nil || len(raw) > 32768 {
		return nil, errors.New("invalid JWKS response")
	}
	var doc struct {
		Keys []struct {
			KTY string `json:"kty"`
			CRV string `json:"crv"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			KID string `json:"kid"`
			X   string `json:"x"`
			D   string `json:"d"`
		} `json:"keys"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil || len(doc.Keys) == 0 || len(doc.Keys) > 8 {
		return nil, errors.New("invalid JWKS document")
	}
	keys := make(map[string]ed25519.PublicKey, len(doc.Keys))
	for _, key := range doc.Keys {
		public, decodeErr := base64.RawURLEncoding.DecodeString(key.X)
		_, duplicate := keys[key.KID]
		if duplicate || key.KTY != "OKP" || key.CRV != "Ed25519" || key.Use != "sig" || key.Alg != "EdDSA" || key.D != "" ||
			!jwksKeyID.MatchString(key.KID) || decodeErr != nil || len(public) != ed25519.PublicKeySize || base64.RawURLEncoding.EncodeToString(public) != key.X {
			return nil, errors.New("invalid JWKS public key")
		}
		keys[key.KID] = ed25519.PublicKey(public)
	}

	return keys, nil
}

func (a *JWTAuthenticator) verificationKey(ctx context.Context, token *jwt.Token) (any, error) {
	if token.Method == jwt.SigningMethodHS256 && (a.jwks == nil || (len(a.secret) > 0 && a.now().Before(a.legacyUntil))) {
		return a.secret, nil
	}
	if token.Method != jwt.SigningMethodEdDSA || a.jwks == nil {
		return nil, errors.New("unexpected signing method")
	}
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.New("missing signing key id")
	}

	return a.jwks.key(ctx, kid)
}

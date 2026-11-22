package middleware

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWKSRolloverAndCache(t *testing.T) {
	t.Parallel()
	key1, key2 := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, 32)), ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, 32))
	jwk := func(id string, key ed25519.PrivateKey) string {
		return fmt.Sprintf(`{"kty":"OKP","crv":"Ed25519","use":"sig","alg":"EdDSA","kid":%q,"x":%q}`, id, base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey)))
	}
	var body atomic.Value
	body.Store(`{"keys":[` + jwk("one", key1) + `]}`)
	var calls atomic.Int64
	var down atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if down.Load() {
			w.WriteHeader(503)

			return
		}
		_, _ = fmt.Fprint(w, body.Load().(string))
	}))
	defer server.Close()
	a, err := NewJWKSAuthenticator(server.URL, "users", "gateway", "", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	a.now = func() time.Time { return now }
	sign := func(id, audience string, key ed25519.PrivateKey) string {
		token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.RegisteredClaims{Subject: "user", Issuer: "users", Audience: jwt.ClaimStrings{audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute))})
		token.Header["kid"] = id
		encoded, signErr := token.SignedString(key)
		if signErr != nil {
			t.Fatal(signErr)
		}

		return "Bearer " + encoded
	}
	if _, err = a.Parse(sign("one", "gateway", key1)); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Parse(sign("one", "wrong", key1)); err == nil {
		t.Fatal("wrong audience accepted")
	}
	for range 5 {
		if _, err = a.Parse(sign("missing", "gateway", key2)); err == nil {
			t.Fatal("unknown kid accepted")
		}
	}
	if calls.Load() != 1 {
		t.Fatal("unknown kid caused fetch fanout", calls.Load())
	}
	body.Store(`{"keys":[` + jwk("one", key1) + `,` + jwk("two", key2) + `]}`)
	now = now.Add(6 * time.Second)
	if _, err = a.Parse(sign("two", "gateway", key2)); err != nil {
		t.Fatal("rollover", err)
	}
	if _, err = a.Parse(sign("one", "gateway", key1)); err != nil {
		t.Fatal("overlap", err)
	}
	down.Store(true)
	now = now.Add(61 * time.Second)
	if _, err = a.Parse(sign("one", "gateway", key1)); err == nil {
		t.Fatal("stale cache accepted")
	}
}

func TestJWKSRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()
	public := base64.RawURLEncoding.EncodeToString(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, 32)).Public().(ed25519.PublicKey))
	key := fmt.Sprintf(`{"kty":"OKP","crv":"Ed25519","use":"sig","alg":"EdDSA","kid":"one","x":%q}`, public)
	for _, body := range []string{`{"keys":[]}`, `{"keys":[` + key + `,` + key + `]}`, strings.Replace(`{"keys":[`+key+`]}`, `"EdDSA"`, `"HS256"`, 1), strings.Replace(`{"keys":[`+key+`]}`, `"kty":`, `"d":"private","kty":`, 1), strings.Repeat("x", 32769)} {
		t.Run(fmt.Sprint(len(body)), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, body) }))
			defer server.Close()
			a, err := NewJWKSAuthenticator(server.URL, "users", "gateway", "", time.Time{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = a.jwks.key(t.Context(), "one"); err == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
}

func TestJWKSLegacyDeadlineAndRedirect(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/redirect", http.StatusFound)
	}))
	defer server.Close()
	now := time.Now()
	a, err := NewJWKSAuthenticator(server.URL, "users", "gateway", "legacy", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.jwks.key(t.Context(), "one"); err == nil || calls.Load() != 1 {
		t.Fatal("redirect followed", calls.Load(), err)
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "user", Issuer: "users", ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}).SignedString([]byte("legacy"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Parse("Bearer " + token); err != nil {
		t.Fatal(err)
	}
	a.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err = a.Parse("Bearer " + token); err == nil {
		t.Fatal("expired legacy accepted")
	}
}

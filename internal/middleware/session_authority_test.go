package middleware

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type sessionAuthorityFake struct {
	calls int
	roles []string
	err   error
}

func (f *sessionAuthorityFake) CheckSession(context.Context, string) (SessionState, error) {
	f.calls++

	return SessionState{UserID: "user", SessionID: "session", Roles: f.roles}, f.err
}
func (f *sessionAuthorityFake) RevokeSession(context.Context, string) error { return f.err }

func TestSessionAuthorityFreshAndBounded(t *testing.T) {
	t.Parallel()

	now := time.Now()
	a := NewJWTAuthenticator("secret", "users", []string{"user"})
	a.now = func() time.Time { return now }
	f := &sessionAuthorityFake{roles: []string{"user"}}
	a.SetSessionVerifier(f)
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{Roles: []string{"admin"}, RegisteredClaims: jwt.RegisteredClaims{Subject: "user", ID: "session", Issuer: "users", ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}}).SignedString([]byte("secret"))
	info, err := a.ParseContext(context.Background(), "Bearer "+token)
	if err != nil || len(info.Roles) != 1 || info.Roles[0] != "user" {
		t.Fatalf("stale roles: %+v %v", info, err)
	}
	_, _ = a.ParseContext(context.Background(), "Bearer "+token)
	if f.calls != 1 {
		t.Fatal(f.calls)
	}
	f.err = errors.New("offline")
	ctx := RefreshAuth(ContextWithAuth(context.Background(), info))
	if _, authErr := RequireAuth(ctx); authErr == nil {
		t.Fatal("mutation outage allowed")
	}
	now = now.Add(6 * time.Second)
	if _, err = a.ParseContext(context.Background(), "Bearer "+token); err == nil {
		t.Fatal("expired cache accepted")
	}
}

func TestSessionCacheBoundAndLogout(t *testing.T) {
	t.Parallel()

	a := NewJWTAuthenticator("secret", "users")
	f := &sessionAuthorityFake{roles: []string{"user"}}
	a.SetSessionVerifier(f)
	info := AuthInfo{UserID: "user", sessionID: "session", authority: a}
	for i := 0; i < 2048; i++ {
		info.Token = fmt.Sprintf("token-%d", i)
		if _, err := a.checkSession(context.Background(), info, false); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.cache.values) > 1024 {
		t.Fatal(len(a.cache.values))
	}
	ctx := ContextWithAuth(context.Background(), info)
	if err := RevokeAuth(ctx); err != nil {
		t.Fatal(err)
	}
	before := f.calls
	_, _ = a.checkSession(context.Background(), info, false)
	if f.calls != before+1 {
		t.Fatal("logout cache retained")
	}
	f.err = errors.New("revoked")
	if _, err := RequireAdmin(ctx); err == nil {
		t.Fatal("admin stale authorization")
	}
}

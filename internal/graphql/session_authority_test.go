package graphql

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type authorityStub struct{ revoked bool }

func (s *authorityStub) CheckSession(context.Context, string) (middleware.SessionState, error) {
	if s.revoked {
		return middleware.SessionState{}, errors.New("revoked")
	}
	return middleware.SessionState{UserID: "user-id", SessionID: "session", Roles: []string{"user"}}, nil
}
func (s *authorityStub) RevokeSession(context.Context, string) error { s.revoked = true; return nil }

func TestLogoutDeniesNextMutationInSameOperation(t *testing.T) {
	svc := &userServiceStub{}
	authority := &authorityStub{}
	a := middleware.NewJWTAuthenticator("secret", "users")
	a.SetSessionVerifier(authority)
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{Subject: "user-id", ID: "session", Issuer: "users", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}).SignedString([]byte("secret"))
	handler := middleware.JWT(a, middleware.SessionCookie(middleware.SessionConfig{}, Handler(svc, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))))
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"mutation { logout updateUser(id: \"user-id\", input: {firstName: \"After logout\"}) { id } }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if !authority.revoked || svc.updateID != "" || !strings.Contains(w.Body.String(), "UNAUTHENTICATED") {
		t.Fatalf("protected sibling executed: update=%s response=%s", svc.updateID, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("cookie not cleared: %s", w.Header().Get("Set-Cookie"))
	}
}

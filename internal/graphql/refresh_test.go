package graphql

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type refreshUserStub struct {
	userServiceStub
	rotated int
	revoked bool
	err     error
}

func (s *refreshUserStub) RefreshSession(_ context.Context, token string) (*users.AuthPayload, error) {
	s.rotated++

	return &users.AuthPayload{User: upstreamUser(), Token: "access-new", RefreshToken: "refresh-new", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339), RefreshExpiresAt: time.Now().Add(24 * time.Hour).Format(time.RFC3339)}, s.err
}
func (s *refreshUserStub) RevokeRefreshSession(context.Context, string) error {
	s.revoked = true

	return s.err
}

func TestBrowserRefreshAndExpiredLogout(t *testing.T) {
	t.Parallel()
	svc := &refreshUserStub{}
	handler := middleware.SessionCookie(middleware.SessionConfig{Secure: true}, Handler(svc, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))))
	send := func(query string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"`+query+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: "refresh-old"})
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		return recorder
	}
	result := send("mutation { refreshSession }")
	if svc.rotated != 1 || strings.Contains(result.Body.String(), "errors") {
		t.Fatalf("refresh failed: %s", result.Body.String())
	}
	if strings.Contains(result.Body.String(), "refresh-new") || strings.Contains(result.Body.String(), "access-new") {
		t.Fatal("credential leaked to JS")
	}
	if len(result.Result().Cookies()) != 2 {
		t.Fatal("missing rotated cookies")
	}
	result = send("mutation { logout }")
	if !svc.revoked {
		t.Fatal("logout without access did not revoke family")
	}
	for _, cookie := range result.Result().Cookies() {
		if cookie.MaxAge != -1 {
			t.Fatal("cookie not cleared")
		}
	}
}

func TestLogoutKeepsCookieWhenRevocationFails(t *testing.T) {
	t.Parallel()
	svc := &refreshUserStub{err: errors.New("users down")}
	handler := middleware.SessionCookie(middleware.SessionConfig{}, Handler(svc, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))))
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"mutation{logout}"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: "retryable"})
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, req)
	if len(result.Result().Cookies()) != 0 || !strings.Contains(result.Body.String(), "errors") {
		t.Fatalf("revocation credential lost: %s", result.Body.String())
	}
}

func TestFreshBrowserSessionDoesNotRotate(t *testing.T) {
	t.Parallel()
	svc := &refreshUserStub{}
	resolver := &mutationResolver{&Resolver{Users: svc}}
	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: testUserID, Token: "access", ExpiresAt: time.Now().Add(time.Hour)})
	expiry, err := resolver.RefreshSession(ctx)
	if err != nil || expiry == nil || svc.rotated != 0 {
		t.Fatalf("unnecessary rotation: %d %v", svc.rotated, err)
	}
}

func TestRejectedRefreshClearsCookiesButOutageDoesNot(t *testing.T) {
	t.Parallel()
	for _, rejected := range []bool{true, false} {
		t.Run(fmt.Sprint(rejected), func(t *testing.T) {
			t.Parallel()
			upstream := errors.New("users down")
			if rejected {
				upstream = &users.Error{Code: "UNAUTHENTICATED", Message: "revoked"}
			}
			svc := &refreshUserStub{err: upstream}
			handler := middleware.SessionCookie(middleware.SessionConfig{}, Handler(svc, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))))
			req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"mutation{refreshSession}"}`))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: "old"})
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, req)
			want := 0
			if rejected {
				want = 2
			}
			if len(result.Result().Cookies()) != want {
				t.Fatal("incorrect failure cookie handling")
			}
		})
	}
}

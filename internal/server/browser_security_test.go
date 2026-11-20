package server

import (
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/overmindv/api-gateway/internal/config"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func TestPublicAndLocalDebugRoutes(t *testing.T) {
	t.Parallel()

	for _, production := range []bool{false, true} {
		cfg := config.HTTP{Production: production, CORSOrigins: []string{"https://learn.example"}}
		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		srv := New(cfg, nil, nil, nil, nil, nil, middleware.NewJWTAuthenticator("test", "users"), true, log, log)
		req := httptest.NewRequest("GET", "/playground", nil)
		result := httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(result, req)
		want := 200
		if production {
			want = 404
		}
		if result.Code != want {
			t.Fatalf("playground production=%v: %d", production, result.Code)
		}
		req = httptest.NewRequest("POST", "/graphql", strings.NewReader(`{"query":"{ __schema { queryType { name } } }"}`))
		req.TLS = &tls.ConnectionState{}
		req.Header.Set("Content-Type", "application/json")
		result = httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(result, req)
		hasSchema := strings.Contains(result.Body.String(), `"queryType":{"name":"Query"}`)
		if hasSchema == production {
			t.Fatalf("introspection production=%v: %s", production, result.Body.String())
		}
		if production && !strings.Contains(result.Body.String(), `"errors"`) {
			t.Fatal("missing neutral error")
		}
	}
}

//nolint:paralleltest // t.Setenv изменяет process-wide environment
func TestServerRejectsCookieCSRFBeforeMutation(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(config.HTTP{CORSOrigins: []string{"https://learn.example"}}, nil, nil, nil, nil, nil, middleware.NewJWTAuthenticator("test", "users"), true, log, log)
	for _, origin := range []string{"https://attacker.example", ""} {
		req := httptest.NewRequest("POST", "/graphql", strings.NewReader(`{"query":"mutation { logout }"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: "token"})
		result := httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(result, req)
		if result.Code != 403 || len(result.Result().Cookies()) != 0 {
			t.Fatalf("CSRF executed: %d", result.Code)
		}
	}
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(`{"query":"mutation { logout }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://learn.example")
	result := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(result, req)
	if result.Code != 200 || len(result.Result().Cookies()) != 1 {
		t.Fatalf("same origin failed: %d %s", result.Code, result.Body.String())
	}
}

func TestLocalPlaygroundOriginCanExecuteQuery(t *testing.T) {
	t.Setenv("TASK_HUNTER_TOKEN", "test")
	t.Setenv("MEDIA_TOKEN", "test")
	t.Setenv("FEED_TOKEN", "test")
	t.Setenv("APP_ENV", "development")
	t.Setenv("CORS_ORIGINS", "http://localhost:3000,http://localhost:5173")
	t.Setenv("PORT", "8081")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := New(cfg.HTTP, nil, nil, nil, nil, nil, middleware.NewJWTAuthenticator("test", "users"), cfg.Session.Secure, log, log)
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(`{"query":"{ __schema { queryType { name } } }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:8081")
	w := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"queryType":{"name":"Query"}`) {
		t.Fatalf("playground: %d %s", w.Code, w.Body.String())
	}
}

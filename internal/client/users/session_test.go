package users

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/config"
)

func TestSessionContractAndRedirect(t *testing.T) {
	var authorization, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		path = r.URL.Path
		if path == "/internal/session/revoke" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"user_id":"u","session_id":"s","roles":["user"],"expires_at":"2030-01-01T00:00:00Z"}`)
	}))
	defer srv.Close()
	c := New(config.Users{GraphQLURL: srv.URL + "/query", Timeout: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := c.CheckSession(context.Background(), "token")
	if err != nil || got.UserID != "u" || path != "/internal/session/check" || authorization != "Bearer token" {
		t.Fatalf("contract: %+v %v", got, err)
	}
	if err = c.RevokeSession(context.Background(), "token"); err != nil || path != "/internal/session/revoke" {
		t.Fatalf("revoke: %v", err)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/query", http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	c.url = redirect.URL + "/query"
	path = ""
	if _, err = c.CheckSession(context.Background(), "token"); err == nil || path != "" {
		t.Fatal("redirect followed")
	}
}

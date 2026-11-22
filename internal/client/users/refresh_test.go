package users

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/config"
)

func TestRefreshUsesCredentialWithoutAccess(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("access forwarded")
		}
		var req struct {
			Query     string            `json:"query"`
			Variables map[string]string `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if req.Variables["refreshToken"] != "opaque" {
			t.Error("missing credential")
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(req.Query, "mutation RefreshSession(") {
			_, _ = io.WriteString(w, `{"data":{"refreshSession":{"token":"new-access","refreshToken":"new-refresh","expiresAt":"2030-01-01T00:00:00Z","refreshExpiresAt":"2030-02-01T00:00:00Z","user":{"id":"user"}}}}`)
		} else {
			_, _ = io.WriteString(w, `{"data":{"revokeRefreshSession":true}}`)
		}
	}))
	defer server.Close()
	client := New(config.Users{GraphQLURL: server.URL, Timeout: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	result, err := client.RefreshSession(context.Background(), "opaque")
	if err != nil || result.RefreshToken != "new-refresh" {
		t.Fatalf("refresh: %v", err)
	}
	if err = client.RevokeRefreshSession(context.Background(), "opaque"); err != nil {
		t.Fatal(err)
	}
}

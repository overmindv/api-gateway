package users

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/config"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func TestRecoveryUsesSignedClientIPWithoutAccess(t *testing.T) {
	t.Parallel()

	seen := map[string]map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("recovery forwarded an access token")
		}
		if r.Header.Get("X-Login-Client-IP") != testClientIP ||
			r.Header.Get("X-Login-Client-Time") == "" ||
			r.Header.Get("X-Login-Client-Signature") == "" {
			t.Error("recovery request is missing signed client IP")
		}
		var req struct {
			Query     string            `json:"query"`
			Variables map[string]string `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		seen[req.Variables["token"]+req.Variables["email"]] = req.Variables
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"requestAccountRecovery":true,"resetAccountPassword":true,"verifyAccountEmail":true}}`)
	}))
	defer server.Close()

	client := New(config.Users{GraphQLURL: server.URL, Timeout: time.Second, LoginContextSecret: "shared-secret"},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := middleware.ClientIPMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if ok, err := client.RequestAccountRecovery(r.Context(), "user@example.com", RecoveryPurposeReset); err != nil || !ok {
			t.Errorf("recovery request: %v %v", ok, err)
		}
		if ok, err := client.ResetAccountPassword(r.Context(), "reset-token", "new-password"); err != nil || !ok {
			t.Errorf("reset: %v %v", ok, err)
		}
		if ok, err := client.VerifyAccountEmail(r.Context(), "verify-token"); err != nil || !ok {
			t.Errorf("verify: %v %v", ok, err)
		}
	}))
	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	req.RemoteAddr = testClientAddr
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if seen["user@example.com"]["purpose"] != RecoveryPurposeReset {
		t.Errorf("purpose = %q", seen["user@example.com"]["purpose"])
	}
	if seen["reset-token"]["password"] != "new-password" {
		t.Error("reset credentials not forwarded")
	}
	if _, ok := seen["verify-token"]; !ok {
		t.Error("verify request not sent")
	}
}

func TestRecoveryKeepsUpstreamRateLimitCode(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"errors":[{"message":"too many","extensions":{"code":"`+codeRateLimited+`"}}]}`)
	}))
	defer server.Close()

	client := New(config.Users{GraphQLURL: server.URL, Timeout: time.Second},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := client.RequestAccountRecovery(context.Background(), "user@example.com", RecoveryPurposeReset)
	var upstream *Error
	if !errors.As(err, &upstream) || upstream.Code != codeRateLimited {
		t.Fatalf("error = %v", err)
	}
}

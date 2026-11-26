package users

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/config"
	"github.com/overmindv/api-gateway/internal/middleware"
)

const privacyTestID = "2f1d4a5e-3f6b-4d5a-8f2e-1c7d9a0b3e4f"

func TestPrivacyLifecycleKeepsSecretsOutOfURL(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "receipt-value") {
			t.Error("receipt leaked to URL")
		}
		if r.Header.Get("X-Login-Client-IP") == "" || r.Header.Get("X-Login-Client-Signature") == "" {
			t.Error("missing signed client IP")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/privacy/requests":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Error("missing access token")
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["kind"] != "export" || body["password"] != "current-password" {
				t.Errorf("body = %v", body)
			}
			_, _ = io.WriteString(w, `{"request":{"id":"`+privacyTestID+`","kind":"export","state":"awaiting_confirmation","created_at":"2026-11-26T10:00:00Z","expires_at":"2026-11-26T10:15:00Z","owners":[]},"receipt":"receipt-value"}`)
		case r.URL.Path == "/internal/privacy/requests/"+privacyTestID+"/confirm":
			if r.Header.Get("X-Privacy-Receipt") != "receipt-value" {
				t.Error("missing receipt")
			}
			if r.Header.Get("Authorization") != "" {
				t.Error("confirm requires no access token")
			}
			_, _ = io.WriteString(w, `{"id":"`+privacyTestID+`","kind":"export","state":"processing","created_at":"2026-11-26T10:00:00Z","expires_at":"2026-11-26T10:15:00Z","owners":[{"owner":"tasks","state":"pending"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/internal/privacy/requests/"+privacyTestID:
			_, _ = io.WriteString(w, `{"id":"`+privacyTestID+`","kind":"delete","state":"processing","publication":"pending","created_at":"2026-11-26T10:00:00Z","expires_at":"2026-11-26T10:15:00Z","owners":[{"owner":"tasks","state":"completed"}]}`)
		case r.URL.Path == "/internal/privacy/requests/"+privacyTestID+"/export":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Error("export requires access token")
			}
			_, _ = io.WriteString(w, `{"profile":{"id":"`+privacyTestID+`","email":"user@example.com","username":"user","first_name":"Ivan","last_name":"Petrov","birth_date":null,"phone":null,"avatar_file_id":null,"timezone":"UTC","locale":"ru-RU","public_name":false,"leaderboard_visible":false,"created_at":"2026-01-01T00:00:00Z"},"services":{"tasks":[{"user_id":"`+privacyTestID+`","watermark":"w","next_cursor":"","records":[{"user_id":"`+privacyTestID+`","id":"`+privacyTestID+`","kind":"submission","occurred_at":"2026-01-02T00:00:00Z","title":"Task"}]}]}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := New(config.Users{GraphQLURL: server.URL, Timeout: time.Second, LoginContextSecret: "shared-secret"},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := middleware.ClientIPMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requestCtx := middleware.ContextWithAuth(r.Context(), middleware.AuthInfo{UserID: "user", Token: "access-token"})
		start, err := client.PreparePrivacyRequest(requestCtx, "export", "current-password")
		if err != nil || start.Receipt != "receipt-value" {
			t.Errorf("prepare: %v %v", start, err)
		}
		confirmed, err := client.ConfirmPrivacyRequest(requestCtx, privacyTestID, "receipt-value")
		if err != nil || confirmed.State != "processing" || len(confirmed.Owners) != 1 {
			t.Errorf("confirm: %v %v", confirmed, err)
		}
		status, err := client.PrivacyRequestStatus(requestCtx, privacyTestID, "receipt-value")
		if err != nil || status.Publication != "pending" {
			t.Errorf("status: %v %v", status, err)
		}
		export, err := client.AccountExport(requestCtx, privacyTestID)
		if err != nil || export.Profile.Email != "user@example.com" || len(export.Services["tasks"]) != 1 {
			t.Errorf("export: %v %v", export, err)
		}
	}))
	request := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	request.RemoteAddr = "192.0.2.42:1111"
	handler.ServeHTTP(httptest.NewRecorder(), request)
}

func TestPrivacyErrorsAreSafeAndTyped(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		status int
		code   string
	}{
		{http.StatusUnauthorized, "UNAUTHENTICATED"},
		{http.StatusForbidden, "PERMISSION_DENIED"},
		{http.StatusTooManyRequests, "RATE_LIMITED"},
		{http.StatusConflict, codePrivacyRequestInvalid},
		{http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, `{"error":"internal detail"}`)
			}))
			defer server.Close()
			client := New(config.Users{GraphQLURL: server.URL, Timeout: time.Second},
				slog.New(slog.NewTextHandler(io.Discard, nil)))
			_, err := client.PrivacyRequestStatus(context.Background(), privacyTestID, "receipt-value")
			var upstream *Error
			if !errors.As(err, &upstream) || upstream.Code != tc.code {
				t.Fatalf("error = %v", err)
			}
			if strings.Contains(err.Error(), "internal detail") {
				t.Fatal("upstream body leaked")
			}
		})
	}
}

func TestPrivacyRejectsInvalidRequestID(t *testing.T) {
	t.Parallel()

	client := New(config.Users{GraphQLURL: "http://127.0.0.1:1", Timeout: time.Second},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := client.ConfirmPrivacyRequest(context.Background(), "../etc/passwd", "receipt")
	var upstream *Error
	if !errors.As(err, &upstream) || upstream.Code != codePrivacyRequestInvalid {
		t.Fatalf("error = %v", err)
	}
}

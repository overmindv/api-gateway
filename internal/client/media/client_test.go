package media

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestClientForwardsTrustedHeaders проверяет service token и actor context запроса Media.
func TestClientForwardsTrustedHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Media-Service-Token") != "service-token" {
			t.Fatal("service token не передан")
		}
		if r.Header.Get("X-User-ID") != "user-id" || r.Header.Get("X-User-Roles") != "admin" {
			t.Fatalf("неожиданный actor context: %q %q", r.Header.Get("X-User-ID"), r.Header.Get("X-User-Roles"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"file_id":"file-id","mode":"single","url":"https://storage","fields":{},"headers":{},"expires_at":"now"}`))
	}))
	defer server.Close()
	client := New(server.URL, "service-token", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	result, err := client.CreateUpload(context.Background(), CreateUploadInput{}, Actor{UserID: "user-id", Roles: []string{"admin"}})
	if err != nil {
		t.Fatalf("вызвать Media: %v", err)
	}
	if result.FileID != "file-id" {
		t.Fatalf("неожиданный response: %+v", result)
	}
}

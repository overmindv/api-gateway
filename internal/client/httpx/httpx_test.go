package httpx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/overmindv/api-gateway/internal/middleware"
)

// TestClampLimit проверяет нормализацию limit пагинации.
func TestClampLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "negative", input: -5, want: DefaultLimit},
		{name: "zero", input: 0, want: DefaultLimit},
		{name: "default", input: DefaultLimit, want: DefaultLimit},
		{name: "lower bound", input: MinLimit, want: MinLimit},
		{name: "typical", input: 15, want: 15},
		{name: "upper bound", input: MaxLimit, want: MaxLimit},
		{name: "over limit", input: MaxLimit + 1, want: MaxLimit},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := ClampLimit(test.input); got != test.want {
				t.Fatalf("ClampLimit(%d) = %d, want %d", test.input, got, test.want)
			}
		})
	}
}

func TestDoJSONWithHeadersForwardsConcurrencyHeadersWithoutOverridingAuthority(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Idempotency-Key"); got != "request-key" {
			t.Errorf("Idempotency-Key = %q, want request-key", got)
		}
		if got := r.Header.Get("X-Expected-Version"); got != "7" {
			t.Errorf("X-Expected-Version = %q, want 7", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer service-token" {
			t.Errorf("Authorization = %q, want service credential", got)
		}
		if got := r.Header.Get("X-User-ID"); got != "jwt-user" {
			t.Errorf("X-User-ID = %q, want JWT actor", got)
		}
		if got := r.Header.Get(middleware.RequestIDHeader); got != "trusted-request" {
			t.Errorf("X-Request-ID = %q, want middleware request ID", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()

	client := New("test", upstream.URL, upstream.Client(), slog.New(slog.NewTextHandler(io.Discard, nil)), func(int) error { return nil }, Header{Name: "Authorization", Value: "Bearer service-token"})
	ctx := middleware.WithRequestID(context.Background(), "trusted-request")
	_, err := DoJSONWithHeaders[struct{}](client, ctx, http.MethodPut, "/write", map[string]string{"value": "ok"}, Actor{UserID: "jwt-user"},
		Header{Name: "Idempotency-Key", Value: "request-key"},
		Header{Name: "X-Expected-Version", Value: "7"},
		Header{Name: "Authorization", Value: "Bearer attacker"},
		Header{Name: "X-User-ID", Value: "other-user"},
		Header{Name: middleware.RequestIDHeader, Value: "untrusted-request"},
	)
	if err != nil {
		t.Fatal(err)
	}
}

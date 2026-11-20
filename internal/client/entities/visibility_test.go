package entities

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

//nolint:paralleltest // subtests используют один httptest.Server и общий канал запросов
func TestCatalogCredentialsFollowVerifiedRole(t *testing.T) {
	t.Parallel()

	gatewayToken := strings.Repeat("a", 32)
	requests := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Header.Clone()
		_, _ = io.WriteString(w, "[]")
	}))
	defer upstream.Close()
	client := NewWithGatewayToken(upstream.URL, gatewayToken, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	const actorID = "11111111-1111-1111-1111-111111111111"
	for _, role := range []string{"", "student", "admin", "superuser"} {
		t.Run(role+" reader", func(t *testing.T) {
			ctx := t.Context()
			if role != "" {
				ctx = middleware.ContextWithAuth(ctx, middleware.AuthInfo{
					UserID: actorID,
					Token:  "session-fixture",
					Roles:  []string{role},
				})
			}
			if _, err := client.ListCourses(ctx, "", ListOptions{}); err != nil {
				t.Fatal(err)
			}
			headers := <-requests
			if role == "admin" || role == "superuser" {
				if headers.Get("Authorization") != "Bearer "+gatewayToken ||
					headers.Get("X-User-ID") != actorID || headers.Get("X-User-Roles") != role {
					t.Error("missing gateway authorization or admin actor")
				}
			} else if headers.Get("Authorization") != "" || headers.Get("X-User-ID") != "" ||
				headers.Get("X-User-Roles") != "" {
				t.Error("public catalog request received privileged headers")
			}
		})
	}
}

func TestCatalogClientDoesNotForwardCredentialsOnRedirect(t *testing.T) {
	t.Parallel()

	requests := make(chan struct{}, 1)
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests <- struct{}{}
		_, _ = io.WriteString(w, "[]")
	}))
	defer destination.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer upstream.Close()
	client := NewWithGatewayToken(upstream.URL, strings.Repeat("a", 32), time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := middleware.ContextWithAuth(t.Context(), middleware.AuthInfo{
		UserID: "11111111-1111-1111-1111-111111111111",
		Token:  "session-fixture",
		Roles:  []string{"admin"},
	})
	if _, err := client.ListCourses(ctx, "", ListOptions{}); err == nil {
		t.Error("catalog redirect accepted")
	}
	select {
	case <-requests:
		t.Error("catalog redirect was followed")
	default:
	}
}

package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/overmindv/api-gateway/internal/apperror"
)

const (
	testUserID    = "user-id"
	testRequestID = "client-request-id"
)

func TestJWTAuthenticator(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	authenticator := NewJWTAuthenticator("secret", "users")
	authenticator.now = func() time.Time { return now }

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   testUserID,
		Issuer:    "users",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	})

	value, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	info, err := authenticator.Parse("Bearer " + value)
	if err != nil || info.UserID != testUserID || info.Token != value {
		t.Fatalf("Parse() = %+v, %v", info, err)
	}

	authenticator.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := authenticator.Parse("Bearer " + value); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("expected expired token rejection, got %v", err)
	}

	if _, err := authenticator.Parse("invalid"); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("expected malformed token rejection, got %v", err)
	}
}

func TestRequireAdminAllowsAdminAndSuperuser(t *testing.T) {
	t.Parallel()

	for _, roles := range [][]string{{"admin"}, {"superuser"}} {
		ctx := ContextWithAuth(t.Context(), AuthInfo{
			UserID: testUserID,
			Token:  "token",
			Roles:  roles,
		})

		if _, err := RequireAdmin(ctx); err != nil {
			t.Fatalf("RequireAdmin(%v) = %v", roles, err)
		}
	}

	ctx := ContextWithAuth(t.Context(), AuthInfo{
		UserID: testUserID,
		Token:  "token",
		Roles:  []string{"student"},
	})
	if _, err := RequireAdmin(ctx); !errors.Is(err, apperror.ErrPermissionDenied) {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestJWTMiddlewareStoresAuthenticationError(t *testing.T) {
	t.Parallel()

	authenticator := NewJWTAuthenticator("secret", "users")

	handler := JWT(authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := RequireAuth(r.Context()); !errors.Is(err, apperror.ErrUnauthenticated) {
			t.Fatalf("expected unauthenticated context, got %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Parallel()

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) != testRequestID {
			t.Fatalf("request id not propagated")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set(RequestIDHeader, testRequestID)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Header().Get(RequestIDHeader) != testRequestID {
		t.Fatalf("response request id missing")
	}
}

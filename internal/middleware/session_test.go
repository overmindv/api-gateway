package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestSessionCookieSetsHttpOnlyCookie проверяет выдачу httpOnly session cookie.
func TestSessionCookieSetsHttpOnlyCookie(t *testing.T) {
	handler := SessionCookie(SessionConfig{Secure: true}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetSessionToken(r.Context(), "jwt-value", time.Hour)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	request := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	setCookies := response.Result().Cookies()
	if len(setCookies) != 1 {
		t.Fatalf("expected 1 Set-Cookie, got %d", len(setCookies))
	}
	cookie := setCookies[0]
	if cookie.Name != SessionCookieName || cookie.Value != "jwt-value" {
		t.Fatalf("unexpected cookie: %+v", cookie)
	}
	if !cookie.HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
	if !cookie.Secure {
		t.Fatal("session cookie must be Secure")
	}
	if cookie.MaxAge != int(time.Hour.Seconds()) {
		t.Fatalf("MaxAge = %d, want %d", cookie.MaxAge, int(time.Hour.Seconds()))
	}
}

// TestSessionCookieClearsExistingCookie проверяет удаление cookie при ClearSessionToken.
func TestSessionCookieClearsExistingCookie(t *testing.T) {
	handler := SessionCookie(SessionConfig{Secure: true}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ClearSessionToken(r.Context())
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	request := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "previous"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	cookie := response.Result().Cookies()[0]
	if cookie.Value != "" {
		t.Fatalf("expected cleared cookie, got %q", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("expected negative MaxAge to delete cookie, got %d", cookie.MaxAge)
	}
}

// TestJWTMiddlewareReadsTokenFromCookie проверяет, что JWT middleware принимает сессию из cookie.
func TestJWTMiddlewareReadsTokenFromCookie(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	authenticator := NewJWTAuthenticator("secret", "users")
	authenticator.now = func() time.Time { return now }

	value := signTestToken(t, "secret", "users", "user-id", now, now.Add(time.Hour))

	var gotInfo AuthInfo
	handler := JWT(authenticator, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := RequireAuth(r.Context())
		if err != nil {
			t.Fatalf("RequireAuth: %v", err)
		}
		gotInfo = info
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: value})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if gotInfo.UserID != "user-id" || gotInfo.Token != value {
		t.Fatalf("AuthInfo from cookie = %+v", gotInfo)
	}
	if response.Body.Len() != 0 {
		t.Fatal("unexpected body")
	}
}

// signTestToken подписывает тестовый JWT.
func signTestToken(t *testing.T, secret, issuer, subject string, issuedAt, expiresAt time.Time) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   subject,
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})

	value, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	return value
}

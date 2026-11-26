package graphql

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type recoveryStub struct {
	userServiceStub
	email, purpose        string
	resetToken, resetPass string
	verifyToken           string
	err                   error
}

func (s *recoveryStub) RequestAccountRecovery(_ context.Context, email, purpose string) (bool, error) {
	s.email, s.purpose = email, purpose

	return s.err == nil, s.err
}

func (s *recoveryStub) ResetAccountPassword(_ context.Context, token, password string) (bool, error) {
	s.resetToken, s.resetPass = token, password

	return s.err == nil, s.err
}

func (s *recoveryStub) VerifyAccountEmail(_ context.Context, token string) (bool, error) {
	s.verifyToken = token

	return s.err == nil, s.err
}

func TestAccountRecoveryDoesNotRequireAuthAndKeepsPurpose(t *testing.T) {
	t.Parallel()

	svc := &recoveryStub{}
	resolver := &mutationResolver{&Resolver{Users: svc}}

	purpose := model.AccountRecoveryPurposeVerify
	ok, err := resolver.RequestAccountRecovery(context.Background(), "user@example.com", &purpose)
	if err != nil || !ok {
		t.Fatalf("recovery: %v %v", ok, err)
	}
	if svc.email != "user@example.com" || svc.purpose != users.RecoveryPurposeVerify {
		t.Fatalf("recovery args: %q %q", svc.email, svc.purpose)
	}

	ok, err = resolver.RequestAccountRecovery(context.Background(), "user@example.com", nil)
	if err != nil || !ok || svc.purpose != users.RecoveryPurposeReset {
		t.Fatalf("default purpose: %q %v", svc.purpose, err)
	}

	if ok, err = resolver.VerifyAccountEmail(context.Background(), "verify-token"); err != nil || !ok || svc.verifyToken != "verify-token" {
		t.Fatalf("verify: %v %v", ok, err)
	}
}

func TestResetAccountPasswordClearsBrowserCookies(t *testing.T) {
	t.Parallel()

	svc := &recoveryStub{}
	handler := middleware.SessionCookie(middleware.SessionConfig{Secure: true},
		Handler(svc, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))))
	request := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"mutation { resetAccountPassword(token: \"reset-token\", password: \"new-password\") }"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: "access"})
	request.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: "refresh"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if strings.Contains(recorder.Body.String(), "errors") {
		t.Fatalf("reset failed: %s", recorder.Body.String())
	}
	if svc.resetToken != "reset-token" || svc.resetPass != "new-password" {
		t.Fatal("reset credentials not forwarded")
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %d", len(cookies))
	}
	for _, cookie := range cookies {
		if cookie.MaxAge != -1 {
			t.Fatalf("cookie %s not cleared", cookie.Name)
		}
	}
}

func TestRecoveryRateLimitIsPresentedWithoutSecrets(t *testing.T) {
	t.Parallel()

	svc := &recoveryStub{err: &users.Error{Code: "RATE_LIMITED", Message: "upstream detail"}}
	handler := Handler(svc, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"mutation { resetAccountPassword(token: \"reset-token\", password: \"new-password\") }"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	body := recorder.Body.String()
	if !strings.Contains(body, "RATE_LIMITED") {
		t.Fatalf("rate limit code lost: %s", body)
	}
	for _, secret := range []string{"reset-token", "new-password", "upstream detail"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response leaked %q: %s", secret, body)
		}
	}
}

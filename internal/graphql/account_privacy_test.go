package graphql

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/overmindv/api-gateway/internal/apperror"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/middleware"
)

const privacyTestRequestID = "2f1d4a5e-3f6b-4d5a-8f2e-1c7d9a0b3e4f"

type privacyStub struct {
	userServiceStub
	kind, password string
	request        users.PrivacyRequest
	start          users.PrivacyStart
	export         users.AccountExport
	err            error
}

func (s *privacyStub) PreparePrivacyRequest(_ context.Context, kind, password string) (users.PrivacyStart, error) {
	s.kind, s.password = kind, password

	return s.start, s.err
}

func (s *privacyStub) ConfirmPrivacyRequest(context.Context, string, string) (users.PrivacyRequest, error) {
	return s.request, s.err
}

func (s *privacyStub) PrivacyRequestStatus(context.Context, string, string) (users.PrivacyRequest, error) {
	return s.request, s.err
}

func (s *privacyStub) AccountExport(context.Context, string) (users.AccountExport, error) {
	return s.export, s.err
}

func TestPrivacyRequestRequiresAuthForPrepare(t *testing.T) {
	t.Parallel()

	svc := &privacyStub{}
	resolver := &mutationResolver{&Resolver{Users: svc}}
	if _, err := resolver.RequestAccountExport(context.Background(), "password"); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("error = %v", err)
	}
	if _, err := resolver.RequestAccountDeletion(context.Background(), "password"); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("error = %v", err)
	}
}

func TestPrivacyPrepareUsesOwnerContractAndKeepsReceiptOutOfURL(t *testing.T) {
	t.Parallel()

	svc := &privacyStub{
		start: users.PrivacyStart{
			Request: users.PrivacyRequest{ID: privacyTestRequestID, Kind: "delete", State: "awaiting_confirmation", CreatedAt: "2026-11-26T10:00:00Z", ExpiresAt: "2026-11-26T10:15:00Z"},
			Receipt: "receipt-value",
		},
	}
	resolver := &mutationResolver{&Resolver{Users: svc}}
	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: "user", Token: "access"})
	result, err := resolver.RequestAccountDeletion(ctx, "current-password")
	if err != nil || result.Receipt != "receipt-value" || result.Request.Kind != "delete" {
		t.Fatalf("start: %v %v", result, err)
	}
	if svc.kind != "delete" || svc.password != "current-password" {
		t.Fatalf("prepare args: %q %q", svc.kind, svc.password)
	}
}

func TestPrivacyDeleteConfirmationClearsBrowserCookies(t *testing.T) {
	t.Parallel()

	svc := &privacyStub{request: users.PrivacyRequest{ID: privacyTestRequestID, Kind: "delete", State: "processing"}}
	resolver := &mutationResolver{&Resolver{Users: svc}}
	handler := middleware.SessionCookie(middleware.SessionConfig{Secure: true}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := middleware.ContextWithAuth(r.Context(), middleware.AuthInfo{UserID: "user", Token: "access"})
		if _, err := resolver.ConfirmPrivacyRequest(ctx, privacyTestRequestID, "receipt-value"); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	request := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: "access"})
	request.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: "refresh"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

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

func TestPrivacyStatusMapsOwnerStates(t *testing.T) {
	t.Parallel()

	svc := &privacyStub{request: users.PrivacyRequest{
		ID: privacyTestRequestID, Kind: "export", State: "processing", Publication: "pending",
		CreatedAt: "2026-11-26T10:00:00Z", ExpiresAt: "2026-11-26T10:15:00Z",
		Owners: []users.PrivacyOwnerStatus{{Owner: "tasks", State: "completed"}, {Owner: "feed", State: "failed", ErrorCode: "processing_failed"}},
	}}
	resolver := &queryResolver{&Resolver{Users: svc}}
	result, err := resolver.PrivacyRequest(context.Background(), privacyTestRequestID, "receipt-value")
	if err != nil {
		t.Fatal(err)
	}
	if result.Publication == nil || *result.Publication != "pending" || len(result.Owners) != 2 {
		t.Fatalf("status = %+v", result)
	}
	if result.Owners[1].ErrorCode == nil || *result.Owners[1].ErrorCode != "processing_failed" {
		t.Fatal("owner error code lost")
	}
}

func TestAccountExportRequiresAuthAndSortsServices(t *testing.T) {
	t.Parallel()

	svc := &privacyStub{export: users.AccountExport{
		Profile: users.AccountExportProfile{ID: privacyTestRequestID, Email: "user@example.com", Username: "user", Timezone: "UTC", Locale: "ru-RU", CreatedAt: "2026-01-01T00:00:00Z"},
		Services: map[string][]users.AccountExportPage{
			"tasks": {{
				UserID: privacyTestRequestID, Watermark: "w1",
				Records: []users.AccountExportRecord{{UserID: privacyTestRequestID, ID: privacyTestRequestID, Kind: "submission", OccurredAt: "2026-01-02T00:00:00Z", Title: "Task"}},
			}},
			"feed": {{UserID: privacyTestRequestID, Watermark: "w2"}},
		},
	}}
	resolver := &queryResolver{&Resolver{Users: svc}}
	if _, err := resolver.AccountExport(context.Background(), privacyTestRequestID); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("error = %v", err)
	}
	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: "user", Token: "access"})
	result, err := resolver.AccountExport(ctx, privacyTestRequestID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Services) != 2 || result.Services[0].Owner != "feed" || result.Services[1].Owner != "tasks" {
		t.Fatalf("services = %+v", result.Services)
	}
	if result.Profile.Email != "user@example.com" || result.Services[1].Pages[0].Records[0].Title == nil {
		t.Fatal("export mapping lost fields")
	}
}

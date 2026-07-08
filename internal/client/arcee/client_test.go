package arcee

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/laserbeak/internal/config"
	"github.com/overmindv/laserbeak/internal/middleware"
)

func TestClientMapsResponseAndForwardsProtectedHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer jwt" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get(middleware.RequestIDHeader) != "request-id" {
			t.Fatalf("request id missing")
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "GetUser") {
			t.Fatalf("unexpected GraphQL request: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"user-id","email":"user@example.com","username":"user","firstName":"","lastName":"","createdAt":"now","updatedAt":"now"}}}`))
	}))
	defer server.Close()

	client := New(config.Arcee{GraphQLURL: server.URL, HealthURL: server.URL, Timeout: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := middleware.WithRequestID(context.Background(), "request-id")
	ctx = middleware.ContextWithAuth(ctx, middleware.AuthInfo{UserID: "user-id", Token: "jwt"})

	user, err := client.GetUser(ctx, "user-id")
	if err != nil || user.ID != "user-id" {
		t.Fatalf("GetUser() = %+v, %v", user, err)
	}
}

func TestClientMapsGraphQLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"already exists","extensions":{"code":"ALREADY_EXISTS"}}],"data":null}`))
	}))
	defer server.Close()

	client := New(config.Arcee{
		GraphQLURL: server.URL,
		HealthURL:  server.URL,
		Timeout:    time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := client.Register(context.Background(), RegisterInput{})

	upstream, ok := err.(*Error)
	if !ok || upstream.Code != "ALREADY_EXISTS" {
		t.Fatalf("error = %#v", err)
	}
}

func TestClientHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()

	client := New(config.Arcee{GraphQLURL: server.URL, HealthURL: server.URL, Timeout: time.Second}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := client.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}

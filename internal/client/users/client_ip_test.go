package users

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/config"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func TestLoginSignsPeerIPAndIgnoresSpoofedHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Login-Client-IP")
		if ip != "192.0.2.42" {
			t.Errorf("IP = %q", ip)
		}
		ts := r.Header.Get("X-Login-Client-Time")
		issued, err := strconv.ParseInt(ts, 10, 64)
		if err != nil || time.Now().Unix()-issued > 5 {
			t.Error("invalid timestamp")
		}
		mac := hmac.New(sha256.New, []byte("shared-secret"))
		_, _ = mac.Write([]byte("login-client-ip-v1\n" + ts + "\n" + ip))
		actual, _ := hex.DecodeString(r.Header.Get("X-Login-Client-Signature"))
		if !hmac.Equal(mac.Sum(nil), actual) {
			t.Error("invalid signature")
		}
		if r.Header.Get("X-Forwarded-For") != "" {
			t.Error("forwarded untrusted header")
		}
		_, _ = w.Write([]byte(`{"data":{"login":{"token":"test","user":{"id":"user"}}}}`))
	}))
	defer server.Close()
	client := New(config.Users{GraphQLURL: server.URL, Timeout: time.Second, LoginContextSecret: "shared-secret"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := middleware.ClientIPMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if _, err := client.Login(r.Context(), LoginInput{Email: "user@example.com", Password: "test"}); err != nil {
			t.Error(err)
		}
	}))
	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	req.RemoteAddr = "[::ffff:192.0.2.42]:1111"
	req.Header.Set("X-Forwarded-For", "198.51.100.9")
	req.Header.Set("X-Login-Client-IP", "198.51.100.9")
	req.Header.Set("X-Login-Client-Time", "999999999")
	req.Header.Set("X-Login-Client-Signature", "forged")
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestBrowserSecurityOrigins(t *testing.T) {
	for _, contentType := range []string{"application/json", "multipart/form-data; boundary=test"} {
		for _, tc := range []struct {
			name, origin string
			cookie       bool
			allowed      bool
		}{
			{"same origin", "https://learn.example", true, true},
			{"foreign origin", "https://attacker.example", true, false},
			{"login csrf", "https://attacker.example", false, false},
			{"missing origin", "", true, false},
			{"null origin", "null", true, false},
			{"non-browser", "", false, true},
		} {
			t.Run(contentType+tc.name, func(t *testing.T) {
				called := false
				h := BrowserSecurity(BrowserSecurityConfig{AllowedOrigins: []string{"https://learn.example"}}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(204) }))
				r := httptest.NewRequest("POST", "/graphql", strings.NewReader("test"))
				r.Header.Set("Content-Type", contentType)
				r.Header.Set("Origin", tc.origin)
				if tc.cookie {
					r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "token"})
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if called != tc.allowed {
					t.Fatalf("handler called=%v", called)
				}
				if !tc.allowed && (w.Code != 403 || !strings.Contains(w.Body.String(), "ORIGIN_NOT_ALLOWED")) {
					t.Fatalf("response: %d %s", w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestBrowserSecurityHTTPS(t *testing.T) {
	for _, tc := range []struct {
		name, peer, forwarded string
		tls, allowed          bool
	}{
		{"tls", "192.0.2.1:10", "", true, true},
		{"plaintext", "192.0.2.1:10", "", false, false},
		{"spoof", "192.0.2.1:10", "https", false, false},
		{"trusted proxy", "10.0.0.2:10", "https", false, true},
		{"ambiguous proxy", "10.0.0.2:10", "https,http", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := BrowserSecurity(BrowserSecurityConfig{Production: true, TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(204) }))
			r := httptest.NewRequest("POST", "/graphql", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-Proto", tc.forwarded)
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if called != tc.allowed {
				t.Fatalf("handler called=%v code=%d", called, w.Code)
			}
		})
	}
}

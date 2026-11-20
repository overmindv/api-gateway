package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
)

// BrowserSecurityConfig задаёт доверенные origins и транспорт production.
type BrowserSecurityConfig struct {
	AllowedOrigins []string
	Production     bool
	TrustedProxies []netip.Prefix
}

// BrowserSecurity проверяет браузерную границу до чтения JSON/multipart тела.
func BrowserSecurity(cfg BrowserSecurityConfig, next http.Handler) http.Handler {
	origins := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		origins[origin] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			next.ServeHTTP(w, r)

			return
		}
		if cfg.Production && !secureTransport(r, cfg.TrustedProxies) {
			browserError(w, "HTTPS_REQUIRED")

			return
		}
		if cfg.Production {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		origin := r.Header.Get("Origin")
		_, cookieErr := r.Cookie(SessionCookieName)
		if len(r.Header.Values("Origin")) > 1 || (origin != "" && !origins[origin]) || (origin == "" && cookieErr == nil) {
			browserError(w, "ORIGIN_NOT_ALLOWED")

			return
		}
		next.ServeHTTP(w, r)
	})
}

func secureTransport(r *http.Request, proxies []netip.Prefix) bool {
	if r.TLS != nil {
		return true
	}
	if len(r.Header.Values("X-Forwarded-Proto")) != 1 || r.Header.Get("X-Forwarded-Proto") != "https" {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	for _, prefix := range proxies {
		if prefix.Contains(addr.Unmap()) {
			return true
		}
	}

	return false
}

func browserError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"message": "Не удалось выполнить действие.", "extensions": map[string]string{"code": code}}}})
}

package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
)

type clientIPKey struct{}

// ClientIP возвращает только IP транспортного peer; XFF и пользовательские заголовки игнорируются.
func ClientIP(ctx context.Context) string {
	value, _ := ctx.Value(clientIPKey{}).(string)

	return value
}

// ClientIPMiddleware извлекает адрес до вызова GraphQL-резолвера.
func ClientIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		addr, err := netip.ParseAddr(host)
		if err == nil {
			r = r.WithContext(context.WithValue(r.Context(), clientIPKey{}, addr.Unmap().WithZone("").String()))
		}
		next.ServeHTTP(w, r)
	})
}

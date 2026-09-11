package middleware

import (
	"context"
	"net/http"
	"time"
)

// SessionCookieName задаёт имя httpOnly session cookie. JS прочитать её не может —
// это устраняет кражу JWT через XSS из localStorage.
const SessionCookieName = "ovm_session"

// SessionConfig описывает атрибуты session cookie.
type SessionConfig struct {
	Secure bool
}

// sessionKey задаёт приватный ключ context для состояния ответа.
type sessionKey struct{}

// sessionState накапливает решение о cookie во время обработки запроса.
type sessionState struct {
	token  string
	maxAge time.Duration
	clear  bool
}

// SetSessionToken помечает ответ, чтобы SessionCookie выставил httpOnly cookie с токеном.
// Вызывается login/register resolver после успешной аутентификации.
func SetSessionToken(ctx context.Context, token string, maxAge time.Duration) {
	if state, ok := ctx.Value(sessionKey{}).(*sessionState); ok {
		state.token = token
		state.maxAge = maxAge
	}
}

// ClearSessionToken помечает ответ, чтобы SessionCookie удалил httpOnly cookie.
// Вызывается logout resolver.
func ClearSessionToken(ctx context.Context) {
	if state, ok := ctx.Value(sessionKey{}).(*sessionState); ok {
		state.clear = true
	}
}

// SessionCookie выдаёт и очищает httpOnly session cookie на границе браузер <-> api-gateway.
// Cookie ставится после завершения обработки, поэтому writer откладывает отправку заголовков
// до первого Write (график GraphQL всегда пишет тело JSON).
func SessionCookie(cfg SessionConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := &sessionState{}
		sw := &sessionResponseWriter{ResponseWriter: w, state: state, secure: cfg.Secure}
		next.ServeHTTP(sw, r.WithContext(context.WithValue(r.Context(), sessionKey{}, state)))

		if !sw.committed {
			sw.commitHeaders()
		}
	})
}

// sessionResponseWriter откладывает отправку заголовков до установки cookie.
type sessionResponseWriter struct {
	http.ResponseWriter
	state     *sessionState
	secure    bool
	committed bool
	status    int
}

func (w *sessionResponseWriter) WriteHeader(status int) {
	if w.committed {
		return
	}
	w.status = status
}

func (w *sessionResponseWriter) commitHeaders() {
	if w.committed {
		return
	}
	w.committed = true

	var cookie *http.Cookie
	switch {
	case w.state.clear:
		cookie = &http.Cookie{
			Name: SessionCookieName, Value: "", Path: "/",
			HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: w.secure,
			MaxAge: -1, Expires: time.Unix(1, 0),
		}
	case w.state.token != "":
		cookie = &http.Cookie{
			Name: SessionCookieName, Value: w.state.token, Path: "/",
			HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: w.secure,
			MaxAge: int(w.state.maxAge.Seconds()),
		}
	}
	if cookie != nil {
		http.SetCookie(w.ResponseWriter, cookie)
	}

	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *sessionResponseWriter) Write(b []byte) (int, error) {
	w.commitHeaders()
	return w.ResponseWriter.Write(b)
}

func (w *sessionResponseWriter) Flush() {
	w.commitHeaders()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

// RequestIDMiddleware добавляет request_id в response header и context запроса.
// На вход получает следующий handler, на выход возвращает middleware handler с сохранением X-Request-ID.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(RequestIDHeader))
		if requestID == "" || len(requestID) > 128 {
			requestID = uuid.NewString()
		}

		w.Header().Set(RequestIDHeader, requestID)
		next.ServeHTTP(w, r.WithContext(WithRequestID(r.Context(), requestID)))
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader сохраняет HTTP status для последующего request logging.
// На вход получает status code, на выход записывает его в исходный ResponseWriter.
func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write сохраняет размер response body для request logging.
// На вход получает байты ответа, на выход возвращает результат исходного ResponseWriter.
func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	written, err := r.ResponseWriter.Write(body)
	r.bytes += written

	return written, err
}

// Logging пишет краткую запись о HTTP-запросе, пропуская пути из ignoredPaths.
// На вход получает logger, следующий handler и список исключённых путей, на выход возвращает HTTP middleware.
func Logging(log *slog.Logger, next http.Handler, ignoredPaths ...string) http.Handler {
	ignored := make(map[string]struct{}, len(ignoredPaths))
	for _, path := range ignoredPaths {
		ignored[path] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		if _, ok := ignored[r.URL.Path]; ok || log == nil {
			return
		}
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		log.InfoContext(r.Context(), "http request",
			"request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path,
			"status", status, "response_bytes", recorder.bytes, "duration", time.Since(started),
		)
	})
}

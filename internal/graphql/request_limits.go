package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"
)

const graphqlJSONLimit = 1 << 20

// boundedRequest проверяет весь bounded body до gqlgen, в том числе chunked upload.
func boundedRequest(next http.Handler, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		r = r.WithContext(ctx)
		if r.Method == http.MethodPost {
			mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			limit := int64(graphqlJSONLimit)
			if mediaType == "multipart/form-data" {
				limit = graphqlMultipartLimit
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
			if err != nil {
				requestLimitError(w, http.StatusBadRequest, "INVALID_REQUEST")

				return
			}
			if int64(len(body)) > limit {
				requestLimitError(w, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE")

				return
			}
			if mediaType == "application/json" && bytes.HasPrefix(bytes.TrimSpace(body), []byte("[")) {
				requestLimitError(w, http.StatusBadRequest, "BATCH_NOT_ALLOWED")

				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		next.ServeHTTP(w, r)
	})
}

func requestLimitError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"message": genericActionMessage, "extensions": map[string]string{"code": code}}}})
}

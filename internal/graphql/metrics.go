package graphql

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Metrics хранит process-local счётчики GraphQL orchestration.
type Metrics struct {
	avatarResolutionFailures atomic.Uint64
}

// RecordAvatarResolutionFailure учитывает fail-soft ошибку batch resolution аватаров.
func (m *Metrics) RecordAvatarResolutionFailure() {
	if m != nil {
		m.avatarResolutionFailures.Add(1)
	}
}

// Handler отдаёт счётчики в Prometheus text format.
func (m *Metrics) Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "gateway_avatar_resolution_failures_total %d\n", m.avatarResolutionFailures.Load())
}

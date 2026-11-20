package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/tasks"
)

func TestQueryLimitsRejectBeforeFanout(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer upstream.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Handler(nil, entities.New(upstream.URL, time.Second, log), nil, log)
	aliases := ""
	for i := 0; i < 21; i++ {
		aliases += fmt.Sprintf("a%d:topics{ id } ", i)
	}
	deep := "topic { id }"
	for i := 0; i < 18; i++ {
		deep = "children { " + deep + " }"
	}
	for _, tc := range []struct{ name, query, code string }{
		{"aliases", "{" + aliases + "}", "OPERATION_LIMIT_EXCEEDED"},
		{"fragments", "{...Many} fragment Many on Query {" + aliases + "}", "OPERATION_LIMIT_EXCEEDED"},
		{"pagination", "{topics(pagination:{limit:100000}){id title}}", "COMPLEXITY_LIMIT_EXCEEDED"},
		{"depth", "{topicTree {" + deep + "}}", "OPERATION_LIMIT_EXCEEDED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := practiceOperationRequest(t, h, tc.query, "", nil)
			if !strings.Contains(body, tc.code) {
				t.Fatalf("missing %s: %s", tc.code, body)
			}
			if calls.Load() != 0 {
				t.Fatal("rejected query called upstream")
			}
		})
	}
}

func TestQueryHTTPBounds(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Handler(nil, nil, nil, log)
	valid := `{"query":"{__typename}"}`
	for _, tc := range []struct{ name, body, code string }{
		{"exact", valid + strings.Repeat(" ", (1<<20)-len(valid)), "Query"},
		{"too large", valid + strings.Repeat(" ", (1<<20)-len(valid)+1), "REQUEST_TOO_LARGE"},
		{"batch", `[{"query":"{__typename}"}]`, "BATCH_NOT_ALLOWED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("POST", "/graphql", strings.NewReader(tc.body))
			req.ContentLength = -1
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if !strings.Contains(w.Body.String(), tc.code) {
				t.Fatalf("response: %s", w.Body.String())
			}
		})
	}
}

func TestWeightedComplexitySaturates(t *testing.T) {
	t.Parallel()

	if got := weightedCost(1000000, int(^uint(0)>>1)); got <= queryComplexityLimit {
		t.Fatalf("overflow: %d", got)
	}
	if got := weightedCost(2, 100); got != 201 {
		t.Fatalf("weight: %d", got)
	}
}

func TestQueryDeadlineCancelsUpstream(t *testing.T) {
	t.Parallel()

	canceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done(); close(canceled) }))
	defer upstream.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := HandlerWithRecommendationsAndMetrics(nil, entities.New(upstream.URL, time.Second, log), nil, nil, nil, nil, nil, log, nil, HandlerOptions{RequestTimeout: 100 * time.Millisecond})
	payload, _ := json.Marshal(map[string]string{"query": "{topics {id}}"})
	req := httptest.NewRequest("POST", "/graphql", bytes.NewReader(payload)).WithContext(context.Background())
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("upstream not canceled")
	}
	if !strings.Contains(w.Body.String(), "DEADLINE_EXCEEDED") {
		t.Fatalf("response: %s", w.Body.String())
	}
}

func TestASTBoundaryAndExpandedFragments(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Handler(nil, nil, nil, log)
	roots := ""
	for i := 0; i < 20; i++ {
		roots += fmt.Sprintf("a%d:__typename ", i)
	}
	for _, tc := range []struct {
		query   string
		allowed bool
	}{
		{"{" + roots + "}", true}, {"{" + roots + "extra:__typename}", false},
		{`{__type(name:"Query"){` + strings.Repeat("name ", 999) + "}}", true},
		{`{__type(name:"Query"){` + strings.Repeat("name ", 1000) + "}}", false},
	} {
		body := practiceOperationRequest(t, h, tc.query, "", nil)
		rejected := strings.Contains(body, "OPERATION_LIMIT_EXCEEDED")
		if rejected == tc.allowed {
			t.Fatalf("allowed=%v: %s", tc.allowed, body)
		}
	}
	query := `{__type(name:"Query"){...F10}} fragment F0 on __Type {name}`
	for i := 1; i <= 10; i++ {
		query += fmt.Sprintf(" fragment F%d on __Type {...F%d ...F%d}", i, i-1, i-1)
	}
	if body := practiceOperationRequest(t, h, query, "", nil); !strings.Contains(body, "OPERATION_LIMIT_EXCEEDED") {
		t.Fatalf("fragment expansion: %s", body)
	}
}

func TestPaginationVariablesAndOrdinaryHistory(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[],"limit":100,"offset":0}`)
	}))
	defer upstream.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Handler(nil, nil, tasks.New(upstream.URL, time.Second, log), log)
	query := `query($pagination:PaginationInput){myITCodeSubmissions(pagination:$pagination){limit offset items{
 runMode id userId taskId taskVersionId taskVersionNumber executionId correlationId language sourceFileName status verdict
 compilation{exitCode stdout stderr durationMs memoryBytes} execution{exitCode stdout stderr durationMs memoryBytes}
 tests{testId verdict stdout stderr durationMs memoryBytes} failure{code message} createdAt updatedAt completedAt
 }}}`
	for _, limit := range []int{100, 100000} {
		body := practiceOperationRequest(t, h, query, "", map[string]any{"pagination": map[string]any{"limit": limit}})
		if limit == 100 && (strings.Contains(body, `"errors"`) || calls.Load() != 1) {
			t.Fatalf("ordinary history: %s calls=%d", body, calls.Load())
		}
		if limit > 100 && (!strings.Contains(body, "COMPLEXITY_LIMIT_EXCEEDED") || calls.Load() != 1) {
			t.Fatalf("variables bypass: %s calls=%d", body, calls.Load())
		}
	}
}

func TestMultipartWholeBodyBound(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Handler(nil, nil, nil, log)
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(strings.Repeat("x", graphqlMultipartLimit+1)))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 413 || !strings.Contains(w.Body.String(), "REQUEST_TOO_LARGE") {
		t.Fatalf("upload limit: %d %s", w.Code, w.Body.String())
	}
}

func TestFrontendTopicsQueryFitsBudget(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer upstream.Close()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := Handler(nil, entities.New(upstream.URL, time.Second, log), nil, log)
	// Контракт существующего frontend/src/api/catalog.ts TOPICS_QUERY.
	query := `query Topics($parentId: ID, $search: String = "") {
 topics(courseId: $parentId, filter: { search: $search }) { ...TopicFields }
 topicTree(courseId: $parentId) { topic { ...TopicFields } children { topic { ...TopicFields } children { topic { ...TopicFields } } } }
 } fragment TopicFields on Topic { id courseId parentTopicId title slug description orderIndex difficulty status createdAt updatedAt }`
	body := practiceOperationRequest(t, h, query, "", nil)
	if strings.Contains(body, `"errors"`) || calls.Load() != 2 {
		t.Fatalf("topics page: %s calls=%d", body, calls.Load())
	}
}

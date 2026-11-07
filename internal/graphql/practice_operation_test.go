package graphql

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/client/tasks"
)

func practiceOperationRequest(t *testing.T, handler http.Handler, query, operationName string, variables map[string]any) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "operationName": operationName, "variables": variables})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body)).WithContext(practiceContext())
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Body.String()
}

func TestPracticePlanRejectsAliasFanoutBeforeUpstream(t *testing.T) {
	field := `practicePlan(topicIds:["` + practiceTopic + `"],minutes:60){totalMinutes}`
	for _, tc := range []struct{ name, query string }{
		{"aliases", `{first:` + field + ` second:` + field + `}`},
		{"named fragments", `query { ...First ...Second } fragment First on Query {first:` + field + `} fragment Second on Query {second:` + field + `}`},
		{"inline fragments", `{... on Query {first:` + field + `} ... on Query {second:` + field + `}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, `{"code":"UNAVAILABLE"}`)
			}))
			defer upstream.Close()
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := HandlerWithRecommendationsAndMetrics(nil, entities.New(upstream.URL, time.Second, log), tasks.New(upstream.URL, time.Second, log), nil, nil, nil, recommendations.New(upstream.URL, "service-token", time.Second, log), log, nil)
			body := practiceOperationRequest(t, handler, tc.query, "", nil)
			if !strings.Contains(body, `"code":"OPERATION_LIMIT_EXCEEDED"`) || calls.Load() != 0 {
				t.Fatalf("fanout must fail before upstream: calls=%d response=%s", calls.Load(), body)
			}
		})
	}
}

func TestPracticePlanAllowsOneCollectedFieldPerOperation(t *testing.T) {
	args := `(topicIds:["` + practiceTopic + `"],minutes:60)`
	for _, tc := range []struct {
		name, query, operationName string
		variables                  map[string]any
	}{
		{name: "single", query: `{practicePlan` + args + `{totalMinutes}}`},
		{name: "merged fragment", query: `query {plan:practicePlan` + args + `{totalMinutes} ...Same} fragment Same on Query {plan:practicePlan` + args + `{policyVersion}}`},
		{name: "skipped alias", query: `query($skip:Boolean!){first:practicePlan` + args + `{totalMinutes} second:practicePlan` + args + ` @skip(if:$skip){totalMinutes}}`, variables: map[string]any{"skip": true}},
		{name: "selected operation only", query: `query Preview {practicePlan` + args + `{totalMinutes}} query Unselected {first:practicePlan` + args + `{totalMinutes} second:practicePlan` + args + `{totalMinutes}}`, operationName: "Preview"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &practiceFixture{}
			resolver := fixture.resolver(t)
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := HandlerWithRecommendationsAndMetrics(nil, resolver.Catalog, resolver.Tasks, nil, nil, nil, resolver.Recommendations, log, nil)
			body := practiceOperationRequest(t, handler, tc.query, tc.operationName, tc.variables)
			if strings.Contains(body, `"errors"`) || !strings.Contains(body, `"totalMinutes":10`) || fixture.recommendationCalls != 1 {
				t.Fatalf("one collected field should execute once: calls=%d response=%s", fixture.recommendationCalls, body)
			}
		})
	}
}

func TestPracticePlanOperationLimitPreservesExistingQueries(t *testing.T) {
	handler := Handler(nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	body := practiceOperationRequest(t, handler, `{first:__typename second:__typename}`, "", nil)
	if body != `{"data":{"first":"Query","second":"Query"}}` {
		t.Fatalf("unrelated query changed: %s", body)
	}
}

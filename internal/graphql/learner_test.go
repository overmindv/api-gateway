package graphql

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/middleware"
)

const learnerTestActor = "jwt-user"

func TestLearnerGoalMutationUsesOnlyJWTActorAndConcurrencyArguments(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/learner/goals/goal-id" {
			t.Fatalf("unexpected upstream request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-User-ID") != learnerTestActor || r.Header.Get("Idempotency-Key") != "request-id" || r.Header.Get("X-Expected-Version") != "3" {
			t.Fatalf("wrong authority/concurrency headers: %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"goal-id","title":"Exam","target_topics":["topic-id"],"weekly_minutes":120,"active":true,"languages":[],"formats":[],"self_assessments":[],"timezone":"Europe/Madrid","version":4,"created_at":"2026-11-23T10:00:00Z","updated_at":"2026-11-23T10:01:00Z","skills":[{"topic_id":"topic-id","state":"unknown"}],"feasibility":"not_evaluated"}`)
	}))
	defer upstream.Close()

	client := recommendations.New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := HandlerWithRecommendationsAndMetrics(nil, nil, nil, nil, nil, nil, client, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	body := `{"query":"mutation { updateLearnerGoal(id:\"goal-id\", input:{title:\"Exam\",targetTopics:[\"topic-id\"],weeklyMinutes:120,active:true,languages:[],formats:[],selfAssessments:[]}, expectedVersion:3, requestId:\"request-id\") { id version skills { topicId state } } }"}`
	response := learnerGraphQLRequest(t, handler, body, learnerTestActor)
	if !strings.Contains(response, `"version":4`) || !strings.Contains(response, `"topicId":"topic-id"`) || strings.Contains(response, `"errors"`) {
		t.Fatalf("unexpected GraphQL response: %s", response)
	}
}

func TestLearnerDiagnosticPreservesSuccessfulStatesAndEvidenceTraceability(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-ID") != learnerTestActor {
			t.Fatalf("actor = %q, want jwt-user", r.Header.Get("X-User-ID"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/learner/goals/goal-id/diagnostic/skip":
			_, _ = io.WriteString(w, `{"id":"session-id","goal_id":"goal-id","goal_version":2,"status":"skipped","baseline":"unknown","tasks":[],"estimated_minutes":0,"stop_rule":{"policy_version":"diagnostic-v1","max_tasks":4,"budget_minutes":0},"started_at":"2026-11-23T10:00:00Z","expires_at":"2026-11-24T10:00:00Z","skipped_at":"2026-11-23T10:00:01Z"}`)
		case "/v1/learner/goals/goal-id/diagnostic/finalize":
			_, _ = io.WriteString(w, `{"id":"session-id","goal_id":"goal-id","goal_version":2,"status":"finalized","baseline":"measured","tasks":[],"estimated_minutes":10,"stop_rule":{"policy_version":"diagnostic-v1","max_tasks":4,"budget_minutes":10},"started_at":"2026-11-23T10:00:00Z","expires_at":"2026-11-24T10:00:00Z","finalized_at":"2026-11-23T10:05:00Z","report":{"assigned":1,"assessed":1,"topics":[],"outcomes":[{"task_id":"task-id","task_version_id":"version-id","topic_id":"topic-id","submission_id":"submission-id","event_id":"event-id","correction_version":7,"verdict":"accepted","graded_at":"2026-11-23T10:04:00Z"}]}}`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	client := recommendations.New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handler := HandlerWithRecommendationsAndMetrics(nil, nil, nil, nil, nil, nil, client, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	skipped := learnerGraphQLRequest(t, handler, `{"query":"mutation { skipLearnerDiagnostic(goalId:\"goal-id\", goalVersion:2) { status tasks { taskId } } }"}`, learnerTestActor)
	if !strings.Contains(skipped, `"status":"skipped"`) || !strings.Contains(skipped, `"tasks":[]`) {
		t.Fatalf("unexpected skipped response: %s", skipped)
	}
	finalized := learnerGraphQLRequest(t, handler, `{"query":"mutation { finalizeLearnerDiagnostic(goalId:\"goal-id\", goalVersion:2, sessionId:\"session-id\", allowPartial:true) { report { outcomes { submissionId eventId correctionVersion } } } }"}`, learnerTestActor)
	for _, want := range []string{`"submissionId":"submission-id"`, `"eventId":"event-id"`, `"correctionVersion":7`} {
		if !strings.Contains(finalized, want) {
			t.Fatalf("missing %s in %s", want, finalized)
		}
	}
}

func TestLearnerOperationsDoNotExposeUserIDArgument(t *testing.T) {
	t.Parallel()

	handler := Handler(nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	response := learnerGraphQLRequest(t, handler, `{"query":"{ learnerProfile(userId:\"other-user\") { availability { version } } }"}`, learnerTestActor)
	if !strings.Contains(response, `"code":"GRAPHQL_VALIDATION_FAILED"`) {
		t.Fatalf("expected schema to reject userId, got %s", response)
	}
}

func learnerGraphQLRequest(t *testing.T, handler http.Handler, body, actor string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: actor, Token: "jwt"}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	return response.Body.String()
}

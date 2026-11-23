package graphql

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/recommendations"
)

const learnerStudyPlanPath = "/v1/learner/goals/goal-id/study-plan"

func TestCurrentLearningPlanPreservesUnavailableCatalogFallbackExactly(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != learnerStudyPlanPath || r.Header.Get("X-User-ID") != learnerTestActor {
			t.Fatalf("unexpected request %s %s actor=%q", r.Method, r.URL.Path, r.Header.Get("X-User-ID"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, studyPlanFallbackFixture)
	}))
	defer upstream.Close()

	response := studyPlanGraphQL(t, upstream.URL, `{ currentLearningPlan(goalId:"goal-id") { version stale evidenceWatermark freshness { status } blockedReason nextStep current } }`)
	for _, want := range []string{`"version":4`, `"stale":false`, `"evidenceWatermark":"watermark"`, `"status":"unavailable"`, `"blockedReason":"catalog_unavailable"`, `"nextStep":null`, `"current":true`} {
		if !strings.Contains(response, want) {
			t.Fatalf("missing %s in %s", want, response)
		}
	}
}

func TestLearningPlanExplanationPreservesReasonAndEvidenceReference(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/v1/learner/goals/goal-id/study-plan/explanation?plan_version=4" {
			t.Fatalf("unexpected URI %s", r.URL.RequestURI())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"plan_version":4,"stale":false,"steps":[{"ordinal":1,"reasons":[{"code":"topic_gap","topic_id":"topic-id","message":"Gap"},{"code":"time_fit","minutes":20,"message":"Fits"}],"evidence":{"topic_id":"topic-id","score_bps":3500,"sample_count":2,"last_evidence_at":"2026-11-23T09:00:00Z","uncertainty":"sufficient_for_policy","ref":{"watermark":"evidence-watermark","policy_revision":"mastery-v1","captured_at":"2026-11-23T09:30:00Z"}}}],"limits":[]}`)
	}))
	defer upstream.Close()

	response := studyPlanGraphQL(t, upstream.URL, `{ learningPlanExplanation(goalId:"goal-id", planVersion:4) { steps { ordinal reasons { code topicId minutes } evidence { scoreBps ref { watermark policyRevision } } } } }`)
	for _, want := range []string{`{"code":"topic_gap","topicId":"topic-id","minutes":null}`, `{"code":"time_fit","topicId":null,"minutes":20}`, `"scoreBps":3500`, `"watermark":"evidence-watermark"`, `"policyRevision":"mastery-v1"`} {
		if !strings.Contains(response, want) {
			t.Fatalf("missing %s in %s", want, response)
		}
	}
}

func TestCurrentLearningPlanPreservesMissingBlockedReason(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, strings.Replace(studyPlanFallbackFixture, `,"blocked_reason":"catalog_unavailable"`, "", 1))
	}))
	defer upstream.Close()

	response := studyPlanGraphQL(t, upstream.URL, `{ currentLearningPlan(goalId:"goal-id") { blockedReason } }`)
	if !strings.Contains(response, `"blockedReason":null`) || strings.Contains(response, `"errors"`) {
		t.Fatalf("missing blocked reason was synthesized: %s", response)
	}
}

func TestLearningPlanWritesPreserveStaleRevisionAndDoNotTreatClickAsOutcome(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case learnerStudyPlanPath:
			if r.Header.Get("Idempotency-Key") != "generation-id" {
				t.Fatalf("missing generation idempotency key")
			}
			w.WriteHeader(http.StatusConflict)
			_, _ = io.WriteString(w, `{"code":"VERSION_CONFLICT","message":"private"}`)
		case "/v1/learner/goals/goal-id/study-plan/steps/1/start":
			_, _ = io.WriteString(w, strings.Replace(studyPlanFallbackFixture, `"weeks":[]`, `"weeks":[{"from":"2026-11-23","through":"2026-11-29","capacity_minutes":120,"planned_minutes":20,"items":[{"kind":"theory","resource_id":"material-id","revision_id":"revision-id","topic_id":"topic-id","estimated_minutes":20,"score":10,"components":{"gap":10,"difficulty":0,"duration":0,"variety":0,"due_review":0},"stage":"theory","ordinal":1,"activity_started_at":"2026-11-23T10:05:00Z"}]}]`, 1))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	conflict := studyPlanGraphQL(t, upstream.URL, `mutation { generateLearningPlan(goalId:"goal-id", goalVersion:2, availabilityVersion:3, planVersion:3, requestId:"generation-id") { version } }`)
	if !strings.Contains(conflict, `"code":"VERSION_CONFLICT"`) {
		t.Fatalf("unexpected conflict response: %s", conflict)
	}
	started := studyPlanGraphQL(t, upstream.URL, `mutation { startLearningPlanStep(goalId:"goal-id", planVersion:4, ordinal:1) { weeks { items { activityStartedAt activityCompletedAt assessment { verdict } } } } }`)
	if !strings.Contains(started, `"activityStartedAt":"2026-11-23T10:05:00Z"`) || !strings.Contains(started, `"activityCompletedAt":null`) || !strings.Contains(started, `"assessment":null`) {
		t.Fatalf("resource click became authoritative outcome: %s", started)
	}
}

func studyPlanGraphQL(t *testing.T, upstreamURL, operation string) string {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := recommendations.New(upstreamURL, "service-secret", time.Second, log)
	handler := HandlerWithRecommendationsAndMetrics(nil, nil, nil, nil, nil, nil, client, log, nil)

	return learnerGraphQLRequest(t, handler, `{"query":`+strconvQuote(operation)+`}`, learnerTestActor)
}

func strconvQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)

	return `"` + value + `"`
}

const studyPlanFallbackFixture = `{"goal_id":"goal-id","version":4,"goal_version":2,"availability_version":3,"target_topics":["topic-id"],"planner_version":"learning-plan-v1","ranking_algorithm":"resource-ranking-v1","ranking_policy":{"policy_version":"policy-v1","target_bps":8000,"unknown_gap_bps":5000,"prerequisite_min_bps":6000,"medium_difficulty_bps":4000,"hard_difficulty_bps":7000,"weights":{"gap":1,"difficulty":1,"duration":1,"variety":1,"due_review":1}},"evidence_watermark":"watermark","evidence_policy":{"revision":"mastery-v1","algorithm":"weighted-v1","gap_max_bps":3000,"demonstrated_min_bps":8000,"min_weighted_tasks":2},"created_at":"2026-11-23T10:00:00Z","timezone":"Europe/Madrid","weekly_minutes":120,"horizon_limited":false,"weeks":[],"unmet_outcomes":[{"topic_id":"topic-id","stage":"theory","reason":"missing_resource","blocked_by":[]}],"coverage_gaps":[],"suggestions":[],"current":true,"stale":false,"freshness":{"status":"unavailable"},"next_step":null,"blocked_reason":"catalog_unavailable","goal_demonstrated":false}`

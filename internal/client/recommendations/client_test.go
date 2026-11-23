package recommendations

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

func TestPracticePlanUsesServiceTokenAndMinimalContract(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/practice-plan" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer service-secret" || r.Header.Get("X-User-ID") != "verified-user" {
			t.Errorf("wrong credentials")
		}
		if r.Header.Get("X-User-Roles") != "" {
			t.Errorf("roles should not be forwarded")
		}
		if r.Header.Get("X-Request-ID") != "request-id" {
			t.Errorf("missing request ID")
		}
		data, _ := io.ReadAll(r.Body)
		for _, secret := range []string{"user-jwt", "selected_option", "correct_option", "source_code", "hidden_tests"} {
			if strings.Contains(string(data), secret) {
				t.Errorf("private data forwarded: %s", secret)
			}
		}
		var input map[string]json.RawMessage
		if err := json.Unmarshal(data, &input); err != nil {
			t.Fatal(err)
		}
		if len(input) != 6 || string(input["input_truncated"]) != "false" {
			t.Errorf("unexpected payload: %s", data)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"policy_version":"practice-v1","total_minutes":10,"warnings":[],"items":[{"task_id":"task","task_version_id":"version","topic_id":"topic","title":"Task","difficulty":"easy","task_type":"single_choice","estimated_minutes":10,"reason_code":"START_TOPIC"}]}`)
	}))
	defer upstream.Close()
	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: "verified-user", Token: "user-jwt", Roles: []string{"admin"}})
	ctx = middleware.WithRequestID(ctx, "request-id")
	client := New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	result, err := client.PracticePlan(ctx, Request{TopicIDs: []string{"topic"}, Minutes: 60, Topics: []Topic{}, Tasks: []Task{}, Evidence: []Evidence{}}, "verified-user")
	if err != nil {
		t.Fatal(err)
	}
	if result.PolicyVersion != "practice-v1" || len(result.Items) != 1 || result.Items[0].TaskVersionID != "version" {
		t.Fatalf("unexpected plan: %+v", result)
	}
}

//nolint:paralleltest // subtests совместно проверяют один log sink и redirect counter
func TestPracticePlanHidesUpstreamDetailsAndRejectsRedirect(t *testing.T) {
	t.Parallel()

	var logs strings.Builder
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls++; w.WriteHeader(200) }))
	defer target.Close()
	for _, status := range []int{400, 503, 307} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status == 307 {
					w.Header().Set("Location", target.URL)
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"code":"INVALID_ARGUMENT","message":"secret upstream host and token"}`)
			}))
			defer upstream.Close()
			client := New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(&logs, nil)))
			_, err := client.PracticePlan(context.Background(), Request{}, "user")
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
	if targetCalls != 0 {
		t.Fatalf("followed redirect carrying actor/token")
	}
	if strings.Contains(logs.String(), "secret upstream") {
		t.Fatal("logged upstream private message")
	}
}

func TestPracticePlanPreservesSafeUpstreamErrorCode(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"code":"VERSION_CONFLICT","message":"private detail"}`)
	}))
	defer upstream.Close()

	client := New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := client.PracticePlan(context.Background(), Request{}, "user")
	var upstreamErr *Error
	if !errors.As(err, &upstreamErr) {
		t.Fatalf("error = %v, want recommendations.Error", err)
	}
	if upstreamErr.Code != "VERSION_CONFLICT" {
		t.Fatalf("code = %q, want VERSION_CONFLICT", upstreamErr.Code)
	}
}

func TestLearnerClientForwardsActorAndConcurrencyMetadata(t *testing.T) {
	t.Parallel()

	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.Header.Get("X-User-ID"); got != "jwt-user" {
			t.Fatalf("X-User-ID = %q, want jwt-user", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/learner/goal-templates":
			_, _ = io.WriteString(w, `[{"version":"exam:1","title":"Exam","suggests_deadline":true}]`)
		case r.Method == http.MethodPut && r.URL.Path == "/v1/learner/profile":
			if r.Header.Get("Idempotency-Key") != "profile-write" || r.Header.Get("X-Expected-Version") != "4" {
				t.Fatalf("wrong profile concurrency headers: %v", r.Header)
			}
			_, _ = io.WriteString(w, `{"availability":{"weekly_minutes":180,"timezone":"Europe/Madrid","version":5,"updated_at":"2026-11-23T10:00:00Z"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/learner/goals":
			if r.Header.Get("Idempotency-Key") != "goal-write" || r.Header.Get("X-Expected-Version") != "" {
				t.Fatalf("wrong create headers: %v", r.Header)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":"goal-id","title":"Exam","target_topics":["topic-id"],"weekly_minutes":120,"active":true,"languages":[],"formats":[],"self_assessments":[],"timezone":"Europe/Madrid","version":1,"created_at":"2026-11-23T10:00:00Z","updated_at":"2026-11-23T10:00:00Z","skills":[],"feasibility":"not_evaluated"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/v1/learner/goals/goal-id":
			if r.Header.Get("Idempotency-Key") != "goal-update" || r.Header.Get("X-Expected-Version") != "1" {
				t.Fatalf("wrong update headers: %v", r.Header)
			}
			_, _ = io.WriteString(w, `{"id":"goal-id","title":"Exam","target_topics":["topic-id"],"weekly_minutes":120,"active":true,"languages":[],"formats":[],"self_assessments":[],"timezone":"Europe/Madrid","version":2,"created_at":"2026-11-23T10:00:00Z","updated_at":"2026-11-23T10:01:00Z","skills":[],"feasibility":"not_evaluated"}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer upstream.Close()

	client := New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if templates, err := client.GoalTemplates(context.Background(), "jwt-user"); err != nil || len(templates) != 1 || !templates[0].SuggestsDeadline {
		t.Fatalf("templates = %+v, err = %v", templates, err)
	}
	profile, err := client.SaveLearnerProfile(context.Background(), AvailabilityInput{WeeklyMinutes: 180, Timezone: "Europe/Madrid"}, "profile-write", 4, "jwt-user")
	if err != nil || profile.Availability == nil || profile.Availability.Version != 5 {
		t.Fatalf("profile = %+v, err = %v", profile, err)
	}
	input := LearnerGoalInput{Title: "Exam", TargetTopics: []string{"topic-id"}, WeeklyMinutes: 120, Active: true, Languages: []string{}, Formats: []string{}, SelfAssessments: []SelfAssessment{}}
	goal, err := client.CreateLearnerGoal(context.Background(), input, "goal-write", "jwt-user")
	if err != nil || goal.Version != 1 {
		t.Fatalf("created goal = %+v, err = %v", goal, err)
	}
	goal, err = client.UpdateLearnerGoal(context.Background(), "goal-id", input, "goal-update", 1, "jwt-user")
	if err != nil || goal.Version != 2 {
		t.Fatalf("updated goal = %+v, err = %v", goal, err)
	}
	if requests != 4 {
		t.Fatalf("requests = %d, want 4", requests)
	}
}

func TestDiagnosticStartIsRepeatableForTheSameActor(t *testing.T) {
	t.Parallel()

	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPut || r.URL.Path != "/v1/learner/goals/goal-id/diagnostic" || r.Header.Get("X-User-ID") != "jwt-user" {
			t.Fatalf("unexpected request %s %s actor=%q", r.Method, r.URL.Path, r.Header.Get("X-User-ID"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"session-id","goal_id":"goal-id","goal_version":2,"status":"ready","baseline":"unknown","tasks":[],"estimated_minutes":10,"stop_rule":{"policy_version":"diagnostic-v1","max_tasks":4,"budget_minutes":10},"started_at":"2026-11-23T10:00:00Z","expires_at":"2026-11-24T10:00:00Z"}`)
	}))
	defer upstream.Close()

	client := New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for range 2 {
		session, err := client.StartDiagnostic(context.Background(), "goal-id", DiagnosticStartInput{GoalVersion: 2, BudgetMinutes: 10}, "jwt-user")
		if err != nil || session.ID != "session-id" || session.StopRule.PolicyVersion != "diagnostic-v1" {
			t.Fatalf("session = %+v, err = %v", session, err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

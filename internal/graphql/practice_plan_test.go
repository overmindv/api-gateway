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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func TestPracticePlanRejectsForgedActorWithoutJWT(t *testing.T) {
	handler := Handler(nil, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest("POST", "/graphql", strings.NewReader(`{"query":"{ practicePlan(topicIds: [\"00000000-0000-4000-8000-000000000001\"], minutes: 60) { totalMinutes } }"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "00000000-0000-4000-8000-000000000002")
	req.Header.Set("X-User-Roles", "admin")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if !bytes.Contains(response.Body.Bytes(), []byte(`"code":"UNAUTHENTICATED"`)) {
		t.Fatalf("expected authentication rejection, got %s", response.Body)
	}
}

const practiceTopic = "00000000-0000-4000-8000-000000000001"
const practiceBase = "00000000-0000-4000-8000-000000000002"
const practiceUser = "00000000-0000-4000-8000-000000000003"
const practiceTask = "00000000-0000-4000-8000-000000000004"
const practiceVersion = "00000000-0000-4000-8000-000000000005"

func practiceContext() context.Context {
	return middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: practiceUser, Token: "private-user-jwt", Roles: []string{"user"}})
}

type practiceFixture struct {
	input               recommendations.Request
	goal                *recommendations.Goal
	calls               int
	recommendationCalls int
	failPath            string
	inactive            bool
	missingTopic        bool
	cycle               bool
	manyTopics          bool
	taskCount           int
	historyCount        int
	observedTopics      map[string]bool
}

func (f *practiceFixture) resolver(t *testing.T) *queryResolver {
	t.Helper()
	f.observedTopics = map[string]bool{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls++
		if r.Header.Get("Authorization") == "Bearer private-user-jwt" {
			t.Error("user JWT forwarded")
		}
		if r.URL.Path == f.failPath {
			w.WriteHeader(503)
			_, _ = io.WriteString(w, `{"code":"FAILURE","message":"private database details"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		write := func(value any) {
			if err := json.NewEncoder(w).Encode(value); err != nil {
				t.Error(err)
			}
		}
		switch {
		case r.URL.Path == "/v1/practice-goal":
			if r.Header.Get("X-User-ID") != practiceUser || r.Header.Get("Authorization") != "Bearer service-token" {
				t.Error("unverified goal actor or token")
			}
			switch r.Method {
			case http.MethodGet:
				if f.goal == nil {
					w.WriteHeader(404)
					write(map[string]string{"code": "NOT_FOUND", "message": "Not found"})
				} else {
					write(f.goal)
				}
			case http.MethodPut:
				var input recommendations.GoalInput
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				revision := int64(1)
				if f.goal != nil {
					revision = f.goal.Revision + 1
				}
				f.goal = &recommendations.Goal{TopicIDs: input.TopicIDs, Minutes: input.Minutes, Revision: revision}
				write(f.goal)
			case http.MethodDelete:
				f.goal = nil
				w.WriteHeader(204)
			}
		case r.URL.Path == "/v1/practice-plan":
			f.recommendationCalls++
			if r.Header.Get("X-User-ID") != practiceUser || r.Header.Get("Authorization") != "Bearer service-token" {
				t.Error("unverified recommendation actor or token")
			}
			data, _ := io.ReadAll(r.Body)
			for _, secret := range []string{"secret-answer", "secret-source", "secret-output", "private-user-jwt", "other-user"} {
				if bytes.Contains(data, []byte(secret)) {
					t.Errorf("secret in recommendation payload: %s", secret)
				}
			}
			if err := json.Unmarshal(data, &f.input); err != nil {
				t.Fatal(err)
			}
			write(recommendations.Plan{PolicyVersion: "practice-v1", TotalMinutes: 10, Warnings: []string{}, Items: []recommendations.Item{{TaskID: practiceTask, TaskVersionID: practiceVersion, TopicID: practiceTopic, Title: "Public", Difficulty: "easy", TaskType: "single_choice", EstimatedMinutes: 10, ReasonCode: "START_TOPIC"}}})
		case strings.HasPrefix(r.URL.Path, "/v1/topics/"):
			id := strings.TrimPrefix(r.URL.Path, "/v1/topics/")
			if strings.HasSuffix(id, "/prerequisites") {
				id = strings.TrimSuffix(id, "/prerequisites")
				prereqs := []entities.TopicPrerequisite{}
				if f.manyTopics {
					for i := 10; i < 43; i++ {
						prereqs = append(prereqs, entities.TopicPrerequisite{TopicID: id, PrerequisiteTopicID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i)})
					}
				} else if id == practiceTopic {
					prereqs = append(prereqs, entities.TopicPrerequisite{TopicID: id, PrerequisiteTopicID: practiceBase})
				} else if f.cycle {
					prereqs = append(prereqs, entities.TopicPrerequisite{TopicID: id, PrerequisiteTopicID: practiceTopic})
				}
				write(prereqs)
			} else {
				if f.missingTopic {
					w.WriteHeader(404)
					write(map[string]string{"code": "TOPIC_NOT_FOUND", "message": "Not found"})
					return
				}
				status := "active"
				if f.inactive {
					status = "draft"
				}
				write(entities.Topic{ID: id, Title: "Topic", Status: status})
			}
		case r.URL.Path == "/v1/tasks":
			topic := r.URL.Query().Get("topic_id")
			f.observedTopics[topic] = true
			if topic != practiceTopic && topic != practiceBase {
				t.Error("unbounded/unselected catalog query")
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			if limit < 1 || limit > 100 {
				t.Error("unbounded page")
			}
			items := []tasks.TaskSummary{}
			if f.taskCount > 0 {
				for i := offset; i < offset+limit && i < f.taskCount; i++ {
					items = append(items, tasks.TaskSummary{ID: fmt.Sprintf("task-%s-%d", topic, i), TaskVersionID: "version", TopicID: &topic, Status: "published", TaskType: "single_choice", Difficulty: "easy", Title: "Public"})
				}
			} else if offset == 0 && topic == practiceTopic {
				items = append(items, tasks.TaskSummary{ID: practiceTask, TaskVersionID: practiceVersion, TopicID: &topic, Status: "published", TaskType: "single_choice", Difficulty: "easy", Title: "Public"}, tasks.TaskSummary{ID: "draft", TaskVersionID: "version", TopicID: &topic, Status: "draft"})
			}
			write(tasks.TaskList{Items: items, Limit: limit, Offset: offset})
		case r.URL.Path == "/v1/me/submissions":
			if r.Header.Get("X-User-ID") != practiceUser || r.URL.Query().Get("task_id") != "" {
				t.Error("history must belong to verified user across tasks")
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			items := []tasks.Submission{}
			if f.historyCount > 0 {
				for i := offset; i < offset+limit && i < f.historyCount; i++ {
					items = append(items, tasks.Submission{UserID: practiceUser, TaskID: fmt.Sprintf("choice-%d", i), TaskVersionID: "version", Correct: true, Verdict: "accepted", CreatedAt: "2026-11-08T12:00:00Z"})
				}
			} else if offset == 0 {
				items = []tasks.Submission{
					{UserID: practiceUser, TaskID: practiceTask, TaskVersionID: practiceVersion, Correct: false, Verdict: "wrong_answer", CreatedAt: "2026-11-08T12:00:00Z", CorrectOptionIDs: []string{"secret-answer"}},
					{UserID: practiceUser, TaskID: practiceTask, TaskVersionID: practiceVersion, Correct: true, Verdict: "accepted", CreatedAt: "2026-11-07T12:00:00Z"},
					{UserID: "other-user", TaskID: "foreign", TaskVersionID: "version", Correct: true, Verdict: "accepted", CreatedAt: "2026-11-08T12:00:00Z"},
				}
			}
			write(tasks.SubmissionList{Items: items, Limit: limit, Offset: offset})
		case r.URL.Path == "/v1/me/code-submissions":
			if r.Header.Get("X-User-ID") != practiceUser || r.URL.Query().Get("task_id") != "" {
				t.Error("code history must belong to verified user across tasks")
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			items := []tasks.CodeSubmission{}
			if f.historyCount > 0 {
				for i := offset; i < offset+limit && i < f.historyCount; i++ {
					verdict := "accepted"
					items = append(items, tasks.CodeSubmission{UserID: practiceUser, TaskID: fmt.Sprintf("code-%d", i), TaskVersionID: "version", Status: "completed", Verdict: &verdict, CreatedAt: "2026-11-08T12:00:00Z"})
				}
			} else if offset == 0 {
				for i, verdict := range []string{"accepted", "wrong_answer", "infrastructure_error", "checker_error", "cancelled"} {
					v := verdict
					items = append(items, tasks.CodeSubmission{UserID: practiceUser, TaskID: fmt.Sprintf("code-%d", i), TaskVersionID: "version", Status: "completed", Verdict: &v, CreatedAt: "2026-11-08T12:00:00Z", SourceFileName: "secret-source", Execution: &tasks.ExecutionPhaseResult{Stdout: "secret-output"}})
				}
				v := "accepted"
				items = append(items, tasks.CodeSubmission{UserID: practiceUser, TaskID: "queued", TaskVersionID: "version", Status: "queued", Verdict: &v, CreatedAt: "2026-11-08T12:00:00Z"})
			}
			write(tasks.CodeSubmissionList{Items: items, Limit: limit, Offset: offset})
		default:
			t.Errorf("unexpected upstream %s", r.URL)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(upstream.Close)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &queryResolver{&Resolver{Catalog: entities.New(upstream.URL, time.Second, log), Tasks: tasks.New(upstream.URL, time.Second, log), Recommendations: recommendations.New(upstream.URL, "service-token", time.Second, log)}}
}

func TestSavedPracticeGoalLifecycle(t *testing.T) {
	f := &practiceFixture{}
	r := f.resolver(t)
	ctx := practiceContext()
	if got, err := r.MyPracticeGoal(ctx); err != nil || got != nil {
		t.Fatalf("missing goal: %+v %v", got, err)
	}
	mutation := &mutationResolver{r.Resolver}
	for want := 1; want <= 2; want++ {
		goal, err := mutation.SavePracticeGoal(ctx, []string{practiceTopic}, 60)
		if err != nil || goal.Revision != want || goal.Minutes != 60 || len(goal.Topics) != 1 || goal.Topics[0].Title != "Topic" {
			t.Fatalf("save goal: %+v %v", goal, err)
		}
	}
	if got, err := r.MyPracticeGoal(ctx); err != nil || got == nil || got.Revision != 2 {
		t.Fatalf("reload goal: %+v %v", got, err)
	}
	for i := 0; i < 2; i++ {
		if ok, err := mutation.DeletePracticeGoal(ctx); err != nil || !ok {
			t.Fatalf("delete goal: %t %v", ok, err)
		}
	}
	if got, err := r.MyPracticeGoal(ctx); err != nil || got != nil {
		t.Fatalf("deleted goal: %+v %v", got, err)
	}
}

func TestSavedPracticeGoalValidatesBeforeStorage(t *testing.T) {
	f := &practiceFixture{}
	r := f.resolver(t)
	mutation := &mutationResolver{r.Resolver}
	_, err := mutation.SavePracticeGoal(practiceContext(), []string{practiceTopic, practiceTopic}, 60)
	code, _ := errorCodeAndMessage(err)
	if code != "INVALID_ARGUMENT" || f.calls != 0 {
		t.Fatalf("invalid goal reached upstream: %s calls=%d", code, f.calls)
	}
}

func TestSavedPracticeGoalShowsDeletedTopicForRepair(t *testing.T) {
	f := &practiceFixture{missingTopic: true, goal: &recommendations.Goal{TopicIDs: []string{practiceTopic}, Minutes: 60, Revision: 1}}
	r := f.resolver(t)
	goal, err := r.MyPracticeGoal(practiceContext())
	if err != nil || goal == nil || len(goal.Topics) != 1 || goal.Topics[0].Status != "archived" {
		t.Fatalf("deleted topic cannot be removed from goal: %+v %v", goal, err)
	}
}

func TestPracticePlanCollectsSafePublishedMetadataAndOwnFinalEvidence(t *testing.T) {
	f := &practiceFixture{}
	r := f.resolver(t)
	plan, err := r.PracticePlan(practiceContext(), []string{practiceTopic}, 60)
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalMinutes != 10 || len(plan.Items) != 1 || plan.Items[0].TaskVersionID != practiceVersion {
		t.Fatalf("wrong plan: %+v", plan)
	}
	if len(f.input.Topics) != 2 || len(f.input.Topics[0].PrerequisiteIDs) != 1 || len(f.input.Tasks) != 1 {
		t.Fatalf("wrong metadata %+v", f.input)
	}
	if len(f.input.Evidence) != 3 {
		t.Fatalf("expected accepted choice, accepted code, wrong answer code; got %+v", f.input.Evidence)
	}
	for _, e := range f.input.Evidence {
		if e.TaskID == practiceTask && e.Verdict != "accepted" {
			t.Fatal("accepted attempt lost during deduplication")
		}
	}
	if f.input.InputTruncated {
		t.Fatal("unexpected truncation")
	}
}

func TestPracticePlanValidatesBeforeOutboundCalls(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ids     []string
		minutes int
	}{
		{"no topics", nil, 60}, {"invalid ID", []string{"bad"}, 60}, {"nil UUID", []string{"00000000-0000-0000-0000-000000000000"}, 60}, {"duplicate", []string{practiceTopic, practiceTopic}, 60}, {"too many", []string{practiceTopic, practiceBase, practiceUser, practiceTask, practiceVersion, practiceTopic}, 60}, {"short", []string{practiceTopic}, 9}, {"long", []string{practiceTopic}, 241},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &practiceFixture{}
			r := f.resolver(t)
			_, err := r.PracticePlan(practiceContext(), tc.ids, tc.minutes)
			code, _ := errorCodeAndMessage(err)
			if code != "INVALID_ARGUMENT" || f.calls != 0 {
				t.Fatalf("code=%s calls=%d", code, f.calls)
			}
		})
	}
}

func TestPracticePlanDependencyFailureDoesNotBecomeColdStart(t *testing.T) {
	for _, path := range []string{"/v1/topics/" + practiceTopic, "/v1/topics/" + practiceTopic + "/prerequisites", "/v1/tasks", "/v1/me/submissions", "/v1/me/code-submissions", "/v1/practice-plan"} {
		t.Run(path, func(t *testing.T) {
			f := &practiceFixture{failPath: path}
			r := f.resolver(t)
			plan, err := r.PracticePlan(practiceContext(), []string{practiceTopic}, 60)
			code, message := errorCodeAndMessage(err)
			if code != "SERVICE_UNAVAILABLE" || plan != nil || strings.Contains(message, "private") {
				t.Fatalf("unsafe dependency result: plan=%+v code=%s message=%s", plan, code, message)
			}
			if path != "/v1/practice-plan" && f.recommendationCalls != 0 {
				t.Fatal("dependency failure used as empty history")
			}
		})
	}
}

func TestPracticePlanRejectsInactiveCyclicOrIncompleteGraph(t *testing.T) {
	for _, f := range []*practiceFixture{{inactive: true}, {cycle: true}, {manyTopics: true}} {
		r := f.resolver(t)
		_, err := r.PracticePlan(practiceContext(), []string{practiceTopic}, 60)
		code, _ := errorCodeAndMessage(err)
		if code != "INVALID_ARGUMENT" || f.recommendationCalls != 0 {
			t.Fatalf("unsafe graph forwarded: code=%s calls=%d", code, f.recommendationCalls)
		}
	}
}

func TestPracticePlanPaginatesAllTopicsAndCapsEachHistory(t *testing.T) {
	f := &practiceFixture{taskCount: 1200, historyCount: 1001}
	r := f.resolver(t)
	plan, err := r.PracticePlan(practiceContext(), []string{practiceTopic, practiceBase}, 240)
	if err != nil {
		t.Fatal(err)
	}
	if !f.input.InputTruncated || len(f.input.Tasks) != 2000 || len(f.input.Evidence) != 2000 {
		t.Fatalf("bounds/truncation wrong: tasks=%d evidence=%d truncated=%t", len(f.input.Tasks), len(f.input.Evidence), f.input.InputTruncated)
	}
	if !f.observedTopics[practiceTopic] || !f.observedTopics[practiceBase] {
		t.Fatal("a requested topic was omitted")
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0] != "PARTIAL_DATA" {
		t.Fatalf("missing partial warning: %+v", plan.Warnings)
	}
}

func TestPracticePlanExactHistoryBoundIsNotTruncated(t *testing.T) {
	f := &practiceFixture{historyCount: 1000}
	r := f.resolver(t)
	_, err := r.PracticePlan(practiceContext(), []string{practiceTopic}, 60)
	if err != nil {
		t.Fatal(err)
	}
	if f.input.InputTruncated {
		t.Fatal("full last page without extra records is complete")
	}
}

func TestPracticePlanDisabled(t *testing.T) {
	r := &queryResolver{&Resolver{}}
	_, err := r.PracticePlan(practiceContext(), []string{practiceTopic}, 60)
	code, _ := errorCodeAndMessage(err)
	if code != "SERVICE_UNAVAILABLE" {
		t.Fatalf("got %s", code)
	}
}

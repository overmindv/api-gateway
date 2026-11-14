package graphql

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestGoalPlanSelectsTopicsThenLoadsCurrentPractice(t *testing.T) {
	f := &practiceFixture{}
	r := f.resolver(t)
	plan, err := r.LearningPlan(practiceContext(), "Junior AI", 60, 3)
	if err != nil || plan.AiStatus != "GENERATED" || len(plan.Materials) != 1 || len(plan.Topics) != 1 || plan.Practice == nil {
		t.Fatalf("bad goal plan: %+v %v", plan, err)
	}
	if f.input.Goal != "" || len(f.input.TopicIDs) != 1 || f.input.TopicIDs[0] != practiceTopic {
		t.Fatal("second inference or wrong topics")
	}
}
func TestGoalPlanRequiresAuthAndBoundedGoal(t *testing.T) {
	f := &practiceFixture{}
	r := f.resolver(t)
	if _, err := r.LearningPlan(context.Background(), "Junior AI", 60, 3); err == nil {
		t.Fatal("auth bypass")
	}
	for _, goal := range []string{"", strings.Repeat("я", 1001)} {
		if _, err := r.LearningPlan(practiceContext(), goal, 60, 3); err == nil {
			t.Fatal("invalid goal")
		}
	}
	if f.calls != 0 {
		t.Fatal("invalid request reached upstream")
	}
}
func TestGoalPlanKeepsMaterialsOnPracticeFailure(t *testing.T) {
	f := &practiceFixture{failPath: "/v1/tasks"}
	r := f.resolver(t)
	plan, err := r.LearningPlan(practiceContext(), "Junior AI", 60, 3)
	if err != nil || len(plan.Materials) != 1 || plan.Practice != nil || len(plan.Warnings) != 1 || plan.Warnings[0] != "PRACTICE_UNAVAILABLE" {
		t.Fatalf("bad degradation %+v %v", plan, err)
	}
}

func TestGoalPlanGraphQLContractAndFanout(t *testing.T) {
	f := &practiceFixture{}
	r := f.resolver(t)
	handler := HandlerWithRecommendationsAndMetrics(nil, r.Catalog, r.Tasks, nil, nil, nil, r.Recommendations, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	field := `learningPlan(goal:"Junior AI",minutes:60,sessionCount:3){aiStatus topics{id title} materials{id url action minutes} practice{totalMinutes} warnings}`
	body := practiceOperationRequest(t, handler, `{`+field+`}`, "", nil)
	if strings.Contains(body, `"errors"`) || !strings.Contains(body, `"GENERATED"`) || !strings.Contains(body, `docs.python.org`) {
		t.Fatal(body)
	}
	calls := f.calls
	body = practiceOperationRequest(t, handler, `{a:`+field+` b:`+field+`}`, "", nil)
	if !strings.Contains(body, "OPERATION_LIMIT_EXCEEDED") || calls != f.calls {
		t.Fatal("fanout", body)
	}
}

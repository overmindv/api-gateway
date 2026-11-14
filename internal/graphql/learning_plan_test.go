package graphql

import (
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestLearningPlanGraphQLSessionContract(t *testing.T) {
	fixture := &practiceFixture{}
	r := fixture.resolver(t)
	handler := HandlerWithRecommendationsAndMetrics(nil, r.Catalog, r.Tasks, nil, nil, nil, r.Recommendations, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	body := practiceOperationRequest(t, handler, `{ practicePlan(topicIds:["`+practiceTopic+`"],minutes:30,sessionCount:3) { requestedSessions totalPlannedMinutes sessions { number totalMinutes items { taskId } } topicInsights { topicId title isTarget acceptedTasks failedTasks eligibleTasks plannedTasks remainingToReadiness recommendedDifficulty minimumMinutes nextAction blockedBy { id title } } } }`, "", nil)
	if strings.Contains(body, `"errors"`) || !strings.Contains(body, `"requestedSessions":3`) || !strings.Contains(body, `"title":"Topic"`) || !strings.Contains(body, `"totalPlannedMinutes":10`) {
		t.Fatalf("roadmap GraphQL mapping failed: %s", body)
	}
}

func TestLearningPlanRejectsSessionRangeBeforeDependencies(t *testing.T) {
	for _, count := range []string{"0", "9", "-1"} {
		fixture := &practiceFixture{}
		r := fixture.resolver(t)
		handler := HandlerWithRecommendationsAndMetrics(nil, r.Catalog, r.Tasks, nil, nil, nil, r.Recommendations, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
		body := practiceOperationRequest(t, handler, `{ practicePlan(topicIds:["`+practiceTopic+`"],minutes:30,sessionCount:`+count+`) { totalMinutes } }`, "", nil)
		if !strings.Contains(body, `"code":"INVALID_ARGUMENT"`) || fixture.calls != 0 {
			t.Fatalf("invalid horizon reached dependencies: %s", body)
		}
	}
}

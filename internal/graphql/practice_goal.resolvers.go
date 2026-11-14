package graphql

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func (r *queryResolver) MyPracticeGoal(ctx context.Context) (*model.PracticeGoal, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	service, ok := r.Recommendations.(recommendations.GoalService)
	if !ok || r.Catalog == nil {
		return nil, practiceUnavailable()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	goal, err := service.GetGoal(ctx, auth.UserID)
	if err != nil || goal == nil {
		return nil, err
	}
	return r.practiceGoalModel(ctx, *goal)
}

func (r *mutationResolver) SavePracticeGoal(ctx context.Context, topicIds []string, minutes int, sessionCount *int) (*model.PracticeGoal, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	service, ok := r.Recommendations.(recommendations.GoalService)
	if !ok || r.Catalog == nil {
		return nil, practiceUnavailable()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if sessionCount != nil && (*sessionCount < 1 || *sessionCount > 8) {
		return nil, practiceInvalid()
	}
	if len(topicIds) < 1 || len(topicIds) > 5 || minutes < 10 || minutes > 240 {
		return nil, practiceInvalid()
	}
	ids := make([]string, 0, len(topicIds))
	seen := make(map[string]bool, len(topicIds))
	for _, raw := range topicIds {
		parsed, err := uuid.Parse(raw)
		if err != nil || parsed == uuid.Nil || seen[parsed.String()] {
			return nil, practiceInvalid()
		}
		id := parsed.String()
		seen[id] = true
		ids = append(ids, id)
	}
	if _, err := r.practiceTopics(ctx, ids); err != nil {
		return nil, err
	}
	goal, err := service.SaveGoal(ctx, recommendations.GoalInput{TopicIDs: ids, Minutes: minutes, SessionCount: practiceSessionCount(sessionCount)}, auth.UserID)
	if err != nil {
		return nil, err
	}
	return r.practiceGoalModel(ctx, goal)
}

func (r *mutationResolver) DeletePracticeGoal(ctx context.Context) (bool, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return false, err
	}
	service, ok := r.Recommendations.(recommendations.GoalService)
	if !ok {
		return false, practiceUnavailable()
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := service.DeleteGoal(ctx, auth.UserID); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Resolver) practiceGoalModel(ctx context.Context, goal recommendations.Goal) (*model.PracticeGoal, error) {
	topics := make([]*model.PracticeGoalTopic, 0, len(goal.TopicIDs))
	for _, id := range goal.TopicIDs {
		topic, err := r.Catalog.GetTopic(ctx, id)
		if err != nil {
			var catalogError *entities.Error
			if errors.As(err, &catalogError) && catalogError.Code == "TOPIC_NOT_FOUND" {
				topics = append(topics, &model.PracticeGoalTopic{ID: id, Title: "Тема недоступна", Status: "archived"})
				continue
			}
			return nil, practiceUnavailable()
		}
		topics = append(topics, &model.PracticeGoalTopic{ID: topic.ID, Title: topic.Title, Status: topic.Status})
	}
	return &model.PracticeGoal{Topics: topics, Minutes: goal.Minutes, SessionCount: max(1, goal.SessionCount), Revision: int(goal.Revision)}, nil
}

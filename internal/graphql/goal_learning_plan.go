package graphql

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// LearningPlan сначала выбирает темы, затем получает практику через сервисы-владельцы.
func (r *queryResolver) LearningPlan(ctx context.Context, goal string, minutes, sessionCount int) (*model.LearningPlan, error) {
	return r.learningPlan(ctx, goal, minutes, sessionCount, nil)
}
func (r *queryResolver) learningPlan(ctx context.Context, goal string, minutes, sessionCount int, completed []string) (*model.LearningPlan, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	goal = strings.TrimSpace(goal)
	if goal == "" || !utf8.ValidString(goal) || utf8.RuneCountInString(goal) > 1000 || minutes < 10 || minutes > 240 || sessionCount < 1 || sessionCount > 8 {
		return nil, practiceInvalid()
	}
	service, ok := r.Recommendations.(recommendations.OutlineService)
	if !ok || r.Catalog == nil {
		return nil, practiceUnavailable()
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	topics := []recommendations.Topic{}
	byID := map[string]string{}
	truncated := false
	for offset := 0; offset <= 200; offset += 100 {
		limit := 100
		if offset == 200 {
			limit = 1
		}
		page, err := r.Catalog.ListTopics(ctx, "", entities.ListOptions{Status: "active", Limit: limit, Offset: offset})
		if err != nil || len(page) > limit {
			return nil, practiceUnavailable()
		}
		if offset == 200 {
			truncated = len(page) > 0
			break
		}
		for _, t := range page {
			if t.Status == "active" && byID[t.ID] == "" {
				topics = append(topics, recommendations.Topic{ID: t.ID, Title: t.Title})
				byID[t.ID] = t.Title
			}
		}
		if len(page) < limit {
			break
		}
	}
	sort.Slice(topics, func(i, j int) bool { return topics[i].ID < topics[j].ID })
	outline, err := service.LearningOutline(ctx, recommendations.OutlineRequest{Goal: goal, Topics: topics, CompletedMaterialIDs: completed}, auth.UserID)
	if err != nil {
		return nil, err
	}
	result := &model.LearningPlan{AiStatus: outline.AIStatus, Topics: []*model.PracticeTopicRef{}, Materials: []*model.LearningMaterial{}, Warnings: []string{}}
	if truncated {
		result.Warnings = append(result.Warnings, "PARTIAL_CATALOG")
	}
	seen := map[string]bool{}
	if len(outline.TopicIDs) > 5 || len(outline.Materials) > 8 {
		return nil, practiceUnavailable()
	}
	for _, id := range outline.TopicIDs {
		if byID[id] == "" || seen[id] {
			return nil, practiceUnavailable()
		}
		seen[id] = true
		result.Topics = append(result.Topics, &model.PracticeTopicRef{ID: id, Title: byID[id]})
	}
	for _, m := range outline.Materials {
		result.Materials = append(result.Materials, &model.LearningMaterial{ID: m.ID, Title: m.Title, URL: m.URL, Action: m.Action, Minutes: m.Minutes})
	}
	if len(outline.TopicIDs) > 0 {
		practice, err := r.PracticePlan(ctx, outline.TopicIDs, minutes, &sessionCount, nil)
		if err != nil {
			result.Warnings = append(result.Warnings, "PRACTICE_UNAVAILABLE")
		} else {
			result.Practice = practice
			if len(practice.Items) == 0 {
				result.Warnings = append(result.Warnings, "NO_PRACTICE")
			}
		}
	} else {
		result.Warnings = append(result.Warnings, "NO_PRACTICE_TOPICS")
	}
	return result, nil
}

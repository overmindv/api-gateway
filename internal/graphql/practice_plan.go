package graphql

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

const (
	practiceTopicLimit   = 32
	practiceTaskLimit    = 2000
	practiceHistoryLimit = 1000
	practicePageSize     = 100
)

// PracticePlan собирает разрешённые metadata и собственную историю проверенного actor.
func (r *queryResolver) PracticePlan(ctx context.Context, topicIDs []string, minutes int) (*model.PracticePlan, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if len(topicIDs) < 1 || len(topicIDs) > 5 || minutes < 10 || minutes > 240 {
		return nil, practiceInvalid()
	}
	ids := make([]string, 0, len(topicIDs))
	seen := make(map[string]bool, len(topicIDs))
	for _, value := range topicIDs {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || seen[id.String()] {
			return nil, practiceInvalid()
		}
		seen[id.String()] = true
		ids = append(ids, id.String())
	}
	if r.Recommendations == nil || r.Catalog == nil || r.Tasks == nil {
		return nil, practiceUnavailable()
	}
	// Общий дедлайн ограничивает последовательную пагинацию всех зависимостей.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	topics, err := r.practiceTopics(ctx, ids)
	if err != nil {
		return nil, err
	}
	candidates, truncated, err := r.practiceCandidates(ctx, topics)
	if err != nil {
		return nil, err
	}
	evidence, historyTruncated, err := r.practiceEvidence(ctx, tasks.Actor{UserID: auth.UserID, Roles: auth.Roles})
	if err != nil {
		return nil, err
	}
	input := recommendations.Request{TopicIDs: ids, Minutes: minutes, Topics: topics, Tasks: candidates, Evidence: evidence, InputTruncated: truncated || historyTruncated}
	plan, err := r.Recommendations.PracticePlan(ctx, input, auth.UserID)
	if err != nil {
		return nil, err
	}
	warnings := append([]string{}, plan.Warnings...)
	if input.InputTruncated {
		found := false
		for _, warning := range warnings {
			if warning == "PARTIAL_DATA" {
				found = true
			}
		}
		if !found {
			warnings = append(warnings, "PARTIAL_DATA")
		}
	}
	items := make([]*model.PracticePlanItem, 0, len(plan.Items))
	for _, item := range plan.Items {
		items = append(items, &model.PracticePlanItem{TaskID: item.TaskID, TaskVersionID: item.TaskVersionID, TopicID: item.TopicID, Title: item.Title, Difficulty: item.Difficulty, TaskType: item.TaskType, EstimatedMinutes: item.EstimatedMinutes, ReasonCode: item.ReasonCode})
	}
	return &model.PracticePlan{PolicyVersion: plan.PolicyVersion, TotalMinutes: plan.TotalMinutes, Warnings: warnings, Items: items}, nil
}

func practiceInvalid() error     { return &recommendations.Error{Code: "INVALID_ARGUMENT"} }
func practiceUnavailable() error { return &recommendations.Error{Code: "SERVICE_UNAVAILABLE"} }

// practiceTopics получает полный ограниченный граф: обрезать prerequisites небезопасно.
func (r *queryResolver) practiceTopics(ctx context.Context, selected []string) ([]recommendations.Topic, error) {
	queue := append([]string{}, selected...)
	seen := make(map[string]bool, len(queue))
	for _, id := range queue {
		seen[id] = true
	}
	topics := make([]recommendations.Topic, 0, len(queue))
	graph := map[string][]string{}
	for i := 0; i < len(queue); i++ {
		id := queue[i]
		topic, err := r.Catalog.GetTopic(ctx, id)
		if err != nil {
			return nil, practiceUnavailable()
		}
		if topic.ID != id || topic.Status != "active" {
			return nil, practiceInvalid()
		}
		links, err := r.Catalog.ListPrerequisites(ctx, id)
		if err != nil {
			return nil, practiceUnavailable()
		}
		prerequisites := make([]string, 0, len(links))
		linked := map[string]bool{}
		for _, link := range links {
			dependency, err := uuid.Parse(link.PrerequisiteTopicID)
			if err != nil || dependency == uuid.Nil || link.TopicID != id {
				return nil, practiceInvalid()
			}
			dependencyID := dependency.String()
			if linked[dependencyID] {
				continue
			}
			linked[dependencyID] = true
			prerequisites = append(prerequisites, dependencyID)
			if !seen[dependencyID] {
				if len(queue) >= practiceTopicLimit {
					return nil, practiceInvalid()
				}
				seen[dependencyID] = true
				queue = append(queue, dependencyID)
			}
		}
		sort.Strings(prerequisites)
		graph[id] = prerequisites
		topics = append(topics, recommendations.Topic{ID: id, PrerequisiteIDs: prerequisites})
	}
	states := map[string]int{}
	var visit func(string) bool
	visit = func(id string) bool {
		if states[id] == 1 {
			return false
		}
		if states[id] == 2 {
			return true
		}
		states[id] = 1
		for _, dependency := range graph[id] {
			if !visit(dependency) {
				return false
			}
		}
		states[id] = 2
		return true
	}
	for _, id := range selected {
		if !visit(id) {
			return nil, practiceInvalid()
		}
	}
	return topics, nil
}

// practiceCandidates обходит темы по кругу, чтобы большая первая тема не вытеснила остальные.
func (r *queryResolver) practiceCandidates(ctx context.Context, topics []recommendations.Topic) ([]recommendations.Task, bool, error) {
	type page struct {
		id     string
		offset int
		done   bool
	}
	pages := make([]page, len(topics))
	for i, topic := range topics {
		pages[i].id = topic.ID
	}
	pageSize := min(practicePageSize, practiceTaskLimit/len(topics))
	candidates := make([]recommendations.Task, 0)
	seen := map[string]bool{}
	scanned := 0
	for scanned < practiceTaskLimit {
		active := false
		for i := range pages {
			p := &pages[i]
			if p.done {
				continue
			}
			active = true
			limit := min(pageSize, practiceTaskLimit-scanned)
			if limit == 0 {
				break
			}
			list, err := r.Tasks.ListPublished(ctx, tasks.TaskFilter{TopicID: &p.id, Limit: limit, Offset: p.offset})
			if err != nil || len(list.Items) > limit {
				return nil, false, practiceUnavailable()
			}
			p.offset += len(list.Items)
			scanned += len(list.Items)
			p.done = len(list.Items) < limit
			for _, task := range list.Items {
				if task.Status != "published" || task.TopicID == nil || *task.TopicID != p.id || seen[task.ID] {
					continue
				}
				seen[task.ID] = true
				candidates = append(candidates, recommendations.Task{ID: task.ID, TaskVersionID: task.TaskVersionID, TopicID: *task.TopicID, Title: task.Title, Difficulty: task.Difficulty, TaskType: task.TaskType})
			}
		}
		if !active {
			break
		}
	}
	// Последняя полная страница не доказывает усечение: проверяем ещё одну запись.
	for _, p := range pages {
		if p.done {
			continue
		}
		list, err := r.Tasks.ListPublished(ctx, tasks.TaskFilter{TopicID: &p.id, Limit: 1, Offset: p.offset})
		if err != nil || len(list.Items) > 1 {
			return nil, false, practiceUnavailable()
		}
		if len(list.Items) > 0 {
			return candidates, true, nil
		}
	}
	return candidates, false, nil
}

// practiceEvidence ограничивает каждую историю до дедупликации и не переносит приватные поля.
func (r *queryResolver) practiceEvidence(ctx context.Context, actor tasks.Actor) ([]recommendations.Evidence, bool, error) {
	evidence := map[string]recommendations.Evidence{}
	add := func(item recommendations.Evidence) {
		key := item.TaskID + "/" + item.TaskVersionID
		previous, ok := evidence[key]
		if ok && previous.Verdict == "accepted" && item.Verdict != "accepted" {
			return
		}
		if ok && !(item.Verdict == "accepted" && previous.Verdict != "accepted") {
			previousTime, _ := time.Parse(time.RFC3339Nano, previous.OccurredAt)
			itemTime, _ := time.Parse(time.RFC3339Nano, item.OccurredAt)
			if !itemTime.After(previousTime) {
				return
			}
		}
		evidence[key] = item
	}
	choiceTruncated, err := practiceHistory(func(limit, offset int) (int, error) {
		list, err := r.Tasks.ListMySubmissions(ctx, nil, limit, offset, actor)
		if err != nil {
			return 0, err
		}
		if offset < practiceHistoryLimit {
			for _, item := range list.Items {
				if item.UserID != actor.UserID {
					continue
				}
				verdict := "wrong_answer"
				if item.Correct {
					verdict = "accepted"
				}
				add(recommendations.Evidence{TaskID: item.TaskID, TaskVersionID: item.TaskVersionID, Verdict: verdict, OccurredAt: item.CreatedAt})
			}
		}
		return len(list.Items), nil
	})
	if err != nil {
		return nil, false, err
	}
	codeTruncated, err := practiceHistory(func(limit, offset int) (int, error) {
		list, err := r.Tasks.ListMyCodeSubmissions(ctx, nil, limit, offset, actor)
		if err != nil {
			return 0, err
		}
		if offset < practiceHistoryLimit {
			for _, item := range list.Items {
				if item.UserID != actor.UserID || item.Status != "completed" || item.Verdict == nil {
					continue
				}
				switch *item.Verdict {
				case "accepted", "wrong_answer", "compilation_error", "runtime_error", "time_limit_exceeded", "memory_limit_exceeded", "output_limit_exceeded":
					occurredAt := item.CreatedAt
					if item.CompletedAt != nil {
						occurredAt = *item.CompletedAt
					}
					add(recommendations.Evidence{TaskID: item.TaskID, TaskVersionID: item.TaskVersionID, Verdict: *item.Verdict, OccurredAt: occurredAt})
				}
			}
		}
		return len(list.Items), nil
	})
	if err != nil {
		return nil, false, err
	}
	keys := make([]string, 0, len(evidence))
	for key := range evidence {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]recommendations.Evidence, 0, len(keys))
	for _, key := range keys {
		result = append(result, evidence[key])
	}
	return result, choiceTruncated || codeTruncated, nil
}

func practiceHistory(fetch func(limit, offset int) (int, error)) (bool, error) {
	offset := 0
	for offset < practiceHistoryLimit {
		limit := min(practicePageSize, practiceHistoryLimit-offset)
		count, err := fetch(limit, offset)
		if err != nil || count > limit {
			return false, practiceUnavailable()
		}
		offset += count
		if count < limit {
			return false, nil
		}
	}
	count, err := fetch(1, offset)
	if err != nil || count > 1 {
		return false, practiceUnavailable()
	}
	return count > 0, nil
}

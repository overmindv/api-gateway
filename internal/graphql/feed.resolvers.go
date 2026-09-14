package graphql

import (
	"context"

	"github.com/overmindv/api-gateway/internal/graphql/model"
)

// Feed возвращает публичную ленту активностей (каталог + задачи) по убыванию времени.
func (r *queryResolver) Feed(ctx context.Context, pagination *model.PaginationInput) (*model.FeedConnection, error) {
	limit, offset := 50, 0
	if pagination != nil {
		limit = intValue(pagination.Limit, 50)
		offset = intValue(pagination.Offset, 0)
	}
	result, err := r.Resolver.Feed.ListFeed(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	items := make([]*model.FeedItem, 0, len(result.Items))
	for _, item := range result.Items {
		kind, ok := feedKind(item.Kind)
		if !ok {
			// Неизвестный тип события (например, выпущенный раньше версии schema)
			// пропускаем, чтобы лента не ломалась на обратной совместимости.
			continue
		}
		items = append(items, &model.FeedItem{
			ID:          item.ID,
			Kind:        kind,
			Title:       item.Title,
			Text:        item.Text,
			Href:        item.Href,
			ActorUserID: item.ActorUserID,
			OccurredAt:  item.OccurredAt,
		})
	}

	return &model.FeedConnection{Items: items, Limit: result.Limit, Offset: result.Offset}, nil
}

// feedKind маппит строковый kind из feed-сервиса в GraphQL enum.
func feedKind(kind string) (model.FeedKind, bool) {
	switch kind {
	case "university.created":
		return model.FeedKindUniversityCreated, true
	case "university.activated":
		return model.FeedKindUniversityActivated, true
	case "program.created":
		return model.FeedKindProgramCreated, true
	case "program.activated":
		return model.FeedKindProgramActivated, true
	case "course.created":
		return model.FeedKindCourseCreated, true
	case "course.activated":
		return model.FeedKindCourseActivated, true
	case "topic.created":
		return model.FeedKindTopicCreated, true
	case "topic.activated":
		return model.FeedKindTopicActivated, true
	case "task.published":
		return model.FeedKindTaskPublished, true
	default:
		return "", false
	}
}

package graphql

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/graphql/model"
)

func (r *queryResolver) ExternalTaskSearch(ctx context.Context, query string) ([]*model.ExternalTaskResult, error) {
	actor, err := collectionActor(ctx)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 100 {
		return nil, practiceInvalid()
	}
	service, ok := r.TaskHunter.(taskhunter.SearchService)
	if !ok {
		return nil, practiceUnavailable()
	}
	items, err := service.Search(ctx, query, actor)
	if err != nil {
		return nil, err
	}
	result := []*model.ExternalTaskResult{}
	for _, item := range items {
		result = append(result, &model.ExternalTaskResult{Title: item.Title, URL: item.URL, Rating: item.Rating, Tags: item.Tags})
	}
	return result, nil
}

package graphql

import (
	"context"
	"sync"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/graphql/model"
)

// optionsForQuery собирает параметры поиска для каталога: только активные элементы.
func optionsForQuery(query string, limit int) entities.ListOptions {
	return entities.ListOptions{Search: query, Status: "active", Limit: limit}
}

// Search — единый поиск по каталогу и опубликованным задачам. Результаты собраны по видам.
// Каждый вид опрашивается параллельно; сбой одного источника не скрывает остальные результаты.
func (r *queryResolver) Search(ctx context.Context, query string, limit int) (*model.SearchResults, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	if query == "" {
		return &model.SearchResults{
			Universities: []*model.University{},
			Programs:     []*model.Program{},
			Courses:      []*model.Course{},
			Topics:       []*model.Topic{},
			Tasks:        []*model.ITTaskSummary{},
		}, nil
	}

	result := &model.SearchResults{}
	var wg sync.WaitGroup
	wg.Add(5)

	groupErr := func() {
		defer wg.Done()
		items, err := r.Catalog.ListUniversities(ctx, optionsForQuery(query, limit))
		if err != nil {
			r.Log.With("kind", "universities", "err", err).Error("search failed for kind")
			return
		}
		result.Universities = collectMap(items, universityModel)
	}

	go groupErr()
	go func() {
		defer wg.Done()
		items, err := r.Catalog.ListPrograms(ctx, "", optionsForQuery(query, limit))
		if err != nil {
			r.Log.With("kind", "programs", "err", err).Error("search failed for kind")
			return
		}
		result.Programs = collectMap(items, programModel)
	}()
	go func() {
		defer wg.Done()
		items, err := r.Catalog.ListCourses(ctx, "", optionsForQuery(query, limit))
		if err != nil {
			r.Log.With("kind", "courses", "err", err).Error("search failed for kind")
			return
		}
		result.Courses = collectMap(items, courseModel)
	}()
	go func() {
		defer wg.Done()
		items, err := r.Catalog.ListTopics(ctx, "", optionsForQuery(query, limit))
		if err != nil {
			r.Log.With("kind", "topics", "err", err).Error("search failed for kind")
			return
		}
		result.Topics = collectMap(items, topicModel)
	}()
	go func() {
		defer wg.Done()
		list, err := r.Tasks.ListPublished(ctx, tasks.TaskFilter{Search: query, Limit: limit})
		if err != nil {
			r.Log.With("kind", "tasks", "err", err).Error("search failed for kind")
			return
		}
		result.Tasks = taskListModel(list).Items
	}()

	wg.Wait()

	ensureEmpty(result)

	return result, nil
}

// ensureEmpty нормализует nil-слайсы в пустые, чтобы GraphQL вернул [] вместо null.
func ensureEmpty(result *model.SearchResults) {
	if result.Universities == nil {
		result.Universities = []*model.University{}
	}
	if result.Programs == nil {
		result.Programs = []*model.Program{}
	}
	if result.Courses == nil {
		result.Courses = []*model.Course{}
	}
	if result.Topics == nil {
		result.Topics = []*model.Topic{}
	}
	if result.Tasks == nil {
		result.Tasks = []*model.ITTaskSummary{}
	}
}

func collectMap[T any, M any](items []T, mapFn func(T) M) []M {
	out := make([]M, 0, len(items))
	for _, item := range items {
		out = append(out, mapFn(item))
	}

	return out
}

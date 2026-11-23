package graphql

import (
	"context"
	"sync"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
	pb "github.com/overmindv/content/pkg/api/content"
)

// optionsForQuery собирает параметры поиска для каталога: только активные элементы.
func optionsForQuery(query string, limit int) entities.ListOptions {
	return entities.ListOptions{Search: query, Status: statusActive, Limit: limit}
}

// Search — единый поиск по каталогу, опубликованным задачам и доступным материалам.
// Каждый вид опрашивается параллельно; сбой источника возвращается как ошибка всего поиска.
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
			Materials:    []*model.MaterialCatalogEntry{},
		}, nil
	}
	pageSize := int32(limit) //nolint:gosec // limit нормализован до диапазона 1..50 выше

	result := &model.SearchResults{}
	errCh := make(chan error, 6)
	var wg sync.WaitGroup
	wg.Add(5)

	groupErr := func() {
		defer wg.Done()
		items, err := r.Catalog.ListUniversities(ctx, optionsForQuery(query, limit))
		if err != nil {
			r.Log.With("kind", "universities", "err", err).Error("search failed for kind")
			errCh <- err

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
			errCh <- err

			return
		}
		result.Programs = collectMap(items, programModel)
	}()
	go func() {
		defer wg.Done()
		items, err := r.Catalog.ListCourses(ctx, "", optionsForQuery(query, limit))
		if err != nil {
			r.Log.With("kind", "courses", "err", err).Error("search failed for kind")
			errCh <- err

			return
		}
		result.Courses = collectMap(items, courseModel)
	}()
	go func() {
		defer wg.Done()
		items, err := r.Catalog.ListTopics(ctx, "", optionsForQuery(query, limit))
		if err != nil {
			r.Log.With("kind", "topics", "err", err).Error("search failed for kind")
			errCh <- err

			return
		}
		result.Topics = collectMap(items, topicModel)
	}()
	go func() {
		defer wg.Done()
		list, err := r.Tasks.ListPublished(ctx, tasks.TaskFilter{Search: query, Limit: limit})
		if err != nil {
			r.Log.With("kind", "tasks", "err", err).Error("search failed for kind")
			errCh <- err

			return
		}
		result.Tasks = taskListModel(list).Items
	}()
	if auth, err := middleware.RequireAuth(ctx); err == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r.Content == nil {
				errCh <- &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}

				return
			}
			page, listErr := r.Content.List(ctx, contentclient.Actor{UserID: auth.UserID, RequestID: middleware.RequestID(ctx)},
				&pb.ListContentCatalogRequest{Status: statusPublished, PageSize: pageSize, Search: query})
			if listErr != nil {
				r.Log.With("kind", "materials", "err", listErr).Error("search failed for kind")
				errCh <- listErr

				return
			}
			result.Materials = materialCatalogModel(page).Items
		}()
	}

	wg.Wait()
	close(errCh)
	if err := <-errCh; err != nil {
		return nil, err
	}

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
	if result.Materials == nil {
		result.Materials = []*model.MaterialCatalogEntry{}
	}
}

func collectMap[T any, M any](items []T, mapFn func(T) M) []M {
	out := make([]M, 0, len(items))
	for _, item := range items {
		out = append(out, mapFn(item))
	}

	return out
}

package graphql

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/tasks"
)

type searchCatalogStub struct {
	entities.CatalogService
	uniOptions entities.ListOptions
}

func (s *searchCatalogStub) ListUniversities(_ context.Context, opts entities.ListOptions) ([]entities.University, error) {
	s.uniOptions = opts
	return []entities.University{{ID: "uni-1", Name: "НИУ ВШЭ", ShortName: "ВШЭ", City: "Москва", Country: "Россия", Status: "active"}}, nil
}

func (s *searchCatalogStub) ListPrograms(_ context.Context, _ string, _ entities.ListOptions) ([]entities.Program, error) {
	return []entities.Program{{ID: "prog-1", Name: "Прикладная математика", Status: "active"}}, nil
}

func (s *searchCatalogStub) ListCourses(_ context.Context, _ string, _ entities.ListOptions) ([]entities.Course, error) {
	return []entities.Course{{ID: "course-1", Name: "Алгоритмы", Status: "active"}}, nil
}

func (s *searchCatalogStub) ListTopics(_ context.Context, _ string, _ entities.ListOptions) ([]entities.Topic, error) {
	return []entities.Topic{{ID: "topic-1", Title: "Динамика", Status: "active"}}, nil
}

type searchTasksStub struct {
	tasks.Service
	lastSearch string
}

func (s *searchTasksStub) ListPublished(_ context.Context, filter tasks.TaskFilter) (tasks.TaskList, error) {
	s.lastSearch = filter.Search
	return tasks.TaskList{Items: []tasks.TaskSummary{{ID: "task-1", Status: "published", Title: "Два указателя", TaskType: "programming"}}}, nil
}

func TestSearchFansOutToAllKinds(t *testing.T) {
	t.Parallel()
	catalog := &searchCatalogStub{}
	tasksStub := &searchTasksStub{}
	resolver := &queryResolver{Resolver: &Resolver{
		Catalog: catalog,
		Tasks:   tasksStub,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}

	result, err := resolver.Search(context.Background(), "алгоритмы", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(result.Universities) != 1 || result.Universities[0].ID != "uni-1" {
		t.Fatalf("universities = %#v", result.Universities)
	}
	if len(result.Programs) != 1 || result.Programs[0].ID != "prog-1" {
		t.Fatalf("programs = %#v", result.Programs)
	}
	if len(result.Courses) != 1 || result.Courses[0].ID != "course-1" {
		t.Fatalf("courses = %#v", result.Courses)
	}
	if len(result.Topics) != 1 || result.Topics[0].ID != "topic-1" {
		t.Fatalf("topics = %#v", result.Topics)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].ID != "task-1" {
		t.Fatalf("tasks = %#v", result.Tasks)
	}

	if catalog.uniOptions.Search != "алгоритмы" || catalog.uniOptions.Status != "active" {
		t.Fatalf("catalog options = %#v", catalog.uniOptions)
	}
	if tasksStub.lastSearch != "алгоритмы" {
		t.Fatalf("task search = %q", tasksStub.lastSearch)
	}
}

func TestSearchEmptyQueryReturnsEmptySlices(t *testing.T) {
	t.Parallel()
	resolver := &queryResolver{Resolver: &Resolver{
		Catalog: &searchCatalogStub{},
		Tasks:   &searchTasksStub{},
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}

	result, err := resolver.Search(context.Background(), "", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Universities == nil || result.Programs == nil || result.Courses == nil || result.Topics == nil || result.Tasks == nil {
		t.Fatalf("пустой запрос должен давать пустые (не nil) слайсы: %#v", result)
	}
	if len(result.Universities)+len(result.Programs)+len(result.Courses)+len(result.Topics)+len(result.Tasks) != 0 {
		t.Fatalf("неожиданные результаты: %#v", result)
	}
}

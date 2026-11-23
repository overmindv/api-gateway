package graphql

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/middleware"
	pb "github.com/overmindv/content/pkg/api/content"
)

type searchCatalogStub struct {
	entities.CatalogService
	uniOptions entities.ListOptions
}

func (s *searchCatalogStub) ListUniversities(_ context.Context, opts entities.ListOptions) ([]entities.University, error) {
	s.uniOptions = opts

	return []entities.University{{ID: "uni-1", Name: "НИУ ВШЭ", ShortName: "ВШЭ", City: "Москва", Country: "Россия", Status: statusActive}}, nil
}

func (s *searchCatalogStub) ListPrograms(_ context.Context, _ string, _ entities.ListOptions) ([]entities.Program, error) {
	return []entities.Program{{ID: "prog-1", Name: "Прикладная математика", Status: statusActive}}, nil
}

func (s *searchCatalogStub) ListCourses(_ context.Context, _ string, _ entities.ListOptions) ([]entities.Course, error) {
	return []entities.Course{{ID: "course-1", Name: "Алгоритмы", Status: statusActive}}, nil
}

func (s *searchCatalogStub) ListTopics(_ context.Context, _ string, _ entities.ListOptions) ([]entities.Topic, error) {
	return []entities.Topic{{ID: "topic-1", Title: "Динамика", Status: statusActive}}, nil
}

type searchTasksStub struct {
	tasks.Service
	lastSearch string
	err        error
}

func (s *searchTasksStub) ListPublished(_ context.Context, filter tasks.TaskFilter) (tasks.TaskList, error) {
	s.lastSearch = filter.Search
	if s.err != nil {
		return tasks.TaskList{}, s.err
	}

	return tasks.TaskList{Items: []tasks.TaskSummary{{ID: "task-1", Status: "published", Title: "Два указателя", TaskType: "programming"}}}, nil
}

type searchContentStub struct {
	contentclient.Service
	actor   contentclient.Actor
	request *pb.ListContentCatalogRequest
	called  bool
}

func (s *searchContentStub) List(_ context.Context, actor contentclient.Actor, request *pb.ListContentCatalogRequest) (*pb.ListContentCatalogResponse, error) {
	s.called, s.actor, s.request = true, actor, request

	return &pb.ListContentCatalogResponse{Entries: []*pb.ContentCatalogEntry{{
		Id: "material-1", Type: "note", Title: "Линейная алгебра", Slug: "linear-algebra", Status: "published",
	}}}, nil
}

func TestSearchFansOutToAllKinds(t *testing.T) {
	t.Parallel()
	catalog := &searchCatalogStub{}
	tasksStub := &searchTasksStub{}
	content := &searchContentStub{}
	resolver := &queryResolver{Resolver: &Resolver{
		Catalog: catalog,
		Tasks:   tasksStub,
		Content: content,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}

	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: testUserID, Token: "jwt"})
	result, err := resolver.Search(ctx, testCatalogSearch, 5)
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
	if len(result.Materials) != 1 || result.Materials[0].ID != "material-1" || result.Materials[0].Type != "note" {
		t.Fatalf("materials = %#v", result.Materials)
	}

	if catalog.uniOptions.Search != testCatalogSearch || catalog.uniOptions.Status != statusActive {
		t.Fatalf("catalog options = %#v", catalog.uniOptions)
	}
	if tasksStub.lastSearch != testCatalogSearch {
		t.Fatalf("task search = %q", tasksStub.lastSearch)
	}
	if !content.called || content.actor.UserID != testUserID || content.request.GetSearch() != testCatalogSearch || content.request.GetPageSize() != 5 {
		t.Fatalf("content request = actor %#v request %#v", content.actor, content.request)
	}
}

func TestSearchAnonymousDoesNotCallContent(t *testing.T) {
	t.Parallel()
	content := &searchContentStub{}
	resolver := &queryResolver{Resolver: &Resolver{
		Catalog: &searchCatalogStub{}, Tasks: &searchTasksStub{}, Content: content,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}

	result, err := resolver.Search(context.Background(), testCatalogSearch, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if content.called || result.Materials == nil || len(result.Materials) != 0 {
		t.Fatalf("anonymous material search = called %v, results %#v", content.called, result.Materials)
	}
}

func TestSearchReturnsUpstreamError(t *testing.T) {
	t.Parallel()
	want := errors.New("tasks unavailable")
	resolver := &queryResolver{Resolver: &Resolver{
		Catalog: &searchCatalogStub{}, Tasks: &searchTasksStub{err: want},
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}}

	if _, err := resolver.Search(context.Background(), testCatalogSearch, 5); !errors.Is(err, want) {
		t.Fatalf("Search error = %v, want %v", err, want)
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
	if result.Universities == nil || result.Programs == nil || result.Courses == nil || result.Topics == nil || result.Tasks == nil || result.Materials == nil {
		t.Fatalf("пустой запрос должен давать пустые (не nil) слайсы: %#v", result)
	}
	if len(result.Universities)+len(result.Programs)+len(result.Courses)+len(result.Topics)+len(result.Tasks) != 0 {
		t.Fatalf("неожиданные результаты: %#v", result)
	}
}

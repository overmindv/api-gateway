package graphql

import (
	"context"
	"errors"
	"testing"

	"github.com/overmindv/api-gateway/internal/apperror"
	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type catalogStub struct {
	entities.CatalogService
	called  bool
	program entities.Program
	course  entities.Course
	topic   entities.Topic
}

func (s *catalogStub) CreateUniversity(_ context.Context, input entities.University, _ entities.Actor) (entities.University, error) {
	s.called = true
	input.ID = "university-id"

	return input, nil
}

func (s *catalogStub) UpdateProgram(_ context.Context, _ string, input entities.Program, _ entities.Actor) (entities.Program, error) {
	s.program = input
	input.ID = "program-id"
	input.Status = "draft"

	return input, nil
}

func (s *catalogStub) UpdateCourse(_ context.Context, _ string, input entities.Course, _ entities.Actor) (entities.Course, error) {
	s.course = input
	input.ID = "course-id"
	input.Status = "draft"

	return input, nil
}

func (s *catalogStub) UpdateTopic(_ context.Context, _ string, input entities.Topic, _ entities.Actor) (entities.Topic, error) {
	s.topic = input
	input.ID = "topic-id"
	input.Status = "draft"
	input.Difficulty = "basic"

	return input, nil
}

func TestCreateUniversityRequiresAdmin(t *testing.T) {
	t.Parallel()
	stub := &catalogStub{}
	resolver := &mutationResolver{Resolver: &Resolver{Catalog: stub}}
	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{
		UserID: "user-id",
		Token:  "token",
		Roles:  []string{"student"},
	})
	_, err := resolver.CreateUniversity(ctx, model.CreateUniversityInput{Name: "НИУ ВШЭ"})
	if !errors.Is(err, apperror.ErrPermissionDenied) {
		t.Fatalf("ожидалась ошибка доступа, получено %v", err)
	}
	if stub.called {
		t.Fatal("Entities не должен вызываться без роли admin")
	}
}

func TestCreateUniversityCallsEntitiesForAdmin(t *testing.T) {
	t.Parallel()
	stub := &catalogStub{}
	resolver := &mutationResolver{Resolver: &Resolver{Catalog: stub}}
	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{
		UserID: "user-id",
		Token:  "token",
		Roles:  []string{"admin"},
	})
	result, err := resolver.CreateUniversity(ctx, model.CreateUniversityInput{Name: "НИУ ВШЭ"})
	if err != nil {
		t.Fatal(err)
	}
	if !stub.called || result.ID != "university-id" {
		t.Fatalf("неожиданный результат: %#v", result)
	}
}

func TestUpdateCatalogBindingsCanBeCleared(t *testing.T) {
	t.Parallel()
	stub := &catalogStub{}
	resolver := &mutationResolver{Resolver: &Resolver{Catalog: stub}}
	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{
		UserID: "user-id",
		Token:  "token",
		Roles:  []string{"admin"},
	})
	clear := true

	if _, err := resolver.UpdateProgram(ctx, "program-id", model.UpdateProgramInput{Name: "Program", ClearUniversity: &clear}); err != nil {
		t.Fatal(err)
	}
	if stub.program.UniversityID != nil {
		t.Fatalf("university_id должен быть nil, получено %v", stub.program.UniversityID)
	}

	if _, err := resolver.UpdateCourse(ctx, "course-id", model.UpdateCourseInput{Name: "Course", ClearProgram: &clear}); err != nil {
		t.Fatal(err)
	}
	if stub.course.ProgramID != nil {
		t.Fatalf("program_id должен быть nil, получено %v", stub.course.ProgramID)
	}

	if _, err := resolver.UpdateTopic(ctx, "topic-id", model.UpdateTopicInput{Title: "Topic", ClearCourse: &clear, ClearParentTopic: &clear}); err != nil {
		t.Fatal(err)
	}
	if stub.topic.CourseID != nil || stub.topic.ParentTopicID != nil {
		t.Fatalf("topic IDs должны быть nil, получено course=%v parent=%v", stub.topic.CourseID, stub.topic.ParentTopicID)
	}
}

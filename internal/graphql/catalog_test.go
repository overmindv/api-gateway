package graphql

import (
	"context"
	"errors"
	"testing"

	"github.com/overmindv/laserbeak/internal/apperror"
	"github.com/overmindv/laserbeak/internal/client/ironhide"
	"github.com/overmindv/laserbeak/internal/graphql/model"
	"github.com/overmindv/laserbeak/internal/middleware"
)

type catalogStub struct {
	ironhide.CatalogService
	called bool
}

func (s *catalogStub) CreateUniversity(_ context.Context, input ironhide.University, _ ironhide.Actor) (ironhide.University, error) {
	s.called = true
	input.ID = "university-id"

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
		t.Fatal("Ironhide не должен вызываться без роли admin")
	}
}

func TestCreateUniversityCallsIronhideForAdmin(t *testing.T) {
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

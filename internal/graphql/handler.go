package graphql

import (
	"context"
	"log/slog"
	"net/http"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/overmindv/api-gateway/internal/client/arcee"
	"github.com/overmindv/api-gateway/internal/client/ironhide"
	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasksit"
	"github.com/overmindv/api-gateway/internal/graphql/generated"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// Handler создаёт GraphQL handler со всеми внутренними сервисами.
func Handler(users arcee.UserService, catalog ironhide.CatalogService, tasks tasksit.Service, log *slog.Logger) http.Handler {
	return HandlerWithTaskHunter(users, catalog, tasks, nil, log)
}

// HandlerWithTaskHunter создаёт GraphQL handler с клиентом очереди сбора.
func HandlerWithTaskHunter(users arcee.UserService, catalog ironhide.CatalogService, tasks tasksit.Service, taskHunter taskhunter.Service, log *slog.Logger) http.Handler {
	var candidates tasksit.CandidateService
	if service, ok := tasks.(tasksit.CandidateService); ok {
		candidates = service
	}
	server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &Resolver{
		Users:      users,
		Catalog:    catalog,
		Tasks:      tasks,
		Candidates: candidates,
		TaskHunter: taskHunter,
	}}))
	server.SetErrorPresenter(ErrorPresenter(log))
	server.SetRecoverFunc(func(ctx context.Context, recovered any) error {
		log.ErrorContext(ctx, "graphql panic recovered", "request_id", middleware.RequestID(ctx), "panic", recovered)

		return gqlgen.DefaultRecover(ctx, recovered)
	})

	return server
}

func Playground() http.Handler {
	return playground.Handler("api-gateway GraphQL", "/graphql")
}

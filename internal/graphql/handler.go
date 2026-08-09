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
	"github.com/overmindv/api-gateway/internal/client/tasksit"
	"github.com/overmindv/api-gateway/internal/graphql/generated"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// Handler создаёт GraphQL handler со всеми внутренними сервисами.
func Handler(users arcee.UserService, catalog ironhide.CatalogService, tasks tasksit.Service, log *slog.Logger) http.Handler {
	server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &Resolver{
		Users:   users,
		Catalog: catalog,
		Tasks:   tasks,
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

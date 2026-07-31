package graphql

import (
	"context"
	"log/slog"
	"net/http"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/overmindv/laserbeak/internal/client/arcee"
	"github.com/overmindv/laserbeak/internal/client/ironhide"
	"github.com/overmindv/laserbeak/internal/graphql/generated"
	"github.com/overmindv/laserbeak/internal/middleware"
)

func Handler(users arcee.UserService, catalog ironhide.CatalogService, log *slog.Logger) http.Handler {
	server := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: &Resolver{Users: users, Catalog: catalog}}))
	server.SetErrorPresenter(ErrorPresenter(log))
	server.SetRecoverFunc(func(ctx context.Context, recovered any) error {
		log.ErrorContext(ctx, "graphql panic recovered", "request_id", middleware.RequestID(ctx), "panic", recovered)

		return gqlgen.DefaultRecover(ctx, recovered)
	})

	return server
}

func Playground() http.Handler {
	return playground.Handler("Laserbeak GraphQL", "/graphql")
}

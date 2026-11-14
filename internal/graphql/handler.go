package graphql

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/feed"
	"github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/generated"
	"github.com/overmindv/api-gateway/internal/middleware"
	"github.com/vektah/gqlparser/v2/ast"
)

const graphqlMultipartLimit = 512 << 10

// Handler создаёт GraphQL handler со всеми внутренними сервисами.
func Handler(users users.UserService, catalog entities.CatalogService, tasks tasks.Service, log *slog.Logger) http.Handler {
	return HandlerWithTaskHunter(users, catalog, tasks, nil, log)
}

// HandlerWithTaskHunter создаёт GraphQL handler с клиентом очереди сбора.
func HandlerWithTaskHunter(users users.UserService, catalog entities.CatalogService, tasksSvc tasks.Service, taskHunter taskhunter.Service, log *slog.Logger) http.Handler {
	return HandlerWithMedia(users, catalog, tasksSvc, taskHunter, nil, log)
}

// HandlerWithMedia создаёт GraphQL handler со всеми текущими сервисами платформы.
func HandlerWithMedia(users users.UserService, catalog entities.CatalogService, tasksSvc tasks.Service, taskHunter taskhunter.Service, mediaSvc media.Service, log *slog.Logger) http.Handler {
	return HandlerWithMediaAndMetrics(users, catalog, tasksSvc, taskHunter, mediaSvc, log, nil)
}

// HandlerWithMediaAndMetrics создаёт GraphQL handler и связывает orchestration metrics.
func HandlerWithMediaAndMetrics(users users.UserService, catalog entities.CatalogService, tasksSvc tasks.Service, taskHunter taskhunter.Service, mediaSvc media.Service, log *slog.Logger, metrics *Metrics) http.Handler {
	return HandlerWithFeedAndMetrics(users, catalog, tasksSvc, taskHunter, mediaSvc, nil, log, metrics)
}

// HandlerWithFeedAndMetrics создаёт GraphQL handler с лентой активностей.
func HandlerWithFeedAndMetrics(users users.UserService, catalog entities.CatalogService, tasksSvc tasks.Service, taskHunter taskhunter.Service, mediaSvc media.Service, feedSvc feed.FeedService, log *slog.Logger, metrics *Metrics) http.Handler {
	return HandlerWithRecommendationsAndMetrics(users, catalog, tasksSvc, taskHunter, mediaSvc, feedSvc, nil, log, metrics)
}

// HandlerWithRecommendationsAndMetrics подключает необязательный preview практики.
func HandlerWithRecommendationsAndMetrics(users users.UserService, catalog entities.CatalogService, tasksSvc tasks.Service, taskHunter taskhunter.Service, mediaSvc media.Service, feedSvc feed.FeedService, recommendationsSvc recommendations.Service, log *slog.Logger, metrics *Metrics) http.Handler {
	var candidates tasks.CandidateService
	if service, ok := tasksSvc.(tasks.CandidateService); ok {
		candidates = service
	}
	server := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: &Resolver{
		Users:           users,
		Catalog:         catalog,
		Tasks:           tasksSvc,
		Candidates:      candidates,
		TaskHunter:      taskHunter,
		Media:           mediaSvc,
		Feed:            feedSvc,
		Recommendations: recommendationsSvc,
		Log:             log,
		Metrics:         metrics,
	}}))
	server.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})
	server.AddTransport(transport.Options{})
	server.AddTransport(transport.GET{})
	server.AddTransport(transport.POST{})
	server.AddTransport(transport.MultipartForm{
		MaxUploadSize: graphqlMultipartLimit,
		MaxMemory:     graphqlMultipartLimit,
	})
	server.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	server.Use(extension.Introspection{})
	server.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})
	server.AroundOperations(limitPracticePlanOperation)
	server.SetErrorPresenter(ErrorPresenter(log))
	server.SetRecoverFunc(func(ctx context.Context, recovered any) error {
		log.ErrorContext(ctx, "graphql panic recovered", "request_id", middleware.RequestID(ctx), "panic", recovered)

		return gqlgen.DefaultRecover(ctx, recovered)
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		server.ServeHTTP(w, r)
	})
}

func Playground() http.Handler {
	return playground.Handler("api-gateway GraphQL", "/graphql")
}

// limitPracticePlanOperation ограничивает дорогое построение одним вызовом на операцию.
// CollectFields учитывает fragments, directives и слияние полей с одинаковым alias.
func limitPracticePlanOperation(ctx context.Context, next gqlgen.OperationHandler) gqlgen.ResponseHandler {
	operation := gqlgen.GetOperationContext(ctx)
	if operation.Operation.Operation == ast.Query || operation.Operation.Operation == ast.Mutation {
		root := "Query"
		if operation.Operation.Operation == ast.Mutation {
			root = "Mutation"
		}
		count := 0
		for _, field := range gqlgen.CollectFields(operation, operation.Operation.SelectionSet, []string{root}) {
			if field.Name != "practicePlan" && field.Name != "learningPlan" && field.Name != "externalTaskSearch" && field.Name != "savedLearningPlan" && field.Name != "myLearningPlans" && field.Name != "saveLearningPlan" && field.Name != "refreshLearningPlan" && field.Name != "cloneLearningPlan" && field.Name != "updateLearningMaterial" && field.Name != "setLearningPlanPublic" && field.Name != "removeLearningPlanTask" {
				continue
			}
			count++
			if count > 1 {
				response := gqlgen.ErrorResponse(ctx, "Не удалось выполнить действие.")
				response.Errors[0].Extensions = map[string]any{"code": "OPERATION_LIMIT_EXCEEDED"}
				return gqlgen.OneShot(response)
			}
		}
	}
	return next(ctx)
}

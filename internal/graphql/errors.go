package graphql

import (
	"context"
	"errors"
	"log/slog"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/overmindv/api-gateway/internal/apperror"
	"github.com/overmindv/api-gateway/internal/client/entities"
	mediaclient "github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/middleware"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

const (
	codeGraphQLError             = "GRAPHQL_ERROR"
	genericActionMessage         = "Не удалось выполнить действие."
	objectMutation               = "Mutation"
	objectQuery                  = "Query"
	warningPartialData           = "PARTIAL_DATA"
	statusActive                 = "active"
	statusPublished              = "published"
	verdictAccepted              = "accepted"
	verdictWrongAnswer           = "wrong_answer"
	runModeGraded                = "graded"
	runModeSamples               = "samples"
	codeNotFound                 = "NOT_FOUND"
	aiStatusCached               = "CACHED"
	aiStatusGenerated            = "GENERATED"
	catalogStatusDraft           = "draft"
	catalogDifficultyBasic       = "basic"
	codeInvalidArgument          = "INVALID_ARGUMENT"
	codeServiceUnavailable       = "SERVICE_UNAVAILABLE"
	executionInfrastructureError = "INFRASTRUCTURE_ERROR"
	testUserID                   = "user-id"
	testPrivateCanary            = "PRIVATE_CANARY"
	testServiceAuth              = "Bearer service-token"
	testGoal                     = "Junior AI"
	practicePlanPath             = "/v1/practice-plan"
	submissionsPath              = "/v1/me/submissions"
	testSearch                   = "user"
	testCatalogSearch            = "алгоритмы"
	testKeyID                    = "key-id"
	pythonLanguage               = "python"
	solutionFile                 = "solution.py"
)

func ErrorPresenter(log *slog.Logger) gqlgen.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		presented := gqlgen.DefaultErrorPresenter(ctx, err)
		code, message := errorCodeAndMessage(err)
		if code == codeGraphQLError {
			if original, ok := presented.Extensions["code"].(string); ok && (original == "COMPLEXITY_LIMIT_EXCEEDED" || original == "OPERATION_LIMIT_EXCEEDED") {
				presented.Message = message

				return presented
			}
			presented.Message = message
			if presented.Extensions == nil {
				presented.Extensions = map[string]any{}
			}
			presented.Extensions["code"] = "GRAPHQL_VALIDATION_FAILED"

			return presented
		}

		presented.Message = message
		if presented.Extensions == nil {
			presented.Extensions = map[string]any{}
		}

		presented.Extensions["code"] = code
		if requestID := middleware.RequestID(ctx); requestID != "" {
			presented.Extensions["request_id"] = requestID
		}

		log.WarnContext(ctx, "graphql error", "request_id", middleware.RequestID(ctx), "code", code, "error", err)

		return presented
	}
}

func errorCodeAndMessage(err error) (string, string) {
	if errors.Is(err, context.DeadlineExceeded) {
		return "DEADLINE_EXCEEDED", genericActionMessage
	}
	if errors.Is(err, apperror.ErrUnauthenticated) {
		return "UNAUTHENTICATED", genericActionMessage
	}
	if errors.Is(err, apperror.ErrPermissionDenied) {
		return "PERMISSION_DENIED", genericActionMessage
	}

	if gqlErr, ok := err.(*gqlerror.Error); ok && gqlErr.Err == nil {
		return codeGraphQLError, genericActionMessage
	}

	var recommendationError *recommendations.Error
	if errors.As(err, &recommendationError) {
		return recommendationError.Code, genericActionMessage
	}

	var upstreamError *users.Error
	if errors.As(err, &upstreamError) {
		return upstreamError.Code, genericActionMessage
	}
	var catalogError *entities.Error
	if errors.As(err, &catalogError) {
		return catalogError.Code, genericActionMessage
	}
	var tasksError *tasks.Error
	if errors.As(err, &tasksError) {
		return tasksError.Code, genericActionMessage
	}
	var collectionError *taskhunter.Error
	if errors.As(err, &collectionError) {
		return collectionError.Code, genericActionMessage
	}
	var mediaError *mediaclient.Error
	if errors.As(err, &mediaError) {
		return mediaError.Code, genericActionMessage
	}

	return "INTERNAL_SERVER_ERROR", genericActionMessage
}

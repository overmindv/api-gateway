package graphql

import (
	"context"
	"errors"
	"log/slog"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/overmindv/api-gateway/internal/apperror"
	"github.com/overmindv/api-gateway/internal/client/entities"
	mediaclient "github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/middleware"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func ErrorPresenter(log *slog.Logger) gqlgen.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		presented := gqlgen.DefaultErrorPresenter(ctx, err)
		code, message := errorCodeAndMessage(err)
		if code == "GRAPHQL_ERROR" {
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
	if errors.Is(err, apperror.ErrUnauthenticated) {
		return "UNAUTHENTICATED", "Не удалось выполнить действие."
	}
	if errors.Is(err, apperror.ErrPermissionDenied) {
		return "PERMISSION_DENIED", "Не удалось выполнить действие."
	}

	if gqlErr, ok := err.(*gqlerror.Error); ok && gqlErr.Err == nil {
		return "GRAPHQL_ERROR", "Не удалось выполнить действие."
	}

	var upstreamError *users.Error
	if errors.As(err, &upstreamError) {
		return upstreamError.Code, "Не удалось выполнить действие."
	}
	var catalogError *entities.Error
	if errors.As(err, &catalogError) {
		return catalogError.Code, "Не удалось выполнить действие."
	}
	var tasksError *tasks.Error
	if errors.As(err, &tasksError) {
		return tasksError.Code, "Не удалось выполнить действие."
	}
	var collectionError *taskhunter.Error
	if errors.As(err, &collectionError) {
		return collectionError.Code, "Не удалось выполнить действие."
	}
	var mediaError *mediaclient.Error
	if errors.As(err, &mediaError) {
		return mediaError.Code, "Не удалось выполнить действие."
	}

	return "INTERNAL_SERVER_ERROR", "Не удалось выполнить действие."
}

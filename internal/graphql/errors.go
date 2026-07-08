package graphql

import (
	"context"
	"errors"
	"log/slog"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/overmindv/laserbeak/internal/apperror"
	"github.com/overmindv/laserbeak/internal/client/arcee"
	"github.com/overmindv/laserbeak/internal/middleware"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func ErrorPresenter(log *slog.Logger) gqlgen.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		presented := gqlgen.DefaultErrorPresenter(ctx, err)
		code, message := errorCodeAndMessage(err)
		if code == "GRAPHQL_ERROR" {
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
		return "UNAUTHENTICATED", "authentication required"
	}

	if gqlErr, ok := err.(*gqlerror.Error); ok && gqlErr.Err == nil {
		return "GRAPHQL_ERROR", gqlErr.Message
	}

	var upstreamError *arcee.Error
	if errors.As(err, &upstreamError) {
		return upstreamError.Code, upstreamError.Message
	}

	return "INTERNAL_SERVER_ERROR", "internal server error"
}

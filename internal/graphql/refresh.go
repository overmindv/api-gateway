package graphql

import (
	"context"
	"errors"
	"time"

	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func setBrowserCredentials(ctx context.Context, payload *users.AuthPayload) {
	middleware.SetSessionToken(ctx, payload.Token, sessionMaxAge(payload.ExpiresAt))
	middleware.ClearRefreshToken(ctx)
	if payload.RefreshToken != "" {
		middleware.SetRefreshToken(ctx, payload.RefreshToken, sessionMaxAge(payload.RefreshExpiresAt))
	}
}

// ensureBrowserSession возвращает только срок, а secrets остаются в httpOnly cookie.
func (r *mutationResolver) ensureBrowserSession(ctx context.Context) (*string, error) {
	if info, err := middleware.RequireAuth(middleware.RefreshAuth(ctx)); err == nil && time.Until(info.ExpiresAt) > 5*time.Second {
		expiry := info.ExpiresAt.Format(time.RFC3339)

		return &expiry, nil
	}
	token := middleware.RefreshToken(ctx)
	if token == "" {
		return nil, nil
	}
	service, ok := r.Users.(users.RefreshService)
	if !ok {
		return nil, errors.New("refresh service unavailable")
	}
	response, err := service.RefreshSession(ctx, token)
	if err != nil {
		var upstream *users.Error
		if errors.As(err, &upstream) && upstream.Code == codeUnauthenticated {
			middleware.ClearSessionToken(ctx)

			return nil, nil
		}

		return nil, err
	}
	if response == nil || response.User == nil || response.Token == "" || response.RefreshToken == "" {
		return nil, errors.New("incomplete refresh response")
	}
	accessExpiry, err := time.Parse(time.RFC3339, response.ExpiresAt)
	if err != nil || !accessExpiry.After(time.Now()) {
		return nil, errors.New("invalid refresh access expiry")
	}
	refreshExpiry, err := time.Parse(time.RFC3339, response.RefreshExpiresAt)
	if err != nil || !refreshExpiry.After(time.Now()) {
		return nil, errors.New("invalid refresh expiry")
	}
	setBrowserCredentials(ctx, response)

	return &response.ExpiresAt, nil
}

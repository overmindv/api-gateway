package graphql

import (
	"context"
	"errors"
	"strings"

	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// RequestAccountRecovery запрашивает письмо восстановления доступа.
// Ответ не зависит от существования email; одинаковую выдачу обеспечивает users.
func (r *mutationResolver) RequestAccountRecovery(ctx context.Context, email string, purpose *model.AccountRecoveryPurpose) (bool, error) {
	service, err := r.recoveryService()
	if err != nil {
		return false, err
	}
	value := users.RecoveryPurposeReset
	if purpose != nil {
		value = strings.ToLower(string(*purpose))
	}
	if value != users.RecoveryPurposeReset && value != users.RecoveryPurposeVerify {
		return false, errors.New("unsupported recovery purpose")
	}

	return service.RequestAccountRecovery(ctx, email, value)
}

// ResetAccountPassword использует одноразовый токен и отзывает прежние сессии.
// После успеха текущие браузерные credentials больше недействительны.
func (r *mutationResolver) ResetAccountPassword(ctx context.Context, token string, password string) (bool, error) {
	service, err := r.recoveryService()
	if err != nil {
		return false, err
	}
	reset, err := service.ResetAccountPassword(ctx, token, password)
	if err != nil {
		return false, err
	}
	middleware.ClearSessionToken(ctx)

	return reset, nil
}

// VerifyAccountEmail подтверждает владение адресом одноразовым токеном.
func (r *mutationResolver) VerifyAccountEmail(ctx context.Context, token string) (bool, error) {
	service, err := r.recoveryService()
	if err != nil {
		return false, err
	}

	return service.VerifyAccountEmail(ctx, token)
}

// recoveryService получает опциональный recovery-контракт Users.
func (r *Resolver) recoveryService() (users.RecoveryService, error) {
	service, ok := r.Users.(users.RecoveryService)
	if !ok || service == nil {
		return nil, errors.New("recovery service unavailable")
	}

	return service, nil
}

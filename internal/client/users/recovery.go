package users

import "context"

const (
	// RecoveryPurposeReset запрашивает письмо со сбросом пароля.
	RecoveryPurposeReset = "reset"
	// RecoveryPurposeVerify запрашивает письмо с подтверждением email.
	RecoveryPurposeVerify = "verify"
)

// RecoveryService расширяет Users API операциями восстановления доступа.
// Все вызовы не требуют access JWT: пользователь ещё не аутентифицирован.
type RecoveryService interface {
	RequestAccountRecovery(context.Context, string, string) (bool, error)
	ResetAccountPassword(context.Context, string, string) (bool, error)
	VerifyAccountEmail(context.Context, string) (bool, error)
}

// RequestAccountRecovery запрашивает письмо восстановления.
// Ответ не зависит от существования email: users сам выравнивает выдачу.
func (c *Client) RequestAccountRecovery(ctx context.Context, email, purpose string) (bool, error) {
	var data struct {
		Requested bool `json:"requestAccountRecovery"`
	}
	err := c.call(ctx, "RequestAccountRecovery",
		`mutation RequestAccountRecovery($email: String!, $purpose: String!) { requestAccountRecovery(email: $email, purpose: $purpose) }`,
		map[string]any{"email": email, "purpose": purpose}, false, &data)
	if err != nil {
		return false, err
	}

	return data.Requested, nil
}

// ResetAccountPassword использует одноразовый reset token и отзывает прежние сессии.
func (c *Client) ResetAccountPassword(ctx context.Context, token, password string) (bool, error) {
	var data struct {
		Reset bool `json:"resetAccountPassword"`
	}
	err := c.call(ctx, "ResetAccountPassword",
		`mutation ResetAccountPassword($token: String!, $password: String!) { resetAccountPassword(token: $token, password: $password) }`,
		map[string]any{"token": token, "password": password}, false, &data)
	if err != nil {
		return false, err
	}

	return data.Reset, nil
}

// VerifyAccountEmail подтверждает владение адресом одноразовым токеном.
func (c *Client) VerifyAccountEmail(ctx context.Context, token string) (bool, error) {
	var data struct {
		Verified bool `json:"verifyAccountEmail"`
	}
	err := c.call(ctx, "VerifyAccountEmail",
		`mutation VerifyAccountEmail($token: String!) { verifyAccountEmail(token: $token) }`,
		map[string]any{"token": token}, false, &data)
	if err != nil {
		return false, err
	}

	return data.Verified, nil
}

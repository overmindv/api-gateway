package graphql

import (
	"time"

	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
)

// defaultSessionTTL задаёт fallback-время жизни session cookie, если expiresAt не парсится.
const defaultSessionTTL = 24 * time.Hour

// sessionMaxAge считает оставшееся время жизни токена для Max-Age cookie.
func sessionMaxAge(expiresAt string) time.Duration {
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return defaultSessionTTL
	}

	remaining := time.Until(expires)
	if remaining <= 0 {
		return defaultSessionTTL
	}

	return remaining
}

func toUser(user *users.User) *model.User {
	return &model.User{
		ID:          user.ID,
		Email:       user.Email,
		Username:    user.Username,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		BirthDate:   user.BirthDate,
		Phone:       user.Phone,
		Roles:       user.Roles,
		IsAdmin:     user.IsAdmin,
		IsSuperuser: user.IsSuperuser,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}
}

func toAuthPayload(response *users.AuthPayload) *model.AuthPayload {
	return &model.AuthPayload{
		User:      toUser(response.User),
		Token:     response.Token,
		ExpiresAt: response.ExpiresAt,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func intValue(value *int, fallback int) int {
	if value == nil {
		return fallback
	}

	return *value
}

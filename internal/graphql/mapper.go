package graphql

import (
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
)

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

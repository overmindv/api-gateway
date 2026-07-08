package graphql

import (
	"github.com/overmindv/laserbeak/internal/client/arcee"
	"github.com/overmindv/laserbeak/internal/graphql/model"
)

func toUser(user *arcee.User) *model.User {
	return &model.User{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		BirthDate: user.BirthDate,
		Phone:     user.Phone,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func toAuthPayload(response *arcee.AuthPayload) *model.AuthPayload {
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

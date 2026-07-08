package graphql

import (
	"errors"
	"testing"

	"github.com/overmindv/laserbeak/internal/apperror"
	"github.com/overmindv/laserbeak/internal/client/arcee"
)

func TestErrorCodeAndMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{
			name: "auth",
			err:  apperror.ErrUnauthenticated,
			code: "UNAUTHENTICATED"},
		{
			name: "upstream",
			err:  &arcee.Error{Code: "NOT_FOUND", Message: "missing"},
			code: "NOT_FOUND"},
		{
			name: "internal",
			err:  errors.New("details"),
			code: "INTERNAL_SERVER_ERROR",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _ := errorCodeAndMessage(test.err)
			if code != test.code {
				t.Fatalf("code = %q", code)
			}
		})
	}
}

package graphql

import (
	"context"
	"errors"
	"testing"

	"github.com/overmindv/api-gateway/internal/apperror"
	"github.com/overmindv/api-gateway/internal/client/arcee"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type userServiceStub struct {
	register              arcee.RegisterInput
	login                 arcee.LoginInput
	getID                 string
	getUsername           string
	listSearch            string
	listLimit, listOffset int
	updateID              string
	update                arcee.UpdateUserInput
	deleteID              string
}

func upstreamUser() *arcee.User {
	return &arcee.User{
		ID:        "user-id",
		Email:     "user@example.com",
		Username:  "user",
		FirstName: "First",
	}
}

func (s *userServiceStub) Register(_ context.Context, input arcee.RegisterInput) (*arcee.AuthPayload, error) {
	s.register = input

	return &arcee.AuthPayload{
		User:      upstreamUser(),
		Token:     "token",
		ExpiresAt: "tomorrow",
	}, nil
}

func (s *userServiceStub) Login(_ context.Context, input arcee.LoginInput) (*arcee.AuthPayload, error) {
	s.login = input

	return &arcee.AuthPayload{
		User:      upstreamUser(),
		Token:     "token",
		ExpiresAt: "tomorrow",
	}, nil
}

func (s *userServiceStub) GetUser(_ context.Context, id string) (*arcee.User, error) {
	s.getID = id

	return upstreamUser(), nil
}

func (s *userServiceStub) GetUserByUsername(_ context.Context, username string) (*arcee.User, error) {
	s.getUsername = username

	return upstreamUser(), nil
}

func (s *userServiceStub) ListUsers(_ context.Context, search string, limit, offset int) ([]*arcee.User, error) {
	s.listSearch, s.listLimit, s.listOffset = search, limit, offset

	return []*arcee.User{upstreamUser()}, nil
}

func (s *userServiceStub) UpdateUser(_ context.Context, id string, input arcee.UpdateUserInput) (*arcee.User, error) {
	s.updateID, s.update = id, input

	return upstreamUser(), nil
}

func (s *userServiceStub) DeleteUser(_ context.Context, id string) (bool, error) {
	s.deleteID = id

	return true, nil
}

func (s *userServiceStub) SetUserAdmin(_ context.Context, id string, admin bool) (*arcee.User, error) {
	user := upstreamUser()
	user.ID = id
	user.IsAdmin = admin

	return user, nil
}

func (s *userServiceStub) SetUserAdminByUsername(_ context.Context, username string, admin bool) (*arcee.User, error) {
	user := upstreamUser()
	user.Username = username
	user.IsAdmin = admin

	return user, nil
}

func TestPublicResolversMapRequests(t *testing.T) {
	stub := &userServiceStub{}
	resolver := &mutationResolver{Resolver: &Resolver{Users: stub}}
	firstName := "First"

	registered, err := resolver.Register(context.Background(), model.RegisterInput{
		Email:     "user@example.com",
		Password:  "password",
		Username:  "user",
		FirstName: &firstName,
	})
	if err != nil {
		t.Fatal(err)
	}

	if stub.register.Email != "user@example.com" || stub.register.FirstName != firstName || registered.Token != "token" {
		t.Fatalf("registration mapping failed: %+v %+v", stub.register, registered)
	}

	if _, err := resolver.Login(context.Background(), model.LoginInput{
		Email:    "user@example.com",
		Password: "password",
	}); err != nil {
		t.Fatal(err)
	}

	if stub.login.Password != "password" {
		t.Fatalf("login mapping failed: %+v", stub.login)
	}
}

func TestProtectedResolversRequireJWT(t *testing.T) {
	stub := &userServiceStub{}

	resolver := &queryResolver{Resolver: &Resolver{Users: stub}}
	if _, err := resolver.GetUser(context.Background(), "user-id"); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated, got %v", err)
	}

	if stub.getID != "" {
		t.Fatal("upstream called without authentication")
	}
}

func TestProtectedResolversMapRequests(t *testing.T) {
	stub := &userServiceStub{}
	root := &Resolver{Users: stub}
	query, mutation := &queryResolver{Resolver: root}, &mutationResolver{Resolver: root}

	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: "user-id", Token: "jwt", Roles: []string{"admin"}})
	if user, err := query.GetUser(ctx, "user-id"); err != nil || user.ID != "user-id" {
		t.Fatalf("GetUser() = %+v, %v", user, err)
	}

	limit, offset := 10, 2
	search := "user"
	if users, err := query.Users(ctx, &search, &limit, &offset); err != nil || len(users) != 1 {
		t.Fatalf("Users() = %+v, %v", users, err)
	}
	if stub.listSearch != "user" || stub.listLimit != 10 || stub.listOffset != 2 {
		t.Fatalf("list mapping failed")
	}

	username, clear := "updated", true
	if _, err := mutation.UpdateUser(ctx, "user-id", model.UpdateUserInput{Username: &username, ClearBirthDate: &clear}); err != nil {
		t.Fatal(err)
	}

	if stub.updateID != "user-id" || stub.update.Username == nil || *stub.update.Username != username || !stub.update.ClearBirthDate {
		t.Fatalf("update mapping failed: %+v", stub.update)
	}

	deleted, err := mutation.DeleteUser(ctx, "user-id")
	if err != nil || !deleted || stub.deleteID != "user-id" {
		t.Fatalf("DeleteUser() = %v, %v", deleted, err)
	}
}

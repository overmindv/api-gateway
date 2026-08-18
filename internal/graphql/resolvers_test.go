package graphql

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/overmindv/api-gateway/internal/apperror"
	mediaclient "github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type mediaServiceStub struct {
	mediaclient.Service
	calls int
	ids   []string
	err   error
}

// ResolvePublicFiles фиксирует batch-вызов и возвращает CDN-варианты для всех IDs.
func (s *mediaServiceStub) ResolvePublicFiles(_ context.Context, ids, _ []string) ([]mediaclient.PublicFile, error) {
	s.calls++
	s.ids = append([]string(nil), ids...)
	if s.err != nil {
		return nil, s.err
	}
	files := make([]mediaclient.PublicFile, 0, len(ids))
	for _, id := range ids {
		files = append(files, mediaclient.PublicFile{
			FileID: id,
			URLs: map[string]string{
				"w128": "https://cdn.example/" + id + "/w128.webp",
				"w768": "https://cdn.example/" + id + "/w768.webp",
			},
		})
	}

	return files, nil
}

// TestHydratePublicUsersFailsSoft проверяет fallback и метрику при недоступном Media.
func TestHydratePublicUsersFailsSoft(t *testing.T) {
	fileID := "file-id"
	metrics := &Metrics{}
	resolver := &Resolver{
		Media:   &mediaServiceStub{err: errors.New("media unavailable")},
		Metrics: metrics,
	}
	result := resolver.hydratePublicUsers(context.Background(), []*users.PublicUser{{
		ID:           "user-id",
		Username:     "user",
		AvatarFileID: &fileID,
	}})
	if len(result) != 1 || result[0].Avatar != nil || metrics.avatarResolutionFailures.Load() != 1 {
		t.Fatalf("ожидался fail-soft fallback с метрикой: %+v metric=%d", result, metrics.avatarResolutionFailures.Load())
	}
}

type userServiceStub struct {
	register              users.RegisterInput
	login                 users.LoginInput
	getID                 string
	getUsername           string
	listSearch            string
	listLimit, listOffset int
	updateID              string
	update                users.UpdateUserInput
	deleteID              string
}

func upstreamUser() *users.User {
	return &users.User{
		ID:        "user-id",
		Email:     "user@example.com",
		Username:  "user",
		FirstName: "First",
	}
}

func (s *userServiceStub) Register(_ context.Context, input users.RegisterInput) (*users.AuthPayload, error) {
	s.register = input

	return &users.AuthPayload{
		User:      upstreamUser(),
		Token:     "token",
		ExpiresAt: "tomorrow",
	}, nil
}

func (s *userServiceStub) Login(_ context.Context, input users.LoginInput) (*users.AuthPayload, error) {
	s.login = input

	return &users.AuthPayload{
		User:      upstreamUser(),
		Token:     "token",
		ExpiresAt: "tomorrow",
	}, nil
}

func (s *userServiceStub) GetUser(_ context.Context, id string) (*users.User, error) {
	s.getID = id

	return upstreamUser(), nil
}

func (s *userServiceStub) GetUserByUsername(_ context.Context, username string) (*users.User, error) {
	s.getUsername = username

	return upstreamUser(), nil
}

func (s *userServiceStub) ListUsers(_ context.Context, search string, limit, offset int) ([]*users.User, error) {
	s.listSearch, s.listLimit, s.listOffset = search, limit, offset

	return []*users.User{upstreamUser()}, nil
}

func (s *userServiceStub) UpdateUser(_ context.Context, id string, input users.UpdateUserInput) (*users.User, error) {
	s.updateID, s.update = id, input

	return upstreamUser(), nil
}

func (s *userServiceStub) DeleteUser(_ context.Context, id string) (bool, error) {
	s.deleteID = id

	return true, nil
}

func (s *userServiceStub) SetUserAdmin(_ context.Context, id string, admin bool) (*users.User, error) {
	user := upstreamUser()
	user.ID = id
	user.IsAdmin = admin

	return user, nil
}

func (s *userServiceStub) SetUserAdminByUsername(_ context.Context, username string, admin bool) (*users.User, error) {
	user := upstreamUser()
	user.Username = username
	user.IsAdmin = admin

	return user, nil
}

func (s *userServiceStub) SetMyAvatar(context.Context, *string) (*users.User, error) {
	return upstreamUser(), nil
}

func (s *userServiceStub) GetPublicUser(context.Context, string) (*users.PublicUser, error) {
	return &users.PublicUser{ID: "user-id", Username: "user"}, nil
}

func (s *userServiceStub) ListPublicUsers(context.Context, string, int, int) ([]*users.PublicUser, error) {
	return []*users.PublicUser{{ID: "user-id", Username: "user"}}, nil
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

// TestHydratePublicUsersUsesSingleMediaBatch проверяет отсутствие N+1 при выдаче пользователей.
func TestHydratePublicUsersUsesSingleMediaBatch(t *testing.T) {
	media := &mediaServiceStub{}
	resolver := &Resolver{Media: media}
	items := make([]*users.PublicUser, 0, 50)
	for index := 0; index < 50; index++ {
		fileID := "file-" + strconv.Itoa(index)
		items = append(items, &users.PublicUser{
			ID:           "user-" + fileID,
			Username:     "user",
			AvatarFileID: &fileID,
		})
	}
	result := resolver.hydratePublicUsers(context.Background(), items)
	if media.calls != 1 || len(media.ids) != 50 {
		t.Fatalf("ожидался один batch-вызов на 50 IDs: calls=%d ids=%d", media.calls, len(media.ids))
	}
	if len(result) != 50 || result[0].Avatar == nil {
		t.Fatalf("аватары не гидратированы: %+v", result)
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

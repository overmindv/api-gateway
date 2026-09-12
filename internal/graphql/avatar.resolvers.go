package graphql

import (
	"context"
	"errors"

	"github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// SetMyAvatar устанавливает или очищает аватар текущего пользователя.
func (r *mutationResolver) SetMyAvatar(ctx context.Context, fileID *string) (*model.User, error) {
	if _, err := middleware.RequireAuth(ctx); err != nil {
		return nil, err
	}
	user, err := r.Users.SetMyAvatar(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("users returned an empty avatar response")
	}
	result := r.hydrateUsers(ctx, []*users.User{user})

	return result[0], nil
}

// UserProfile возвращает безопасный профиль авторизованному пользователю.
func (r *queryResolver) UserProfile(ctx context.Context, id string) (*model.PublicUser, error) {
	if _, err := middleware.RequireAuth(ctx); err != nil {
		return nil, err
	}
	user, err := r.Resolver.Users.GetPublicUser(ctx, id) //nolint:staticcheck // Поле Users конфликтует с GraphQL resolver Users.
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("users returned an empty public profile response")
	}
	result := r.hydratePublicUsers(ctx, []*users.PublicUser{user})

	return result[0], nil
}

// SearchUsers выполняет безопасный поиск пользователей с offset pagination.
func (r *queryResolver) SearchUsers(ctx context.Context, search string, pagination *model.PaginationInput) (*model.PublicUserConnection, error) {
	if _, err := middleware.RequireAuth(ctx); err != nil {
		return nil, err
	}
	limit, offset := 20, 0
	if pagination != nil {
		limit = intValue(pagination.Limit, 20)
		offset = intValue(pagination.Offset, 0)
	}
	items, err := r.Resolver.Users.ListPublicUsers(ctx, search, limit, offset) //nolint:staticcheck // Поле Users конфликтует с GraphQL resolver Users.
	if err != nil {
		return nil, err
	}

	return &model.PublicUserConnection{
		Items:  r.hydratePublicUsers(ctx, items),
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (r *Resolver) hydrateUsers(ctx context.Context, items []*users.User) []*model.User {
	models := make([]*model.User, 0, len(items))
	ids := make([]string, 0, len(items))
	modelAvatarIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		models = append(models, toUser(item))
		avatarID := ""
		if item.AvatarFileID != nil {
			avatarID = *item.AvatarFileID
			ids = append(ids, avatarID)
		}
		modelAvatarIDs = append(modelAvatarIDs, avatarID)
	}
	resolved := r.resolveAvatars(ctx, ids)
	for index, avatarID := range modelAvatarIDs {
		if avatarID != "" {
			models[index].Avatar = resolved[avatarID]
		}
	}

	return models
}

func (r *Resolver) hydratePublicUsers(ctx context.Context, items []*users.PublicUser) []*model.PublicUser {
	models := make([]*model.PublicUser, 0, len(items))
	ids := make([]string, 0, len(items))
	modelAvatarIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		models = append(models, &model.PublicUser{
			ID:        item.ID,
			Username:  item.Username,
			FirstName: item.FirstName,
			LastName:  item.LastName,
			IsAdmin:   item.IsAdmin,
			CreatedAt: item.CreatedAt,
		})
		avatarID := ""
		if item.AvatarFileID != nil {
			avatarID = *item.AvatarFileID
			ids = append(ids, avatarID)
		}
		modelAvatarIDs = append(modelAvatarIDs, avatarID)
	}
	resolved := r.resolveAvatars(ctx, ids)
	for index, avatarID := range modelAvatarIDs {
		if avatarID != "" {
			models[index].Avatar = resolved[avatarID]
		}
	}

	return models
}

func (r *Resolver) resolveAvatars(ctx context.Context, ids []string) map[string]*model.UserAvatar {
	result := make(map[string]*model.UserAvatar)
	if len(ids) == 0 || r.Media == nil {
		return result
	}
	files, err := r.Media.ResolvePublicFiles(ctx, ids, []string{"w128", "w320", "w768"})
	if err != nil {
		r.Metrics.RecordAvatarResolutionFailure()
		if r.Log != nil {
			r.Log.WarnContext(ctx, "avatar batch resolution failed", "error", err, "count", len(ids))
		}

		return result
	}
	for _, file := range files {
		small := firstURL(file, "w128", "w320", "original")
		medium := firstURL(file, "w768", "w320", "original")
		if small != "" && medium != "" {
			result[file.FileID] = &model.UserAvatar{
				FileID:    file.FileID,
				SmallURL:  small,
				MediumURL: medium,
			}
		}
	}

	return result
}

func firstURL(file media.PublicFile, variants ...string) string {
	for _, variant := range variants {
		if value := file.URLs[variant]; value != "" {
			return value
		}
	}

	return ""
}

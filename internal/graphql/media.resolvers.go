package graphql

import (
	"context"
	"sort"

	"github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// CreateMediaUpload создаёт прямую загрузку без передачи бинарных данных через gateway.
func (r *mutationResolver) CreateMediaUpload(ctx context.Context, input model.CreateMediaUploadInput) (*model.MediaUpload, error) {
	actor, err := mediaActor(ctx)
	if err != nil {
		return nil, err
	}
	result, err := r.Media.CreateUpload(ctx, media.CreateUploadInput{
		OriginalName: input.OriginalName,
		ContentType:  input.ContentType,
		SizeBytes:    int64(input.SizeBytes),
		Checksum:     input.ChecksumSha256,
		Purpose:      input.Purpose.String(),
		Visibility:   input.Visibility.String(),
	}, actor)
	if err != nil {
		return nil, err
	}

	return mediaUploadModel(result), nil
}

// CreateMediaUploadParts подписывает выбранные части multipart upload.
func (r *mutationResolver) CreateMediaUploadParts(ctx context.Context, fileID string, partNumbers []int) ([]*model.MediaUploadPart, error) {
	actor, err := mediaActor(ctx)
	if err != nil {
		return nil, err
	}
	result, err := r.Media.CreateParts(ctx, fileID, partNumbers, actor)
	if err != nil {
		return nil, err
	}
	parts := make([]*model.MediaUploadPart, 0, len(result))
	for _, part := range result {
		parts = append(parts, &model.MediaUploadPart{PartNumber: part.PartNumber, URL: part.URL, ExpiresAt: part.ExpiresAt})
	}

	return parts, nil
}

// CompleteMediaUpload завершает direct upload и запускает асинхронную проверку.
func (r *mutationResolver) CompleteMediaUpload(ctx context.Context, input model.CompleteMediaUploadInput) (*model.MediaFile, error) {
	actor, err := mediaActor(ctx)
	if err != nil {
		return nil, err
	}
	parts := make([]media.CompletedPart, 0, len(input.Parts))
	for _, part := range input.Parts {
		parts = append(parts, media.CompletedPart{PartNumber: part.PartNumber, ETag: part.Etag})
	}
	result, err := r.Media.CompleteUpload(ctx, input.FileID, parts, actor)
	if err != nil {
		return nil, err
	}

	return mediaFileModel(result), nil
}

// DeleteMediaFile выполняет soft delete принадлежащего пользователю файла.
func (r *mutationResolver) DeleteMediaFile(ctx context.Context, id string) (bool, error) {
	actor, err := mediaActor(ctx)
	if err != nil {
		return false, err
	}
	err = r.Media.DeleteFile(ctx, id, actor)

	return err == nil, err
}

// MediaFile возвращает доступные actor метаданные файла.
func (r *queryResolver) MediaFile(ctx context.Context, id string) (*model.MediaFile, error) {
	actor := optionalMediaActor(ctx)
	result, err := r.Media.GetFile(ctx, id, actor)
	if err != nil {
		return nil, err
	}

	return mediaFileModel(result), nil
}

// MyMediaFiles возвращает страницу файлов текущего пользователя.
func (r *queryResolver) MyMediaFiles(ctx context.Context, filter *model.MediaFileFilter, pagination *model.PaginationInput) (*model.MediaFileConnection, error) {
	actor, err := mediaActor(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := 20, 0
	if pagination != nil {
		limit, offset = intValue(pagination.Limit, 20), intValue(pagination.Offset, 0)
	}
	status := ""
	if filter != nil && filter.Status != nil {
		status = filter.Status.String()
	}
	result, err := r.Media.ListFiles(ctx, status, limit, offset, actor)
	if err != nil {
		return nil, err
	}
	items := make([]*model.MediaFile, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, mediaFileModel(item))
	}

	return &model.MediaFileConnection{Items: items, Limit: result.Limit, Offset: result.Offset}, nil
}

// MediaDownloadURL возвращает public URL или короткий private URL; variant зарезервирован контрактом.
func (r *queryResolver) MediaDownloadURL(ctx context.Context, fileID string, variant *string) (*model.MediaDownload, error) {
	actor := optionalMediaActor(ctx)
	variantName := "original"
	if variant != nil && *variant != "" {
		variantName = *variant
	}
	result, err := r.Media.DownloadURL(ctx, fileID, variantName, actor)
	if err != nil {
		return nil, err
	}

	return &model.MediaDownload{URL: result.URL, ExpiresAt: result.ExpiresAt}, nil
}

func mediaActor(ctx context.Context) (media.Actor, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return media.Actor{}, err
	}

	return media.Actor{UserID: auth.UserID, Roles: auth.Roles}, nil
}

func optionalMediaActor(ctx context.Context) media.Actor {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return media.Actor{}
	}

	return media.Actor{UserID: auth.UserID, Roles: auth.Roles}
}

func mediaUploadModel(item media.UploadTarget) *model.MediaUpload {
	return &model.MediaUpload{
		FileID: item.FileID, Mode: model.MediaUploadMode(item.Mode), URL: item.URL,
		Fields: mediaFields(item.Fields), Headers: mediaFields(item.Headers), MultipartUploadID: item.MultipartID,
		PartSize: int(item.PartSize), ExpiresAt: item.ExpiresAt,
	}
}

func mediaFields(values map[string]string) []*model.MediaFormField {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]*model.MediaFormField, 0, len(keys))
	for _, key := range keys {
		result = append(result, &model.MediaFormField{Name: key, Value: values[key]})
	}

	return result
}

func mediaFileModel(item media.File) *model.MediaFile {
	return &model.MediaFile{
		ID: item.ID, OwnerUserID: item.OwnerUserID, Purpose: model.MediaPurpose(item.Purpose),
		Visibility: model.MediaVisibility(item.Visibility), OriginalName: item.OriginalName,
		DeclaredContentType: item.DeclaredContentType, DetectedContentType: item.DetectedContentType,
		SizeBytes: int(item.SizeBytes), Status: model.MediaStatus(item.Status), FailureCode: item.FailureCode,
		PublicURL: item.PublicURL, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt,
	}
}

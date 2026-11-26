package graphql

import (
	"context"
	"errors"
	"sort"

	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// RequestAccountExport создаёт запрос экспорта после проверки текущего пароля.
func (r *mutationResolver) RequestAccountExport(ctx context.Context, password string) (*model.PrivacyRequestStart, error) {
	return r.preparePrivacy(ctx, "export", password)
}

// RequestAccountDeletion создаёт запрос удаления после проверки текущего пароля.
func (r *mutationResolver) RequestAccountDeletion(ctx context.Context, password string) (*model.PrivacyRequestStart, error) {
	return r.preparePrivacy(ctx, "delete", password)
}

// ConfirmPrivacyRequest подтверждает запрос одноразовым receipt.
// Для удаления текущие браузерные credentials отозваны на стороне Users.
func (r *mutationResolver) ConfirmPrivacyRequest(ctx context.Context, id string, receipt string) (*model.PrivacyRequest, error) {
	service, err := r.privacyService()
	if err != nil {
		return nil, err
	}
	request, err := service.ConfirmPrivacyRequest(ctx, id, receipt)
	if err != nil {
		return nil, err
	}
	if request.Kind == "delete" {
		middleware.ClearSessionToken(ctx)
	}

	return privacyRequestModel(request), nil
}

// PrivacyRequest возвращает состояние запроса по receipt.
func (r *queryResolver) PrivacyRequest(ctx context.Context, id string, receipt string) (*model.PrivacyRequest, error) {
	service, err := r.privacyService()
	if err != nil {
		return nil, err
	}
	request, err := service.PrivacyRequestStatus(ctx, id, receipt)
	if err != nil {
		return nil, err
	}

	return privacyRequestModel(request), nil
}

// AccountExport скачивает готовый артефакт владельца аккаунта.
func (r *queryResolver) AccountExport(ctx context.Context, id string) (*model.AccountExport, error) {
	if _, err := middleware.RequireAuth(ctx); err != nil {
		return nil, err
	}
	service, err := r.privacyService()
	if err != nil {
		return nil, err
	}
	export, err := service.AccountExport(ctx, id)
	if err != nil {
		return nil, err
	}

	return accountExportModel(export), nil
}

func (r *mutationResolver) preparePrivacy(ctx context.Context, kind, password string) (*model.PrivacyRequestStart, error) {
	if _, err := middleware.RequireAuth(ctx); err != nil {
		return nil, err
	}
	service, err := r.privacyService()
	if err != nil {
		return nil, err
	}
	start, err := service.PreparePrivacyRequest(ctx, kind, password)
	if err != nil {
		return nil, err
	}

	return &model.PrivacyRequestStart{Request: privacyRequestModel(start.Request), Receipt: start.Receipt}, nil
}

// privacyService получает опциональный privacy-контракт Users.
func (r *Resolver) privacyService() (users.PrivacyService, error) {
	service, ok := r.Users.(users.PrivacyService)
	if !ok || service == nil {
		return nil, errors.New("privacy service unavailable")
	}

	return service, nil
}

func privacyRequestModel(value users.PrivacyRequest) *model.PrivacyRequest {
	owners := make([]*model.PrivacyOwnerStatus, 0, len(value.Owners))
	for _, owner := range value.Owners {
		owners = append(owners, &model.PrivacyOwnerStatus{
			Owner:     owner.Owner,
			State:     owner.State,
			ErrorCode: optionalString(owner.ErrorCode),
		})
	}

	return &model.PrivacyRequest{
		ID:          value.ID,
		Kind:        value.Kind,
		State:       value.State,
		Publication: optionalString(value.Publication),
		CreatedAt:   value.CreatedAt,
		ExpiresAt:   value.ExpiresAt,
		Owners:      owners,
	}
}

func accountExportModel(value users.AccountExport) *model.AccountExport {
	owners := make([]string, 0, len(value.Services))
	for owner := range value.Services {
		owners = append(owners, owner)
	}
	sort.Strings(owners)
	services := make([]*model.AccountExportService, 0, len(owners))
	for _, owner := range owners {
		pages := make([]*model.AccountExportPage, 0, len(value.Services[owner]))
		for _, page := range value.Services[owner] {
			pages = append(pages, accountExportPageModel(page))
		}
		services = append(services, &model.AccountExportService{Owner: owner, Pages: pages})
	}

	return &model.AccountExport{
		Profile: &model.AccountExportProfile{
			ID: value.Profile.ID, Email: value.Profile.Email, Username: value.Profile.Username,
			FirstName: value.Profile.FirstName, LastName: value.Profile.LastName,
			BirthDate: value.Profile.BirthDate, Phone: value.Profile.Phone, AvatarFileID: value.Profile.AvatarFileID,
			Timezone: value.Profile.Timezone, Locale: value.Profile.Locale,
			PublicName: value.Profile.PublicName, LeaderboardVisible: value.Profile.LeaderboardVisible,
			CreatedAt: value.Profile.CreatedAt,
		},
		Services: services,
	}
}

func accountExportPageModel(value users.AccountExportPage) *model.AccountExportPage {
	records := make([]*model.AccountExportRecord, 0, len(value.Records))
	for _, record := range value.Records {
		records = append(records, &model.AccountExportRecord{
			UserID: record.UserID, ID: record.ID, Kind: record.Kind, OccurredAt: record.OccurredAt,
			ResourceID: optionalString(record.ResourceID), Title: optionalString(record.Title),
			Text: optionalString(record.Text), Result: optionalString(record.Result),
		})
	}

	return &model.AccountExportPage{
		UserID:     value.UserID,
		Watermark:  value.Watermark,
		NextCursor: optionalString(value.NextCursor),
		Records:    records,
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

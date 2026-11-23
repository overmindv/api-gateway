package graphql

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (r *Resolver) materialService(ctx context.Context) (contentclient.Service, contentclient.Actor, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, contentclient.Actor{}, err
	}
	if r.Content == nil {
		return nil, contentclient.Actor{}, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}

	return r.Content, contentclient.Actor{UserID: auth.UserID, RequestID: middleware.RequestID(ctx)}, nil
}

func invalidMaterialInput() error { return &contentclient.Error{Code: "INVALID_ARGUMENT"} }

func materialID(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || len(value) != 36 {
		return "", invalidMaterialInput()
	}

	return id.String(), nil
}

func materialCatalogRequest(filter *model.MaterialCatalogFilter, limit int, cursor *string) (*pb.ListContentCatalogRequest, error) {
	if limit < 1 || limit > 50 || (cursor != nil && len(*cursor) > 256) {
		return nil, invalidMaterialInput()
	}
	req := &pb.ListContentCatalogRequest{PageSize: int32(limit)}
	if cursor != nil {
		req.Cursor = *cursor
	}
	if filter == nil {
		return req, nil
	}
	var err error
	if filter.TopicID != nil {
		if req.TopicId, err = materialID(*filter.TopicID); err != nil {
			return nil, err
		}
	}
	if filter.AuthorID != nil {
		if req.AuthorId, err = materialID(*filter.AuthorID); err != nil {
			return nil, err
		}
	}
	if filter.Status != nil {
		req.Status = string(*filter.Status)
	}

	return req, nil
}

func materialHistoryRequest(id string, limit int, before *int) (*pb.ListContentRevisionHistoryRequest, error) {
	id, err := materialID(id)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 50 {
		return nil, invalidMaterialInput()
	}
	req := &pb.ListContentRevisionHistoryRequest{ContentItemId: id, PageSize: int32(limit)}
	if before != nil {
		cursor := *before
		if cursor < 1 || cursor > math.MaxInt32 {
			return nil, invalidMaterialInput()
		}
		req.BeforeRevision = int32(cursor)
	}

	return req, nil
}

func materialCreateRequest(input model.CreateMaterialDraftInput) (*pb.CreateContentItemRequest, error) {
	if len(input.Title) > 256 || len(input.Slug) > 160 || len(input.Description) > 4096 || len(input.Tags) > 32 ||
		!validDraftText(input.Source, input.Message) {
		return nil, invalidMaterialInput()
	}
	for _, tag := range input.Tags {
		if len(tag) > 80 {
			return nil, invalidMaterialInput()
		}
	}
	assets, err := materialAttachments(input.Attachments)
	if err != nil {
		return nil, err
	}
	learning, err := materialLearningInput(input.Learning)
	if err != nil {
		return nil, err
	}

	return &pb.CreateContentItemRequest{
		Type: string(input.Type), Status: "draft", Title: input.Title, Slug: input.Slug, Description: input.Description,
		Format: string(input.Format), Source: input.Source, Message: input.Message, Tags: input.Tags, Assets: assets, Learning: learning,
	}, nil
}

func materialSaveRequest(input model.SaveMaterialDraftInput) (*pb.CreateContentRevisionRequest, error) {
	id, err := materialID(input.MaterialID)
	if err != nil {
		return nil, err
	}
	expected, err := materialID(input.ExpectedRevisionID)
	if err != nil {
		return nil, err
	}
	if !validDraftText(input.Source, input.Message) || (input.Attachments != nil && input.Metadata == nil) {
		return nil, invalidMaterialInput()
	}
	assets, err := materialAttachments(input.Attachments)
	if err != nil {
		return nil, err
	}
	req := &pb.CreateContentRevisionRequest{
		ContentItemId: id, ExpectedRevisionId: expected, Format: string(input.Format), Source: input.Source, Message: input.Message,
	}
	if assets != nil {
		req.Attachments = &pb.AttachmentSet{Assets: assets}
	}

	return req, nil
}

func validDraftText(source, message string) bool {
	return len(source) <= 128<<10 && len(message) <= 1024
}

func materialDraftMetadata(input *model.MaterialDraftMetadataInput) (*pb.DraftMetadata, error) {
	if input == nil {
		return nil, nil
	}
	title := strings.TrimSpace(input.Title)
	if title == "" || len(input.Title) > 256 || strings.IndexByte(title, 0) >= 0 || len(input.TopicIds) > 8 {
		return nil, invalidMaterialInput()
	}
	topics, err := materialTopicIDs(input.TopicIds)
	if err != nil {
		return nil, err
	}

	return &pb.DraftMetadata{Title: title, TopicIds: topics}, nil
}

func materialAttachments(input []*model.MaterialAttachmentInput) ([]*pb.CreateContentAsset, error) {
	if input == nil {
		return nil, nil
	}
	if len(input) > 32 {
		return nil, invalidMaterialInput()
	}
	assets := make([]*pb.CreateContentAsset, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, asset := range input {
		if asset == nil || asset.Position < 0 || asset.Position > math.MaxInt32 ||
			len(asset.Title) > 256 || strings.IndexByte(asset.Title, 0) >= 0 {
			return nil, invalidMaterialInput()
		}
		id, err := materialID(asset.FileID)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[id]; exists {
			return nil, invalidMaterialInput()
		}
		seen[id] = struct{}{}
		assets = append(assets, &pb.CreateContentAsset{AssetId: id, Kind: string(asset.Kind), Title: asset.Title, Position: int32(asset.Position)})
	}

	return assets, nil
}

func materialLearningInput(input *model.MaterialLearningInput) (*pb.ContentLearningMetadata, error) {
	if input == nil {
		return nil, nil
	}
	if len(input.TopicIds) > 8 || len(input.LearningOutcomes) > 8 || input.EstimatedMinutes < 0 || input.EstimatedMinutes > 480 {
		return nil, invalidMaterialInput()
	}
	for _, outcome := range input.LearningOutcomes {
		if len(outcome) > 256 {
			return nil, invalidMaterialInput()
		}
	}
	topics, err := materialTopicIDs(input.TopicIds)
	if err != nil {
		return nil, err
	}

	return &pb.ContentLearningMetadata{
		TopicIds: topics, LearningOutcomes: input.LearningOutcomes, EstimatedMinutes: int32(input.EstimatedMinutes),
	}, nil
}

func materialTopicIDs(input []string) ([]string, error) {
	topics := make([]string, 0, len(input))
	for _, raw := range input {
		id, err := materialID(raw)
		if err != nil {
			return nil, err
		}
		topics = append(topics, id)
	}

	return topics, nil
}

func materialModel(item *pb.ContentItem) *model.Material {
	if item == nil {
		return nil
	}
	tags := make([]string, 0, len(item.Tags))
	for _, tag := range item.Tags {
		tags = append(tags, tag.GetName())
	}

	return &model.Material{
		ID: item.Id, Type: model.MaterialType(item.Type), Status: model.MaterialStatus(item.Status),
		Title: item.Title, Slug: item.Slug, Description: item.Description, AuthorID: materialOptional(item.CreatedBy),
		Revision: materialRevisionModel(item.CurrentRevision), PublishedRevisionID: materialOptional(item.PublishedRevisionId),
		PublishedAt: materialOptional(materialTimestamp(item.PublishedAt)), UpdatedAt: materialTimestamp(item.UpdatedAt),
		Tags: tags, Attachments: materialAttachmentModels(item.Assets), Learning: materialLearningModel(item.Learning),
	}
}

func materialAttachmentModels(assets []*pb.ContentAsset) []*model.MaterialAttachment {
	attachments := make([]*model.MaterialAttachment, 0, len(assets))
	for _, a := range assets {
		attachments = append(attachments, &model.MaterialAttachment{
			FileID: a.GetAssetId(), Kind: model.MaterialAssetKind(a.GetKind()), Title: a.GetTitle(), Position: int(a.GetPosition()),
		})
	}

	return attachments
}

func materialCatalogModel(page *pb.ListContentCatalogResponse) *model.MaterialCatalogPage {
	items := make([]*model.MaterialCatalogEntry, 0, len(page.GetEntries()))
	for _, row := range page.GetEntries() {
		items = append(items, &model.MaterialCatalogEntry{
			ID: row.GetId(), Type: model.MaterialType(row.GetType()), Title: row.GetTitle(), Slug: row.GetSlug(), Status: model.MaterialStatus(row.GetStatus()),
			AuthorID: materialOptional(row.GetAuthorId()), RevisionID: materialOptional(row.GetRevisionId()),
			CreatedAt: materialTimestamp(row.GetCreatedAt()), Learning: materialLearningModel(row.GetLearning()),
		})
	}

	return &model.MaterialCatalogPage{Items: items, NextCursor: materialOptional(page.GetNextCursor())}
}

func materialRevisionModel(revision *pb.ContentRevision) *model.MaterialRevision {
	if revision == nil {
		return nil
	}

	return &model.MaterialRevision{
		ID: revision.Id, Number: int(revision.Revision), Source: revision.Source, Format: model.MaterialFormat(revision.Format),
		Message: revision.Message, AuthorID: materialOptional(revision.CreatedBy), CreatedAt: materialTimestamp(revision.CreatedAt),
		RestoredFromRevisionID: materialOptional(revision.GetRestoredFromRevisionId()),
	}
}

func materialHistoryModel(page *pb.ListContentRevisionHistoryResponse) *model.MaterialRevisionPage {
	items := make([]*model.MaterialRevisionSummary, 0, len(page.GetItems()))
	for _, row := range page.GetItems() {
		items = append(items, &model.MaterialRevisionSummary{
			ID: row.GetId(), Number: int(row.GetRevision()), Format: model.MaterialFormat(row.GetFormat()), Message: row.GetMessage(),
			AuthorID: materialOptional(row.GetCreatedBy()), CreatedAt: materialTimestamp(row.GetCreatedAt()),
			RestoredFromRevisionID: materialOptional(row.GetRestoredFromRevisionId()),
		})
	}
	var next *int
	if cursor := page.GetNextBeforeRevision(); cursor != 0 {
		number := int(cursor)
		next = &number
	}

	return &model.MaterialRevisionPage{Items: items, NextBeforeRevision: next}
}

func materialLearningModel(learning *pb.ContentLearningMetadata) *model.MaterialLearning {
	if learning == nil {
		return nil
	}

	return &model.MaterialLearning{
		TopicIds: learning.TopicIds, LearningOutcomes: learning.LearningOutcomes, EstimatedMinutes: int(learning.EstimatedMinutes),
	}
}

func materialOptional(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

func materialTimestamp(stamp *timestamppb.Timestamp) string {
	if stamp == nil || !stamp.IsValid() {
		return ""
	}

	return stamp.AsTime().Format(time.RFC3339Nano)
}

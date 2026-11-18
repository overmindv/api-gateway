package graphql

import (
	"context"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	pb "github.com/overmindv/content/pkg/api/content"
)

func (r *queryResolver) Materials(ctx context.Context, filter *model.MaterialCatalogFilter, limit int, cursor *string) (*model.MaterialCatalogPage, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	req, err := materialCatalogRequest(filter, limit, cursor)
	if err != nil {
		return nil, err
	}
	page, err := client.List(ctx, actor, req)
	if err != nil {
		return nil, err
	}
	return materialCatalogModel(page), nil
}

func (r *queryResolver) Material(ctx context.Context, id string) (*model.Material, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	id, err = materialID(id)
	if err != nil {
		return nil, err
	}
	result, err := client.Get(ctx, actor, &pb.GetContentItemRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return materialResult(result.GetItem())
}

func (r *queryResolver) MaterialRevisions(ctx context.Context, id string, limit int, beforeRevision *int) (*model.MaterialRevisionPage, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	req, err := materialHistoryRequest(id, limit, beforeRevision)
	if err != nil {
		return nil, err
	}
	page, err := client.Revisions(ctx, actor, req)
	if err != nil {
		return nil, err
	}
	return materialHistoryModel(page), nil
}

func (r *queryResolver) PublishedMaterial(ctx context.Context, id string) (*model.Material, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	id, err = materialID(id)
	if err != nil {
		return nil, err
	}
	result, err := client.GetPublished(ctx, actor, &pb.GetPublishedContentItemRequest{Id: id})
	if err != nil {
		return nil, err
	}
	item := result.GetItem()
	if item.GetStatus() != "published" || item.GetPublishedRevisionId() == "" ||
		item.GetCurrentRevision().GetId() != item.GetPublishedRevisionId() {
		return nil, &contentclient.Error{Code: "NOT_FOUND"}
	}
	return materialModel(item), nil
}

func (r *mutationResolver) CreateMaterialDraft(ctx context.Context, input model.CreateMaterialDraftInput) (*model.Material, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	req, err := materialCreateRequest(input)
	if err != nil {
		return nil, err
	}
	result, err := client.CreateDraft(ctx, actor, req)
	if err != nil {
		return nil, err
	}
	return materialResult(result.GetItem())
}

func (r *mutationResolver) SaveMaterialDraft(ctx context.Context, input model.SaveMaterialDraftInput) (*model.MaterialRevision, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	req, err := materialSaveRequest(input)
	if err != nil {
		return nil, err
	}
	draft, err := materialDraftMetadata(input.Metadata)
	if err != nil {
		return nil, err
	}
	revision, err := client.SaveDraft(ctx, actor, req, draft)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}
	return materialRevisionModel(revision), nil
}

func (r *mutationResolver) PublishMaterial(ctx context.Context, id string, expectedRevisionID string) (*model.Material, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	id, err = materialID(id)
	if err != nil {
		return nil, err
	}
	expected, err := materialID(expectedRevisionID)
	if err != nil {
		return nil, err
	}
	result, err := client.Publish(ctx, actor, &pb.PublishContentItemRequest{Id: id, ExpectedRevisionId: expected})
	if err != nil {
		return nil, err
	}
	return materialResult(result.GetItem())
}

func materialResult(item *pb.ContentItem) (*model.Material, error) {
	if item == nil {
		return nil, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}
	return materialModel(item), nil
}

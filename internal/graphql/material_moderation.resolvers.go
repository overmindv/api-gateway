package graphql

import (
	"context"
	"strconv"
	"strings"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
	pb "github.com/overmindv/content/pkg/api/content"
)

func (r *mutationResolver) RestoreMaterialRevision(ctx context.Context, materialID, expectedPublishedRevisionID, sourceRevisionID string) (*model.MaterialRevision, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	if err = reviewMaterialIDs(&materialID, &expectedPublishedRevisionID, &sourceRevisionID); err != nil {
		return nil, err
	}
	result, err := client.RestoreRevision(ctx, actor, &pb.RestorePublishedRevisionRequest{
		ContentItemId: materialID, ExpectedPublishedRevisionId: expectedPublishedRevisionID, SourceRevisionId: sourceRevisionID,
	})
	if err != nil {
		return nil, err
	}
	revision := result.GetRevision()
	if revision.GetContentItemId() != materialID || revision.GetRestoredFromRevisionId() != sourceRevisionID {
		return nil, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}

	return materialRevisionModel(revision), nil
}

func (r *queryResolver) MaterialModeration(ctx context.Context, id string) (*model.MaterialModerationState, error) {
	client, actor, err := r.materialModerator(ctx)
	if err != nil {
		return nil, err
	}
	id, err = materialID(id)
	if err != nil {
		return nil, err
	}
	result, err := client.GetModeration(ctx, actor, &pb.GetMaterialModerationRequest{MaterialId: id})
	if err != nil {
		return nil, err
	}

	return moderationStateModel(id, result.GetState())
}

func (r *mutationResolver) SetMaterialModeration(ctx context.Context, input model.SetMaterialModerationInput) (*model.MaterialModerationState, error) {
	client, actor, err := r.materialModerator(ctx)
	if err != nil {
		return nil, err
	}
	req, err := moderationRequest(input)
	if err != nil {
		return nil, err
	}
	result, err := client.SetModeration(ctx, actor, req)
	if err != nil {
		return nil, err
	}

	return moderationStateModel(req.GetMaterialId(), result.GetState())
}

func (r *Resolver) materialModerator(ctx context.Context) (contentclient.ModerationService, contentclient.Actor, error) {
	auth, err := middleware.RequireVerifiedAdmin(ctx)
	if err != nil {
		return nil, contentclient.Actor{}, err
	}
	if r.Content == nil {
		return nil, contentclient.Actor{}, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}

	return r.Content, contentclient.Actor{UserID: auth.UserID, RequestID: middleware.RequestID(ctx)}, nil
}

func moderationRequest(input model.SetMaterialModerationInput) (*pb.SetMaterialModerationRequest, error) {
	if err := reviewMaterialIDs(&input.MaterialID, &input.ExpectedPublishedRevisionID); err != nil {
		return nil, err
	}
	version, err := strconv.ParseInt(input.ExpectedVersion, 10, 64)
	if err != nil || version < 0 || strconv.FormatInt(version, 10) != input.ExpectedVersion ||
		len(input.Reason) > 1024 || strings.TrimSpace(input.Reason) == "" {
		return nil, invalidMaterialInput()
	}
	var action pb.MaterialModerationAction
	switch input.Action {
	case model.MaterialModerationActionHide:
		action = pb.MaterialModerationAction_MATERIAL_MODERATION_ACTION_HIDE
	case model.MaterialModerationActionRestore:
		action = pb.MaterialModerationAction_MATERIAL_MODERATION_ACTION_RESTORE
	default:
		return nil, invalidMaterialInput()
	}

	return &pb.SetMaterialModerationRequest{
		MaterialId: input.MaterialID, ExpectedPublishedRevisionId: input.ExpectedPublishedRevisionID, ExpectedModerationVersion: &version,
		Action: action, Reason: input.Reason,
	}, nil
}

func moderationStateModel(id string, state *pb.MaterialModerationState) (*model.MaterialModerationState, error) {
	revisionID, err := materialID(state.GetPublishedRevisionId())
	if err != nil || state.GetMaterialId() != id || state.GetVersion() < 0 {
		return nil, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}

	return &model.MaterialModerationState{
		MaterialID: id, PublishedRevisionID: revisionID, Version: strconv.FormatInt(state.GetVersion(), 10), Hidden: state.GetHidden(),
	}, nil
}

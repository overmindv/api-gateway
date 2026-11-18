package graphql

import (
	"context"
	"math"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	pb "github.com/overmindv/content/pkg/api/content"
)

func (r *queryResolver) MaterialChangeProposal(ctx context.Context, id string) (*model.MaterialChangeProposal, error) {
	id, err := materialID(id)
	return resolveMaterialReview(ctx, r.Resolver, &pb.GetChangeProposalRequest{ProposalId: id}, err,
		contentclient.Service.GetProposal, func(resp *pb.GetChangeProposalResponse) *model.MaterialChangeProposal {
			return materialChangeModel(resp.GetProposal())
		})
}

func (r *queryResolver) MaterialChangeHistory(ctx context.Context, proposalID string, limit int, beforeOrdinal *int) (*model.MaterialChangeHistoryPage, error) {
	cursor, err := materialReviewOrdinal(limit, beforeOrdinal)
	if err == nil {
		err = normalizeMaterialIDs(&proposalID)
	}
	req := &pb.ListChangeProposalHistoryRequest{ProposalId: proposalID, PageSize: int32(limit), BeforeOrdinal: cursor}
	return resolveMaterialReview(ctx, r.Resolver, req, err, contentclient.Service.ProposalHistory, changeHistoryModel)
}

func (r *queryResolver) MaterialChangeDiff(ctx context.Context, proposalID string, revisionID *string) (*model.MaterialChangeDiff, error) {
	revision, err := optionalMaterialID(revisionID)
	if err == nil {
		err = normalizeMaterialIDs(&proposalID)
	}
	req := &pb.GetChangeProposalDiffRequest{ProposalId: proposalID, RevisionId: revision}
	return resolveMaterialReview(ctx, r.Resolver, req, err, contentclient.Service.ProposalDiff, changeDiffModel)
}

func (r *queryResolver) MaterialReviewStatus(ctx context.Context, proposalID string, revisionID *string) (*model.MaterialReviewStatus, error) {
	revision, err := optionalMaterialID(revisionID)
	if err == nil {
		err = normalizeMaterialIDs(&proposalID)
	}
	req := &pb.GetReviewRevisionStatusRequest{ProposalId: proposalID, RevisionId: revision}
	return resolveMaterialReview(ctx, r.Resolver, req, err, contentclient.Service.ReviewStatus, materialReviewStatusModel)
}

func (r *queryResolver) MaterialReviewThreads(ctx context.Context, proposalID string, limit int, afterID *string) (*model.MaterialReviewThreadPage, error) {
	cursor, err := materialReviewPage(limit, afterID)
	if err == nil {
		err = normalizeMaterialIDs(&proposalID)
	}
	req := &pb.ListReviewThreadsRequest{ProposalId: proposalID, PageSize: int32(limit), AfterId: cursor}
	return resolveMaterialReview(ctx, r.Resolver, req, err, contentclient.Service.ReviewThreads, reviewThreadPageModel)
}

func (r *queryResolver) MaterialReviewReplies(ctx context.Context, proposalID string, threadID string, limit int, afterID *string) (*model.MaterialReviewReplyPage, error) {
	thread, err := reviewNumericID(threadID, false)
	var cursor int64
	if err == nil {
		cursor, err = materialReviewPage(limit, afterID)
	}
	if err == nil {
		err = normalizeMaterialIDs(&proposalID)
	}
	req := &pb.ListReviewRepliesRequest{ProposalId: proposalID, ThreadId: thread, PageSize: int32(limit), AfterId: cursor}
	return resolveMaterialReview(ctx, r.Resolver, req, err, contentclient.Service.ReviewReplies, reviewReplyPageModel)
}

func (r *mutationResolver) OpenMaterialChange(ctx context.Context, input model.OpenMaterialChangeInput) (*model.MaterialChangeProposal, error) {
	assets, err := prepareMaterialChange(input.Source, input.Message, input.Attachments, &input.MaterialID, &input.BaseRevisionID)
	req := &pb.OpenChangeProposalRequest{
		ContentItemId: input.MaterialID, BaseRevisionId: input.BaseRevisionID, Source: input.Source, Message: input.Message, Attachments: assets,
	}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.OpenProposal, func(resp *pb.OpenChangeProposalResponse) *model.MaterialChangeProposal {
			return materialChangeModel(resp.GetProposal())
		})
}

func (r *mutationResolver) AppendMaterialChange(ctx context.Context, input model.AppendMaterialChangeInput) (*model.MaterialChangeProposal, error) {
	assets, err := prepareMaterialChange(input.Source, input.Message, input.Attachments, &input.ProposalID, &input.ExpectedHeadID)
	req := &pb.AppendChangeProposalRevisionRequest{
		ProposalId: input.ProposalID, ExpectedHeadId: input.ExpectedHeadID, Source: input.Source, Message: input.Message, Attachments: assets,
	}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.AppendProposal, func(resp *pb.AppendChangeProposalRevisionResponse) *model.MaterialChangeProposal {
			return materialChangeModel(resp.GetProposal())
		})
}

func (r *mutationResolver) RebaseMaterialChange(ctx context.Context, input model.RebaseMaterialChangeInput) (*model.MaterialChangeProposal, error) {
	assets, err := prepareMaterialChange(input.Source, input.Message, input.Attachments, &input.ProposalID, &input.ExpectedHeadID, &input.BaseRevisionID)
	req := &pb.RebaseChangeProposalRequest{
		ProposalId: input.ProposalID, ExpectedHeadId: input.ExpectedHeadID, BaseRevisionId: input.BaseRevisionID,
		Source: input.Source, Message: input.Message, Attachments: assets,
	}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.RebaseProposal, func(resp *pb.RebaseChangeProposalResponse) *model.MaterialChangeProposal {
			return materialChangeModel(resp.GetProposal())
		})
}

func prepareMaterialChange(source, message string, attachments []*model.MaterialAttachmentInput, ids ...*string) (*pb.AttachmentSet, error) {
	if err := normalizeMaterialIDs(ids...); err != nil {
		return nil, err
	}
	if !validDraftText(source, message) {
		return nil, invalidMaterialInput()
	}
	return proposalAttachmentSet(attachments)
}

func (r *mutationResolver) CloseMaterialChange(ctx context.Context, proposalID string) (*model.MaterialChangeProposal, error) {
	proposalID, err := materialID(proposalID)
	return resolveMaterialReview(ctx, r.Resolver, &pb.CloseChangeProposalRequest{ProposalId: proposalID}, err,
		contentclient.Service.CloseProposal, func(resp *pb.CloseChangeProposalResponse) *model.MaterialChangeProposal {
			return materialChangeModel(resp.GetProposal())
		})
}

func (r *mutationResolver) MergeMaterialChange(ctx context.Context, proposalID string, expectedHeadID string) (*model.MaterialMergeResult, error) {
	err := normalizeMaterialIDs(&proposalID, &expectedHeadID)
	req := &pb.MergeChangeProposalRequest{ProposalId: proposalID, ExpectedHeadId: expectedHeadID}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.MergeProposal, func(resp *pb.MergeChangeProposalResponse) *model.MaterialMergeResult {
			if resp.GetContentItemId() == "" || resp.GetPublishedRevisionId() == "" {
				return nil
			}
			return &model.MaterialMergeResult{MaterialID: resp.ContentItemId, PublishedRevisionID: resp.PublishedRevisionId}
		})
}

func (r *mutationResolver) SubmitMaterialReview(ctx context.Context, input model.SubmitMaterialReviewInput) (*model.MaterialReviewDecision, error) {
	err := normalizeMaterialIDs(&input.ProposalID, &input.RevisionID)
	if err == nil && len(input.Reason) > 4096 {
		err = invalidMaterialInput()
	}
	req := &pb.SubmitReviewDecisionRequest{
		ProposalId: input.ProposalID, RevisionId: input.RevisionID, Decision: string(input.Decision), Reason: input.Reason,
	}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.SubmitReview, func(resp *pb.SubmitReviewDecisionResponse) *model.MaterialReviewDecision {
			return reviewDecisionModel(resp.GetDecision())
		})
}

func (r *mutationResolver) CreateMaterialReviewThread(ctx context.Context, input model.CreateMaterialReviewThreadInput) (*model.MaterialReviewThread, error) {
	err := normalizeMaterialIDs(&input.ProposalID, &input.RevisionID)
	if err == nil && (input.LineNumber < 0 || input.LineNumber > math.MaxInt32 || len(input.Body) > 4096) {
		err = invalidMaterialInput()
	}
	req := &pb.CreateReviewThreadRequest{
		ProposalId: input.ProposalID, RevisionId: input.RevisionID, LineNumber: int32(input.LineNumber), Body: input.Body,
	}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.CreateThread, func(resp *pb.CreateReviewThreadResponse) *model.MaterialReviewThread {
			return reviewThreadModel(resp.GetThread())
		})
}

func (r *mutationResolver) ReplyToMaterialReviewThread(ctx context.Context, proposalID string, threadID string, body string) (*model.MaterialReviewReply, error) {
	thread, err := reviewNumericID(threadID, false)
	if err == nil {
		err = normalizeMaterialIDs(&proposalID)
	}
	if err == nil && len(body) > 4096 {
		err = invalidMaterialInput()
	}
	req := &pb.ReplyToReviewThreadRequest{ProposalId: proposalID, ThreadId: thread, Body: body}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.ReplyToThread, func(resp *pb.ReplyToReviewThreadResponse) *model.MaterialReviewReply {
			return reviewReplyModel(resp.GetReply())
		})
}

func (r *mutationResolver) SetMaterialReviewThreadResolved(ctx context.Context, proposalID string, threadID string, resolved bool) (*model.MaterialReviewThread, error) {
	thread, err := reviewNumericID(threadID, false)
	if err == nil {
		err = normalizeMaterialIDs(&proposalID)
	}
	req := &pb.SetReviewThreadResolvedRequest{ProposalId: proposalID, ThreadId: thread, Resolved: resolved}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.SetThreadResolved, func(resp *pb.SetReviewThreadResolvedResponse) *model.MaterialReviewThread {
			return reviewThreadModel(resp.GetThread())
		})
}

func (r *mutationResolver) RestoreMaterialRevision(ctx context.Context, materialID string, expectedPublishedRevisionID string, sourceRevisionID string) (*model.MaterialRevision, error) {
	err := normalizeMaterialIDs(&materialID, &expectedPublishedRevisionID, &sourceRevisionID)
	req := &pb.RestorePublishedRevisionRequest{
		ContentItemId: materialID, ExpectedPublishedRevisionId: expectedPublishedRevisionID, SourceRevisionId: sourceRevisionID,
	}
	return resolveMaterialReview(ctx, r.Resolver, req, err,
		contentclient.Service.RestoreRevision, func(resp *pb.RestorePublishedRevisionResponse) *model.MaterialRevision {
			return materialRevisionModel(resp.GetRevision())
		})
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
	state, err := client.GetModeration(ctx, actor, &pb.GetMaterialModerationRequest{MaterialId: id})
	if err != nil {
		return nil, err
	}
	return moderationStateModel(state)
}

func (r *mutationResolver) SetMaterialModeration(ctx context.Context, input model.SetMaterialModerationInput) (*model.MaterialModerationState, error) {
	client, actor, err := r.materialModerator(ctx)
	if err != nil {
		return nil, err
	}
	if err := normalizeMaterialIDs(&input.MaterialID, &input.ExpectedPublishedRevisionID); err != nil {
		return nil, err
	}
	version, err := reviewNumericID(input.ExpectedVersion, true)
	if err != nil || len(input.Reason) > 1024 {
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
	state, err := client.SetModeration(ctx, actor, &pb.SetMaterialModerationRequest{
		MaterialId: input.MaterialID, ExpectedPublishedRevisionId: input.ExpectedPublishedRevisionID,
		ExpectedModerationVersion: &version, Action: action, Reason: input.Reason,
	})
	if err != nil {
		return nil, err
	}
	return moderationStateModel(state)
}

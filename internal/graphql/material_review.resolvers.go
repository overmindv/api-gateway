package graphql

import (
	"context"
	"math"
	"strconv"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	pb "github.com/overmindv/content/pkg/api/content"
)

func (r *queryResolver) MaterialReviewStatus(ctx context.Context, proposalID string, revisionID *string) (*model.MaterialReviewStatus, error) {
	req := &pb.GetReviewRevisionStatusRequest{}
	err := reviewMaterialIDs(&proposalID)
	if err == nil && revisionID != nil {
		req.RevisionId, err = materialID(*revisionID)
	}
	req.ProposalId = proposalID

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.ReviewStatus, reviewStatusModel)
}

func (r *queryResolver) MaterialReviewThreads(ctx context.Context, proposalID string, limit int, afterID *string) (*model.MaterialReviewThreadPage, error) {
	size, cursor, err := reviewPage(limit, afterID)
	if err == nil {
		err = reviewMaterialIDs(&proposalID)
	}
	req := &pb.ListReviewThreadsRequest{ProposalId: proposalID, PageSize: size, AfterId: cursor}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.ReviewThreads, reviewThreadPageModel)
}

func (r *queryResolver) MaterialReviewReplies(ctx context.Context, proposalID string, threadID string, limit int, afterID *string) (*model.MaterialReviewReplyPage, error) {
	size, cursor, err := reviewPage(limit, afterID)
	var thread int64
	if err == nil {
		thread, err = reviewNumericID(threadID)
	}
	if err == nil {
		err = reviewMaterialIDs(&proposalID)
	}
	req := &pb.ListReviewRepliesRequest{ProposalId: proposalID, ThreadId: thread, PageSize: size, AfterId: cursor}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.ReviewReplies, reviewReplyPageModel)
}

func (r *mutationResolver) RebaseMaterialChange(ctx context.Context, input model.RebaseMaterialChangeInput) (*model.MaterialChangeProposal, error) {
	id, head, err := proposalWriteInput(input.ProposalID, input.ExpectedHeadID, input.Source, input.Message)
	req := &pb.RebaseChangeProposalRequest{
		ProposalId: id, ExpectedHeadId: head, Source: input.Source, Message: input.Message,
	}
	if err == nil {
		req.BaseRevisionId, err = materialID(input.BaseRevisionID)
	}
	if err == nil {
		req.Attachments, err = proposalAttachments(input.Attachments)
	}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.RebaseProposal,
		func(resp *pb.RebaseChangeProposalResponse) *model.MaterialChangeProposal {
			return proposalModel(resp.GetProposal())
		})
}

func (r *mutationResolver) MergeMaterialChange(ctx context.Context, proposalID string, expectedHeadID string) (*model.MaterialMergeResult, error) {
	err := reviewMaterialIDs(&proposalID, &expectedHeadID)
	req := &pb.MergeChangeProposalRequest{ProposalId: proposalID, ExpectedHeadId: expectedHeadID}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.MergeProposal,
		func(resp *pb.MergeChangeProposalResponse) *model.MaterialMergeResult {
			if resp.GetContentItemId() == "" || resp.GetPublishedRevisionId() == "" {
				return nil
			}

			return &model.MaterialMergeResult{MaterialID: resp.GetContentItemId(), PublishedRevisionID: resp.GetPublishedRevisionId()}
		})
}

func (r *mutationResolver) SubmitMaterialReview(ctx context.Context, input model.SubmitMaterialReviewInput) (*model.MaterialReviewDecision, error) {
	err := reviewMaterialIDs(&input.ProposalID, &input.RevisionID)
	if err == nil && len(input.Reason) > 4096 {
		err = invalidMaterialInput()
	}
	req := &pb.SubmitReviewDecisionRequest{
		ProposalId: input.ProposalID, RevisionId: input.RevisionID, Decision: string(input.Decision), Reason: input.Reason,
	}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.SubmitReview,
		func(resp *pb.SubmitReviewDecisionResponse) *model.MaterialReviewDecision {
			return reviewDecisionModel(resp.GetDecision())
		})
}

func (r *mutationResolver) CreateMaterialReviewThread(ctx context.Context, input model.CreateMaterialReviewThreadInput) (*model.MaterialReviewThread, error) {
	err := reviewMaterialIDs(&input.ProposalID, &input.RevisionID)
	if err == nil && len(input.Body) > 4096 {
		err = invalidMaterialInput()
	}
	req := &pb.CreateReviewThreadRequest{ProposalId: input.ProposalID, RevisionId: input.RevisionID,
		Body: input.Body}
	if line := input.LineNumber; line >= 0 && line <= math.MaxInt32 {
		req.LineNumber = int32(line)
	} else if err == nil {
		err = invalidMaterialInput()
	}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.CreateThread,
		func(resp *pb.CreateReviewThreadResponse) *model.MaterialReviewThread {
			return reviewThreadModel(resp.GetThread())
		})
}

func (r *mutationResolver) ReplyToMaterialReviewThread(ctx context.Context, proposalID string, threadID string, body string) (*model.MaterialReviewReply, error) {
	thread, err := reviewNumericID(threadID)
	if err == nil {
		err = reviewMaterialIDs(&proposalID)
	}
	if err == nil && len(body) > 4096 {
		err = invalidMaterialInput()
	}
	req := &pb.ReplyToReviewThreadRequest{ProposalId: proposalID, ThreadId: thread, Body: body}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.ReplyToThread,
		func(resp *pb.ReplyToReviewThreadResponse) *model.MaterialReviewReply {
			return reviewReplyModel(resp.GetReply())
		})
}

func (r *mutationResolver) SetMaterialReviewThreadResolved(ctx context.Context, proposalID string, threadID string, resolved bool) (*model.MaterialReviewThread, error) {
	thread, err := reviewNumericID(threadID)
	if err == nil {
		err = reviewMaterialIDs(&proposalID)
	}
	req := &pb.SetReviewThreadResolvedRequest{ProposalId: proposalID, ThreadId: thread, Resolved: resolved}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.SetThreadResolved,
		func(resp *pb.SetReviewThreadResolvedResponse) *model.MaterialReviewThread {
			return reviewThreadModel(resp.GetThread())
		})
}

func reviewMaterialIDs(ids ...*string) error {
	for _, raw := range ids {
		id, err := materialID(*raw)
		if err != nil {
			return err
		}
		*raw = id
	}

	return nil
}

func reviewNumericID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != raw {
		return 0, invalidMaterialInput()
	}

	return id, nil
}

func reviewPage(limit int, afterID *string) (int32, int64, error) {
	if limit < 1 || limit > 50 {
		return 0, 0, invalidMaterialInput()
	}
	size := int32(limit)
	if afterID == nil {
		return size, 0, nil
	}

	cursor, err := reviewNumericID(*afterID)

	return size, cursor, err
}

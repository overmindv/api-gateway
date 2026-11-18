package graphql

import (
	"context"
	"math"
	"strconv"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
	pb "github.com/overmindv/content/pkg/api/content"
)

func resolveMaterialReview[Req, Resp, View any](ctx context.Context, r *Resolver, req *Req, invalid error,
	invoke func(contentclient.Service, context.Context, contentclient.Actor, *Req) (*Resp, error),
	mapView func(*Resp) *View) (*View, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	if invalid != nil {
		return nil, invalid
	}
	resp, err := invoke(client, ctx, actor, req)
	if err != nil {
		return nil, err
	}
	view := mapView(resp)
	if view == nil {
		return nil, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}
	return view, nil
}

func (r *Resolver) materialModerator(ctx context.Context) (contentclient.Service, contentclient.Actor, error) {
	auth, err := middleware.RequireVerifiedAdmin(ctx)
	if err != nil {
		return nil, contentclient.Actor{}, err
	}
	if r.Content == nil {
		return nil, contentclient.Actor{}, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}
	return r.Content, contentclient.Actor{UserID: auth.UserID, RequestID: middleware.RequestID(ctx)}, nil
}

func normalizeMaterialIDs(ids ...*string) error {
	for _, raw := range ids {
		id, err := materialID(*raw)
		if err != nil {
			return err
		}
		*raw = id
	}
	return nil
}

func materialReviewOrdinal(limit int, before *int) (int32, error) {
	if limit < 1 || limit > 50 {
		return 0, invalidMaterialInput()
	}
	if before == nil {
		return 0, nil
	}
	if *before < 1 || *before > math.MaxInt32 {
		return 0, invalidMaterialInput()
	}
	return int32(*before), nil
}

func reviewNumericID(raw string, allowZero bool) (int64, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 || (n == 0 && !allowZero) || strconv.FormatInt(n, 10) != raw {
		return 0, invalidMaterialInput()
	}
	return n, nil
}

func materialReviewPage(limit int, afterID *string) (int64, error) {
	if limit < 1 || limit > 50 {
		return 0, invalidMaterialInput()
	}
	if afterID == nil {
		return 0, nil
	}
	return reviewNumericID(*afterID, false)
}

func reviewPageCursor(after int64) *string {
	if after == 0 {
		return nil
	}
	return materialOptional(strconv.FormatInt(after, 10))
}

func optionalMaterialID(value *string) (string, error) {
	if value == nil {
		return "", nil
	}
	return materialID(*value)
}

func proposalAttachmentSet(input []*model.MaterialAttachmentInput) (*pb.AttachmentSet, error) {
	assets, err := materialAttachments(input)
	if err != nil || input == nil {
		return nil, err
	}
	return &pb.AttachmentSet{Assets: assets}, nil
}

func materialChangeModel(item *pb.ChangeProposal) *model.MaterialChangeProposal {
	if item == nil || item.Head == nil {
		return nil
	}
	return &model.MaterialChangeProposal{
		ID: item.Id, MaterialID: item.ContentItemId, BaseRevisionID: item.BaseRevisionId, AuthorID: item.AuthorId,
		Status: model.MaterialChangeStatus(item.Status), CreatedAt: materialTimestamp(item.CreatedAt),
		ClosedAt: materialOptional(materialTimestamp(item.ClosedAt)), BaseOutdated: item.BaseOutdated,
		MergedRevisionID: materialOptional(item.MergedRevisionId),
		Head: &model.MaterialChangeRevision{
			ID: item.Head.Id, ParentRevisionID: materialOptional(item.Head.ParentRevisionId), Ordinal: int(item.Head.Ordinal),
			Source: item.Head.Source, Message: item.Head.Message, CreatedAt: materialTimestamp(item.Head.CreatedAt),
			Attachments: materialAttachmentModels(item.Head.Assets),
		},
	}
}

func changeHistoryModel(resp *pb.ListChangeProposalHistoryResponse) *model.MaterialChangeHistoryPage {
	items := make([]*model.MaterialChangeHistoryEntry, 0, len(resp.GetEntries()))
	for _, entry := range resp.GetEntries() {
		items = append(items, &model.MaterialChangeHistoryEntry{
			ID: entry.GetId(), ParentRevisionID: materialOptional(entry.GetParentRevisionId()), BaseRevisionID: entry.GetBaseRevisionId(),
			Ordinal: int(entry.GetOrdinal()), AuthorID: entry.GetAuthorId(), Message: entry.GetMessage(),
			CreatedAt: materialTimestamp(entry.GetCreatedAt()),
		})
	}
	page := &model.MaterialChangeHistoryPage{Items: items}
	if next := resp.GetNextBeforeOrdinal(); next != 0 {
		ordinal := int(next)
		page.NextBeforeOrdinal = &ordinal
	}
	return page
}

func changeDiffModel(resp *pb.GetChangeProposalDiffResponse) *model.MaterialChangeDiff {
	if resp == nil {
		return nil
	}
	return &model.MaterialChangeDiff{
		BaseRevisionID: resp.BaseRevisionId, RevisionID: resp.RevisionId,
		UnifiedDiff: resp.UnifiedDiff, Truncated: resp.Truncated,
	}
}

func reviewDecisionModel(decision *pb.ReviewDecision) *model.MaterialReviewDecision {
	if decision == nil {
		return nil
	}
	return &model.MaterialReviewDecision{
		ReviewerID: decision.ReviewerId, Decision: model.MaterialReviewDecisionKind(decision.Decision),
		Reason: decision.Reason, CreatedAt: materialTimestamp(decision.CreatedAt), Legacy: decision.Legacy,
	}
}

func materialReviewStatusModel(result *pb.GetReviewRevisionStatusResponse) *model.MaterialReviewStatus {
	if result == nil {
		return nil
	}
	decisions := make([]*model.MaterialReviewDecision, 0, len(result.Decisions))
	for _, decision := range result.Decisions {
		decisions = append(decisions, reviewDecisionModel(decision))
	}
	assignments := make([]*model.MaterialReviewAssignment, 0, len(result.Assignments))
	for _, assignment := range result.Assignments {
		assignments = append(assignments, &model.MaterialReviewAssignment{
			ReviewerID: assignment.GetReviewerId(), Slot: int(assignment.GetSlot()),
		})
	}
	return &model.MaterialReviewStatus{
		RevisionID: result.RevisionId, Current: result.Current, Decisions: decisions, Assignments: assignments,
		Outcome: model.MaterialReviewOutcome(result.Outcome), WaitReason: materialOptional(result.WaitReason),
		TopicID: materialOptional(result.TopicId),
	}
}

func reviewThreadModel(thread *pb.ReviewThread) *model.MaterialReviewThread {
	if thread == nil {
		return nil
	}
	return &model.MaterialReviewThread{
		ID: strconv.FormatInt(thread.Id, 10), RevisionID: thread.RevisionId, LineNumber: int(thread.LineNumber),
		AuthorID: thread.AuthorId, Body: thread.Body, CreatedAt: materialTimestamp(thread.CreatedAt), Resolved: thread.Resolved,
		ResolvedAt: materialOptional(materialTimestamp(thread.ResolvedAt)), ResolvedBy: materialOptional(thread.ResolvedBy),
	}
}

func reviewReplyModel(reply *pb.ReviewReply) *model.MaterialReviewReply {
	if reply == nil {
		return nil
	}
	return &model.MaterialReviewReply{
		ID: strconv.FormatInt(reply.Id, 10), AuthorID: reply.AuthorId, Body: reply.Body, CreatedAt: materialTimestamp(reply.CreatedAt),
	}
}

func reviewThreadPageModel(page *pb.ListReviewThreadsResponse) *model.MaterialReviewThreadPage {
	items := make([]*model.MaterialReviewThread, 0, len(page.GetThreads()))
	for _, thread := range page.GetThreads() {
		items = append(items, reviewThreadModel(thread))
	}
	return &model.MaterialReviewThreadPage{Items: items, NextAfterID: reviewPageCursor(page.GetNextAfterId())}
}

func reviewReplyPageModel(page *pb.ListReviewRepliesResponse) *model.MaterialReviewReplyPage {
	items := make([]*model.MaterialReviewReply, 0, len(page.GetReplies()))
	for _, reply := range page.GetReplies() {
		items = append(items, reviewReplyModel(reply))
	}
	return &model.MaterialReviewReplyPage{Items: items, NextAfterID: reviewPageCursor(page.GetNextAfterId())}
}

func moderationStateModel(state *pb.MaterialModerationState) (*model.MaterialModerationState, error) {
	if state == nil {
		return nil, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}
	return &model.MaterialModerationState{
		MaterialID: state.MaterialId, PublishedRevisionID: state.PublishedRevisionId,
		Version: strconv.FormatInt(state.Version, 10), Hidden: state.Hidden,
	}, nil
}

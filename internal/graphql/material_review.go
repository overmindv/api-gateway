package graphql

import (
	"strconv"

	"github.com/overmindv/api-gateway/internal/graphql/model"
	pb "github.com/overmindv/content/pkg/api/content"
)

func reviewStatusModel(result *pb.GetReviewRevisionStatusResponse) *model.MaterialReviewStatus {
	outcome := model.MaterialReviewOutcome(result.GetOutcome())
	if !outcome.IsValid() {
		return nil
	}
	decisions := make([]*model.MaterialReviewDecision, 0, len(result.GetDecisions()))
	for _, decision := range result.GetDecisions() {
		if view := reviewDecisionModel(decision); view != nil {
			decisions = append(decisions, view)
		}
	}
	assignments := make([]*model.MaterialReviewAssignment, 0, len(result.GetAssignments()))
	for _, assignment := range result.GetAssignments() {
		assignments = append(assignments, &model.MaterialReviewAssignment{
			ReviewerID: assignment.GetReviewerId(), Slot: int(assignment.GetSlot()),
		})
	}

	return &model.MaterialReviewStatus{
		RevisionID: result.GetRevisionId(), Current: result.GetCurrent(), Decisions: decisions, Assignments: assignments,
		Outcome: outcome, WaitReason: materialOptional(result.GetWaitReason()), TopicID: materialOptional(result.GetTopicId()),
	}
}

func reviewDecisionModel(decision *pb.ReviewDecision) *model.MaterialReviewDecision {
	kind := model.MaterialReviewDecisionKind(decision.GetDecision())
	if !kind.IsValid() {
		return nil
	}

	return &model.MaterialReviewDecision{
		ReviewerID: decision.GetReviewerId(), Decision: kind, Reason: decision.GetReason(),
		CreatedAt: materialTimestamp(decision.GetCreatedAt()), Legacy: decision.GetLegacy(),
	}
}

func reviewThreadModel(thread *pb.ReviewThread) *model.MaterialReviewThread {
	if thread.GetId() <= 0 {
		return nil
	}

	return &model.MaterialReviewThread{
		ID: strconv.FormatInt(thread.GetId(), 10), RevisionID: thread.GetRevisionId(),
		LineNumber: int(thread.GetLineNumber()), AuthorID: thread.GetAuthorId(), Body: thread.GetBody(),
		CreatedAt: materialTimestamp(thread.GetCreatedAt()), Resolved: thread.GetResolved(),
		ResolvedAt: materialOptional(materialTimestamp(thread.GetResolvedAt())), ResolvedBy: materialOptional(thread.GetResolvedBy()),
	}
}

func reviewReplyModel(reply *pb.ReviewReply) *model.MaterialReviewReply {
	if reply.GetId() <= 0 {
		return nil
	}

	return &model.MaterialReviewReply{
		ID: strconv.FormatInt(reply.GetId(), 10), AuthorID: reply.GetAuthorId(), Body: reply.GetBody(),
		CreatedAt: materialTimestamp(reply.GetCreatedAt()),
	}
}

func reviewThreadPageModel(page *pb.ListReviewThreadsResponse) *model.MaterialReviewThreadPage {
	items := make([]*model.MaterialReviewThread, 0, len(page.GetThreads()))
	for _, thread := range page.GetThreads() {
		if view := reviewThreadModel(thread); view != nil {
			items = append(items, view)
		}
	}

	return &model.MaterialReviewThreadPage{Items: items, NextAfterID: reviewPageCursor(page.GetNextAfterId())}
}

func reviewReplyPageModel(page *pb.ListReviewRepliesResponse) *model.MaterialReviewReplyPage {
	items := make([]*model.MaterialReviewReply, 0, len(page.GetReplies()))
	for _, reply := range page.GetReplies() {
		if view := reviewReplyModel(reply); view != nil {
			items = append(items, view)
		}
	}

	return &model.MaterialReviewReplyPage{Items: items, NextAfterID: reviewPageCursor(page.GetNextAfterId())}
}

func reviewPageCursor(id int64) *string {
	if id <= 0 {
		return nil
	}

	return materialOptional(strconv.FormatInt(id, 10))
}

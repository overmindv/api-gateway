package graphql

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	reviewThreadID   = 9007199254740993
	reviewThreadText = "9007199254740993"
	reviewReplyText  = "9007199254740994"
	waitingMergeID   = "00000000-0000-4000-8000-000000000201"
	blockedMergeID   = "00000000-0000-4000-8000-000000000202"
	conflictedMerge  = "00000000-0000-4000-8000-000000000203"
)

func (materialRPCFixture) GetReviewRevisionStatus(ctx context.Context, req *pb.GetReviewRevisionStatusRequest) (*pb.GetReviewRevisionStatusResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetProposalId() != proposalFixtureID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}

	return &pb.GetReviewRevisionStatusResponse{
		RevisionId: req.GetRevisionId(), Current: req.GetRevisionId() == proposalHeadID, Outcome: "pending",
		Assignments: []*pb.ReviewAssignment{{ReviewerId: materialOwner, Slot: 1}},
		Decisions:   []*pb.ReviewDecision{{ReviewerId: materialReader, Decision: "approve", Legacy: true}},
	}, nil
}

func (materialRPCFixture) SubmitReviewDecision(ctx context.Context, req *pb.SubmitReviewDecisionRequest) (*pb.SubmitReviewDecisionResponse, error) {
	if materialRPCActor(ctx) != materialOwner {
		return nil, status.Error(codes.PermissionDenied, proposalDetail)
	}
	if req.GetRevisionId() != proposalHeadID {
		return nil, status.Error(codes.Aborted, proposalDetail)
	}

	return &pb.SubmitReviewDecisionResponse{
		Decision: &pb.ReviewDecision{ReviewerId: materialOwner, Decision: req.GetDecision(), Reason: req.GetReason()},
	}, nil
}

func fixtureReviewThread() *pb.ReviewThread {
	return &pb.ReviewThread{Id: reviewThreadID, RevisionId: proposalHeadID, LineNumber: 7, AuthorId: materialOwner, Body: "Question"}
}

func (materialRPCFixture) CreateReviewThread(ctx context.Context, req *pb.CreateReviewThreadRequest) (*pb.CreateReviewThreadResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetRevisionId() != proposalHeadID || req.GetLineNumber() != 7 {
		return nil, status.Error(codes.InvalidArgument, proposalDetail)
	}

	return &pb.CreateReviewThreadResponse{Thread: fixtureReviewThread()}, nil
}

func (materialRPCFixture) ListReviewThreads(ctx context.Context, req *pb.ListReviewThreadsRequest) (*pb.ListReviewThreadsResponse, error) {
	if materialRPCActor(ctx) != materialOwner {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	if req.GetPageSize() != 2 || req.GetAfterId() != reviewThreadID-1 {
		return nil, status.Error(codes.InvalidArgument, proposalDetail)
	}

	return &pb.ListReviewThreadsResponse{Threads: []*pb.ReviewThread{fixtureReviewThread()}, NextAfterId: reviewThreadID}, nil
}

func (materialRPCFixture) ReplyToReviewThread(ctx context.Context, req *pb.ReplyToReviewThreadRequest) (*pb.ReplyToReviewThreadResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetThreadId() != reviewThreadID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}

	return &pb.ReplyToReviewThreadResponse{Reply: &pb.ReviewReply{Id: reviewThreadID + 1, AuthorId: materialOwner, Body: req.GetBody()}}, nil
}

func (materialRPCFixture) ListReviewReplies(ctx context.Context, req *pb.ListReviewRepliesRequest) (*pb.ListReviewRepliesResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetThreadId() != reviewThreadID || req.GetAfterId() != reviewThreadID+1 {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}

	return &pb.ListReviewRepliesResponse{Replies: []*pb.ReviewReply{{Id: reviewThreadID + 1, AuthorId: materialOwner, Body: "Reply"}}}, nil
}

func (materialRPCFixture) SetReviewThreadResolved(ctx context.Context, req *pb.SetReviewThreadResolvedRequest) (*pb.SetReviewThreadResolvedResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetThreadId() != reviewThreadID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	thread := fixtureReviewThread()
	thread.Resolved = req.GetResolved()

	return &pb.SetReviewThreadResolvedResponse{Thread: thread}, nil
}

func (materialRPCFixture) RebaseChangeProposal(ctx context.Context, req *pb.RebaseChangeProposalRequest) (*pb.RebaseChangeProposalResponse, error) {
	if materialRPCActor(ctx) != materialReader {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	if req.GetExpectedHeadId() != proposalHeadID || req.GetBaseRevisionId() != materialPublishedID || req.GetAttachments() == nil ||
		len(req.GetAttachments().GetAssets()) != 0 {
		return nil, status.Error(codes.InvalidArgument, proposalDetail)
	}
	p := proposalFixture(req.GetSource())
	p.Head.Ordinal = 2

	return &pb.RebaseChangeProposalResponse{Proposal: p}, nil
}

func (materialRPCFixture) MergeChangeProposal(ctx context.Context, req *pb.MergeChangeProposalRequest) (*pb.MergeChangeProposalResponse, error) {
	if materialRPCActor(ctx) != materialOwner {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	if req.GetExpectedHeadId() != proposalHeadID {
		return nil, status.Error(codes.Aborted, proposalDetail)
	}
	switch req.GetProposalId() {
	case waitingMergeID:
		return nil, status.Error(codes.FailedPrecondition, "ATTACHMENTS_PENDING")
	case blockedMergeID:
		return nil, status.Error(codes.FailedPrecondition, "UNRESOLVED_REVIEW_THREADS")
	case conflictedMerge:
		return nil, status.Error(codes.Aborted, "MERGE_CONFLICT")
	}

	return &pb.MergeChangeProposalResponse{ContentItemId: materialFixtureID, PublishedRevisionId: materialPublishedID}, nil
}

func checkReviewGraphQL(run func(string, string, ...string)) {
	get := `{ materialReviewStatus(proposalId:%q, revisionId:%q) { current outcome assignments { slot } decisions { legacy } } }`
	run(materialOwner, fmt.Sprintf(get, proposalFixtureID, proposalHeadID), `"current":true`, `"slot":1`, `"legacy":true`)
	run(materialOwner, fmt.Sprintf(get, proposalFixtureID, materialPublishedID), `"current":false`)
	run(proposalStranger, fmt.Sprintf(get, proposalFixtureID, proposalHeadID), `"code":"NOT_FOUND"`)
	run("", fmt.Sprintf(get, proposalFixtureID, proposalHeadID), `"code":"UNAUTHENTICATED"`)
	vote := `mutation { submitMaterialReview(input:{proposalId:%q,revisionId:%q,decision:approve}) { decision } }`
	run(materialReader, fmt.Sprintf(vote, proposalFixtureID, proposalHeadID), `"code":"PERMISSION_DENIED"`)
	run(materialOwner, fmt.Sprintf(vote, proposalFixtureID, materialPublishedID), `"code":"REVISION_CONFLICT"`)
	run(materialOwner, fmt.Sprintf(vote, proposalFixtureID, proposalHeadID), `"decision":"approve"`)
	run(materialOwner, fmt.Sprintf(`mutation { createMaterialReviewThread(
		input:{proposalId:%q,revisionId:%q,lineNumber:7,body:"Question"}) { id lineNumber }
	}`, proposalFixtureID, proposalHeadID), reviewThreadText, `"lineNumber":7`)
	threads := `{ materialReviewThreads(proposalId:%q,limit:2,afterId:%q) { items { id body } nextAfterId } }`
	run(materialOwner, fmt.Sprintf(threads, proposalFixtureID, "9007199254740992"), `"nextAfterId":"9007199254740993"`)
	run(materialOwner, fmt.Sprintf(threads, proposalFixtureID, "9223372036854775808"), `"code":"INVALID_ARGUMENT"`)
	run(materialOwner, fmt.Sprintf(`mutation { replyToMaterialReviewThread(proposalId:%q,threadId:%q,body:"Reply") { id } }`,
		proposalFixtureID, reviewThreadText), reviewReplyText)
	run(materialOwner, fmt.Sprintf(`{ materialReviewReplies(proposalId:%q,threadId:%q,limit:2,afterId:%q) { items { body } } }`,
		proposalFixtureID, reviewThreadText, reviewReplyText), `"body":"Reply"`)
	run(materialOwner, fmt.Sprintf(`mutation { setMaterialReviewThreadResolved(proposalId:%q,threadId:%q,resolved:true) { resolved } }`,
		proposalFixtureID, reviewThreadText), `"resolved":true`)
	run(materialReader, fmt.Sprintf(`mutation { rebaseMaterialChange(
		input:{proposalId:%q,expectedHeadId:%q,baseRevisionId:%q,source:"# Rebase",attachments:[]}) { head { ordinal } }
	}`, proposalFixtureID, proposalHeadID, materialPublishedID), `"ordinal":2`)
	merge := `mutation { mergeMaterialChange(proposalId:%q,expectedHeadId:%q) { publishedRevisionId } }`
	run(materialReader, fmt.Sprintf(merge, proposalFixtureID, proposalHeadID), `"code":"NOT_FOUND"`)
	run(materialOwner, fmt.Sprintf(merge, waitingMergeID, proposalHeadID), `"code":"ATTACHMENTS_PENDING"`)
	run(materialOwner, fmt.Sprintf(merge, blockedMergeID, proposalHeadID), `"code":"UNRESOLVED_REVIEW_THREADS"`)
	run(materialOwner, fmt.Sprintf(merge, conflictedMerge, proposalHeadID), `"code":"MERGE_CONFLICT"`)
	run(materialOwner, fmt.Sprintf(merge, proposalFixtureID, proposalHeadID), `"publishedRevisionId":"`+materialPublishedID+`"`)

	q := make([]string, 0, 7)
	for i := range 7 {
		q = append(q, fmt.Sprintf("part%d: materialReviewStatus(proposalId:%q){ current }", i, proposalFixtureID))
	}
	run(materialOwner, "{ "+strings.Join(q, " ")+" }", `"code":"COMPLEXITY_LIMIT_EXCEEDED"`)
}

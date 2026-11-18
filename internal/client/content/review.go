package content

import (
	"context"

	pb "github.com/overmindv/content/pkg/api/content"
)

type ReviewService interface {
	OpenProposal(context.Context, Actor, *pb.OpenChangeProposalRequest) (*pb.OpenChangeProposalResponse, error)
	GetProposal(context.Context, Actor, *pb.GetChangeProposalRequest) (*pb.GetChangeProposalResponse, error)
	AppendProposal(context.Context, Actor, *pb.AppendChangeProposalRevisionRequest) (*pb.AppendChangeProposalRevisionResponse, error)
	RebaseProposal(context.Context, Actor, *pb.RebaseChangeProposalRequest) (*pb.RebaseChangeProposalResponse, error)
	CloseProposal(context.Context, Actor, *pb.CloseChangeProposalRequest) (*pb.CloseChangeProposalResponse, error)
	MergeProposal(context.Context, Actor, *pb.MergeChangeProposalRequest) (*pb.MergeChangeProposalResponse, error)
	ProposalHistory(context.Context, Actor, *pb.ListChangeProposalHistoryRequest) (*pb.ListChangeProposalHistoryResponse, error)
	ProposalDiff(context.Context, Actor, *pb.GetChangeProposalDiffRequest) (*pb.GetChangeProposalDiffResponse, error)
	ReviewStatus(context.Context, Actor, *pb.GetReviewRevisionStatusRequest) (*pb.GetReviewRevisionStatusResponse, error)
	SubmitReview(context.Context, Actor, *pb.SubmitReviewDecisionRequest) (*pb.SubmitReviewDecisionResponse, error)
	ReviewThreads(context.Context, Actor, *pb.ListReviewThreadsRequest) (*pb.ListReviewThreadsResponse, error)
	ReviewReplies(context.Context, Actor, *pb.ListReviewRepliesRequest) (*pb.ListReviewRepliesResponse, error)
	CreateThread(context.Context, Actor, *pb.CreateReviewThreadRequest) (*pb.CreateReviewThreadResponse, error)
	ReplyToThread(context.Context, Actor, *pb.ReplyToReviewThreadRequest) (*pb.ReplyToReviewThreadResponse, error)
	SetThreadResolved(context.Context, Actor, *pb.SetReviewThreadResolvedRequest) (*pb.SetReviewThreadResolvedResponse, error)
	RestoreRevision(context.Context, Actor, *pb.RestorePublishedRevisionRequest) (*pb.RestorePublishedRevisionResponse, error)
	GetModeration(context.Context, Actor, *pb.GetMaterialModerationRequest) (*pb.MaterialModerationState, error)
	SetModeration(context.Context, Actor, *pb.SetMaterialModerationRequest) (*pb.MaterialModerationState, error)
}

func (c *Client) OpenProposal(ctx context.Context, actor Actor, req *pb.OpenChangeProposalRequest) (*pb.OpenChangeProposalResponse, error) {
	return call(ctx, c, actor, req, c.rpc.OpenChangeProposal)
}

func (c *Client) GetProposal(ctx context.Context, actor Actor, req *pb.GetChangeProposalRequest) (*pb.GetChangeProposalResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetChangeProposal)
}

func (c *Client) AppendProposal(ctx context.Context, actor Actor, req *pb.AppendChangeProposalRevisionRequest) (*pb.AppendChangeProposalRevisionResponse, error) {
	return call(ctx, c, actor, req, c.rpc.AppendChangeProposalRevision)
}

func (c *Client) RebaseProposal(ctx context.Context, actor Actor, req *pb.RebaseChangeProposalRequest) (*pb.RebaseChangeProposalResponse, error) {
	return call(ctx, c, actor, req, c.rpc.RebaseChangeProposal)
}

func (c *Client) CloseProposal(ctx context.Context, actor Actor, req *pb.CloseChangeProposalRequest) (*pb.CloseChangeProposalResponse, error) {
	return call(ctx, c, actor, req, c.rpc.CloseChangeProposal)
}

func (c *Client) MergeProposal(ctx context.Context, actor Actor, req *pb.MergeChangeProposalRequest) (*pb.MergeChangeProposalResponse, error) {
	return call(ctx, c, actor, req, c.rpc.MergeChangeProposal)
}

func (c *Client) ProposalHistory(ctx context.Context, actor Actor, req *pb.ListChangeProposalHistoryRequest) (*pb.ListChangeProposalHistoryResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListChangeProposalHistory)
}

func (c *Client) ProposalDiff(ctx context.Context, actor Actor, req *pb.GetChangeProposalDiffRequest) (*pb.GetChangeProposalDiffResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetChangeProposalDiff)
}

func (c *Client) ReviewStatus(ctx context.Context, actor Actor, req *pb.GetReviewRevisionStatusRequest) (*pb.GetReviewRevisionStatusResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetReviewRevisionStatus)
}

func (c *Client) SubmitReview(ctx context.Context, actor Actor, req *pb.SubmitReviewDecisionRequest) (*pb.SubmitReviewDecisionResponse, error) {
	return call(ctx, c, actor, req, c.rpc.SubmitReviewDecision)
}

func (c *Client) ReviewThreads(ctx context.Context, actor Actor, req *pb.ListReviewThreadsRequest) (*pb.ListReviewThreadsResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListReviewThreads)
}

func (c *Client) ReviewReplies(ctx context.Context, actor Actor, req *pb.ListReviewRepliesRequest) (*pb.ListReviewRepliesResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListReviewReplies)
}

func (c *Client) CreateThread(ctx context.Context, actor Actor, req *pb.CreateReviewThreadRequest) (*pb.CreateReviewThreadResponse, error) {
	return call(ctx, c, actor, req, c.rpc.CreateReviewThread)
}

func (c *Client) ReplyToThread(ctx context.Context, actor Actor, req *pb.ReplyToReviewThreadRequest) (*pb.ReplyToReviewThreadResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ReplyToReviewThread)
}

func (c *Client) SetThreadResolved(ctx context.Context, actor Actor, req *pb.SetReviewThreadResolvedRequest) (*pb.SetReviewThreadResolvedResponse, error) {
	return call(ctx, c, actor, req, c.rpc.SetReviewThreadResolved)
}

func (c *Client) RestoreRevision(ctx context.Context, actor Actor, req *pb.RestorePublishedRevisionRequest) (*pb.RestorePublishedRevisionResponse, error) {
	return call(ctx, c, actor, req, c.rpc.RestorePublishedRevision)
}

func (c *Client) GetModeration(ctx context.Context, actor Actor, req *pb.GetMaterialModerationRequest) (*pb.MaterialModerationState, error) {
	return callWithToken(ctx, c.moderationToken, c.timeout, actor, req, c.rpc.GetMaterialModeration)
}

func (c *Client) SetModeration(ctx context.Context, actor Actor, req *pb.SetMaterialModerationRequest) (*pb.MaterialModerationState, error) {
	return callWithToken(ctx, c.moderationToken, c.timeout, actor, req, c.rpc.SetMaterialModeration)
}

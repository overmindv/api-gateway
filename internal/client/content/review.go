package content

import (
	"context"

	pb "github.com/overmindv/content/pkg/api/content"
)

type ReviewService interface {
	ReviewStatus(context.Context, Actor, *pb.GetReviewRevisionStatusRequest) (*pb.GetReviewRevisionStatusResponse, error)
	ReviewThreads(context.Context, Actor, *pb.ListReviewThreadsRequest) (*pb.ListReviewThreadsResponse, error)
	ReviewReplies(context.Context, Actor, *pb.ListReviewRepliesRequest) (*pb.ListReviewRepliesResponse, error)
	SubmitReview(context.Context, Actor, *pb.SubmitReviewDecisionRequest) (*pb.SubmitReviewDecisionResponse, error)
	CreateThread(context.Context, Actor, *pb.CreateReviewThreadRequest) (*pb.CreateReviewThreadResponse, error)
	ReplyToThread(context.Context, Actor, *pb.ReplyToReviewThreadRequest) (*pb.ReplyToReviewThreadResponse, error)
	SetThreadResolved(context.Context, Actor, *pb.SetReviewThreadResolvedRequest) (*pb.SetReviewThreadResolvedResponse, error)
}

func (c *Client) RebaseProposal(ctx context.Context, actor Actor, req *pb.RebaseChangeProposalRequest) (*pb.RebaseChangeProposalResponse, error) {
	return call(ctx, c, actor, req, c.rpc.RebaseChangeProposal)
}

func (c *Client) MergeProposal(ctx context.Context, actor Actor, req *pb.MergeChangeProposalRequest) (*pb.MergeChangeProposalResponse, error) {
	return call(ctx, c, actor, req, c.rpc.MergeChangeProposal)
}

func (c *Client) ReviewStatus(ctx context.Context, actor Actor, req *pb.GetReviewRevisionStatusRequest) (*pb.GetReviewRevisionStatusResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetReviewRevisionStatus)
}

func (c *Client) ReviewThreads(ctx context.Context, actor Actor, req *pb.ListReviewThreadsRequest) (*pb.ListReviewThreadsResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListReviewThreads)
}

func (c *Client) ReviewReplies(ctx context.Context, actor Actor, req *pb.ListReviewRepliesRequest) (*pb.ListReviewRepliesResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListReviewReplies)
}

func (c *Client) SubmitReview(ctx context.Context, actor Actor, req *pb.SubmitReviewDecisionRequest) (*pb.SubmitReviewDecisionResponse, error) {
	return call(ctx, c, actor, req, c.rpc.SubmitReviewDecision)
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

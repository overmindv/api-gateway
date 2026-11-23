package content

import (
	"context"

	pb "github.com/overmindv/content/pkg/api/content"
)

type ProposalService interface {
	ReviewService
	SubmitInitialPublication(context.Context, Actor, *pb.SubmitInitialPublicationRequest) (*pb.SubmitInitialPublicationResponse, error)
	RebaseProposal(context.Context, Actor, *pb.RebaseChangeProposalRequest) (*pb.RebaseChangeProposalResponse, error)
	MergeProposal(context.Context, Actor, *pb.MergeChangeProposalRequest) (*pb.MergeChangeProposalResponse, error)
	OpenProposal(context.Context, Actor, *pb.OpenChangeProposalRequest) (*pb.OpenChangeProposalResponse, error)
	GetProposal(context.Context, Actor, *pb.GetChangeProposalRequest) (*pb.GetChangeProposalResponse, error)
	AppendProposal(context.Context, Actor, *pb.AppendChangeProposalRevisionRequest) (*pb.AppendChangeProposalRevisionResponse, error)
	CloseProposal(context.Context, Actor, *pb.CloseChangeProposalRequest) (*pb.CloseChangeProposalResponse, error)
	ProposalHistory(context.Context, Actor, *pb.ListChangeProposalHistoryRequest) (*pb.ListChangeProposalHistoryResponse, error)
	ProposalDiff(context.Context, Actor, *pb.GetChangeProposalDiffRequest) (*pb.GetChangeProposalDiffResponse, error)
}

func (c *Client) SubmitInitialPublication(ctx context.Context, actor Actor, req *pb.SubmitInitialPublicationRequest) (*pb.SubmitInitialPublicationResponse, error) {
	return call(ctx, c, actor, req, c.rpc.SubmitInitialPublication)
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

func (c *Client) CloseProposal(ctx context.Context, actor Actor, req *pb.CloseChangeProposalRequest) (*pb.CloseChangeProposalResponse, error) {
	return call(ctx, c, actor, req, c.rpc.CloseChangeProposal)
}

func (c *Client) ProposalHistory(ctx context.Context, actor Actor, req *pb.ListChangeProposalHistoryRequest) (*pb.ListChangeProposalHistoryResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListChangeProposalHistory)
}

func (c *Client) ProposalDiff(ctx context.Context, actor Actor, req *pb.GetChangeProposalDiffRequest) (*pb.GetChangeProposalDiffResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetChangeProposalDiff)
}

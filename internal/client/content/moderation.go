package content

import (
	"context"

	pb "github.com/overmindv/content/pkg/api/content"
)

type ModerationService interface {
	RestoreRevision(context.Context, Actor, *pb.RestorePublishedRevisionRequest) (*pb.RestorePublishedRevisionResponse, error)
	GetModeration(context.Context, Actor, *pb.GetMaterialModerationRequest) (*pb.GetMaterialModerationResponse, error)
	SetModeration(context.Context, Actor, *pb.SetMaterialModerationRequest) (*pb.SetMaterialModerationResponse, error)
}

func (c *Client) RestoreRevision(ctx context.Context, actor Actor, req *pb.RestorePublishedRevisionRequest) (*pb.RestorePublishedRevisionResponse, error) {
	return call(ctx, c, actor, req, c.rpc.RestorePublishedRevision)
}

func (c *Client) GetModeration(ctx context.Context, actor Actor, req *pb.GetMaterialModerationRequest) (*pb.GetMaterialModerationResponse, error) {
	return callWithToken(ctx, c.moderationToken, c.timeout, actor, req, c.rpc.GetMaterialModeration)
}

func (c *Client) SetModeration(ctx context.Context, actor Actor, req *pb.SetMaterialModerationRequest) (*pb.SetMaterialModerationResponse, error) {
	return callWithToken(ctx, c.moderationToken, c.timeout, actor, req, c.rpc.SetMaterialModeration)
}

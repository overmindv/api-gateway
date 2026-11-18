package content

import (
	"context"
	"crypto/tls"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/overmindv/api-gateway/internal/config"
	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Actor struct {
	UserID    string
	RequestID string
}

type Service interface {
	List(context.Context, Actor, *pb.ListContentCatalogRequest) (*pb.ListContentCatalogResponse, error)
	Get(context.Context, Actor, *pb.GetContentItemRequest) (*pb.GetContentItemResponse, error)
	Revisions(context.Context, Actor, *pb.ListContentRevisionHistoryRequest) (*pb.ListContentRevisionHistoryResponse, error)
	CreateDraft(context.Context, Actor, *pb.CreateContentItemRequest) (*pb.CreateContentItemResponse, error)
	SaveDraft(context.Context, Actor, *pb.CreateContentRevisionRequest) (*pb.CreateContentRevisionResponse, error)
	Publish(context.Context, Actor, *pb.PublishContentItemRequest) (*pb.PublishContentItemResponse, error)
}

type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

const messageLimit = 512 << 10

type Client struct {
	conn    *grpc.ClientConn
	rpc     pb.ContentServiceClient
	token   string
	timeout time.Duration
}

func New(cfg config.Content) (*Client, error) {
	transport := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	if cfg.Insecure {
		transport = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(cfg.Address, grpc.WithTransportCredentials(transport), grpc.WithDisableRetry(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(messageLimit), grpc.MaxCallSendMsgSize(messageLimit)))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, rpc: pb.NewContentServiceClient(conn), token: cfg.ServiceToken, timeout: cfg.Timeout}, nil
}

func (c *Client) Close() error { return c.conn.Close() }

func call[Req, Resp any](ctx context.Context, c *Client, actor Actor, req *Req,
	rpc func(context.Context, *Req, ...grpc.CallOption) (*Resp, error)) (*Resp, error) {
	id, err := uuid.Parse(actor.UserID)
	if err != nil || id == uuid.Nil || len(actor.UserID) != 36 {
		return nil, &Error{Code: "UNAUTHENTICATED"}
	}
	md := metadata.Pairs("authorization", "Bearer "+c.token, "x-user-id", id.String())
	if len(actor.RequestID) <= 128 && actor.RequestID != "" &&
		strings.IndexFunc(actor.RequestID, func(r rune) bool { return r < 32 || r >= 127 }) < 0 {
		md.Set("x-request-id", actor.RequestID)
	}
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, md), c.timeout)
	defer cancel()
	result, err := rpc(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}
	return result, nil
}

func rpcError(err error) error {
	code := "SERVICE_UNAVAILABLE"
	switch status.Code(err) {
	case codes.NotFound:
		code = "NOT_FOUND"
	case codes.PermissionDenied:
		code = "PERMISSION_DENIED"
	case codes.InvalidArgument:
		code = "INVALID_ARGUMENT"
	case codes.Aborted:
		code = "REVISION_CONFLICT"
	case codes.AlreadyExists:
		code = "ALREADY_EXISTS"
	case codes.FailedPrecondition:
		code = "FAILED_PRECONDITION"
	case codes.ResourceExhausted:
		code = "RESOURCE_EXHAUSTED"
	case codes.DeadlineExceeded:
		code = "DEADLINE_EXCEEDED"
	case codes.Canceled:
		code = "CANCELLED"
	}
	return &Error{Code: code}
}

func (c *Client) List(ctx context.Context, actor Actor, req *pb.ListContentCatalogRequest) (*pb.ListContentCatalogResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListContentCatalog)
}

func (c *Client) Get(ctx context.Context, actor Actor, req *pb.GetContentItemRequest) (*pb.GetContentItemResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetContentItem)
}

func (c *Client) Revisions(ctx context.Context, actor Actor, req *pb.ListContentRevisionHistoryRequest) (*pb.ListContentRevisionHistoryResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListContentRevisionHistory)
}

func (c *Client) CreateDraft(ctx context.Context, actor Actor, req *pb.CreateContentItemRequest) (*pb.CreateContentItemResponse, error) {
	return call(ctx, c, actor, req, c.rpc.CreateContentItem)
}

func (c *Client) SaveDraft(ctx context.Context, actor Actor, req *pb.CreateContentRevisionRequest) (*pb.CreateContentRevisionResponse, error) {
	return call(ctx, c, actor, req, c.rpc.CreateContentRevision)
}

func (c *Client) Publish(ctx context.Context, actor Actor, req *pb.PublishContentItemRequest) (*pb.PublishContentItemResponse, error) {
	return call(ctx, c, actor, req, c.rpc.PublishContentItem)
}

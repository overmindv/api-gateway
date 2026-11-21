package content

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
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
	ProposalService
	ModerationService
	List(context.Context, Actor, *pb.ListContentCatalogRequest) (*pb.ListContentCatalogResponse, error)
	Get(context.Context, Actor, *pb.GetContentItemRequest) (*pb.GetContentItemResponse, error)
	GetPublished(context.Context, Actor, *pb.GetPublishedContentItemRequest) (*pb.GetPublishedContentItemResponse, error)
	DownloadPublishedAttachment(context.Context, Actor, *pb.GetPublishedAttachmentDownloadRequest) (*pb.GetPublishedAttachmentDownloadResponse, error)
	Revisions(context.Context, Actor, *pb.ListContentRevisionHistoryRequest) (*pb.ListContentRevisionHistoryResponse, error)
	CreateDraft(context.Context, Actor, *pb.CreateContentItemRequest) (*pb.CreateContentItemResponse, error)
	SaveDraft(context.Context, Actor, *pb.CreateContentRevisionRequest, *pb.DraftMetadata) (*pb.ContentRevision, error)
}

type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

const messageLimit = 512 << 10
const codeUnavailable = "SERVICE_UNAVAILABLE"
const codeMergeConflict = "MERGE_CONFLICT"
const codeModerationConflict = "MODERATION_CONFLICT"

type Client struct {
	conn            *grpc.ClientConn
	rpc             pb.ContentServiceClient
	token           string
	moderationToken string
	timeout         time.Duration
}

func New(cfg config.Content) (*Client, error) {
	transport, err := contentCredentials(cfg)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(cfg.Address, grpc.WithTransportCredentials(transport), grpc.WithDisableRetry(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(messageLimit), grpc.MaxCallSendMsgSize(messageLimit)))
	if err != nil {
		return nil, err
	}

	return &Client{conn: conn, rpc: pb.NewContentServiceClient(conn), token: cfg.ServiceToken,
		moderationToken: cfg.ModerationToken, timeout: cfg.Timeout}, nil
}

func contentCredentials(cfg config.Content) (credentials.TransportCredentials, error) {
	if cfg.Insecure {
		if cfg.CAFile != "" {
			return nil, errors.New("content CA file requires TLS")
		}

		return insecure.NewCredentials(), nil
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, errors.New("could not read the Content gRPC CA file")
		}
		tlsConfig.RootCAs = x509.NewCertPool()
		if !tlsConfig.RootCAs.AppendCertsFromPEM(pem) {
			return nil, errors.New("the Content gRPC CA file contains no certificates")
		}
	}

	return credentials.NewTLS(tlsConfig), nil
}

func (c *Client) Close() error { return c.conn.Close() }

func call[Req, Resp any](ctx context.Context, c *Client, actor Actor, req *Req,
	rpc func(context.Context, *Req, ...grpc.CallOption) (*Resp, error)) (*Resp, error) {
	return callWithToken(ctx, c.token, c.timeout, actor, req, rpc)
}

func callWithToken[Req, Resp any](ctx context.Context, token string, timeout time.Duration, actor Actor, req *Req,
	rpc func(context.Context, *Req, ...grpc.CallOption) (*Resp, error)) (*Resp, error) {
	if token == "" {
		return nil, &Error{Code: codeUnavailable}
	}
	id, err := uuid.Parse(actor.UserID)
	if err != nil || id == uuid.Nil || len(actor.UserID) != 36 {
		return nil, &Error{Code: "UNAUTHENTICATED"}
	}
	md := metadata.Pairs("authorization", "Bearer "+token, "x-user-id", id.String())
	if len(actor.RequestID) <= 128 && actor.RequestID != "" &&
		strings.IndexFunc(actor.RequestID, func(r rune) bool { return r < 32 || r >= 127 }) < 0 {
		md.Set("x-request-id", actor.RequestID)
	}
	ctx, cancel := context.WithTimeout(metadata.NewOutgoingContext(ctx, md), timeout)
	defer cancel()
	result, err := rpc(ctx, req)
	if err != nil {
		return nil, rpcError(err)
	}

	return result, nil
}

func rpcError(err error) error {
	code := codeUnavailable
	switch status.Code(err) {
	case codes.NotFound:
		code = "NOT_FOUND"
	case codes.PermissionDenied:
		code = "PERMISSION_DENIED"
	case codes.InvalidArgument:
		code = "INVALID_ARGUMENT"
	case codes.Aborted:
		code = "REVISION_CONFLICT"
		switch message := status.Convert(err).Message(); message {
		case codeMergeConflict, codeModerationConflict:
			code = message
		}
	case codes.AlreadyExists:
		code = "ALREADY_EXISTS"
	case codes.FailedPrecondition:
		code = "FAILED_PRECONDITION"
		switch message := status.Convert(err).Message(); message {
		case "ATTACHMENTS_PENDING", "REVIEW_REQUIRED", "UNRESOLVED_REVIEW_THREADS":
			code = message
		}
	case codes.ResourceExhausted:
		code = "RESOURCE_EXHAUSTED"
	case codes.DeadlineExceeded:
		code = "DEADLINE_EXCEEDED"
	case codes.Canceled:
		code = "CANCELED"
	}

	return &Error{Code: code}
}

func (c *Client) List(ctx context.Context, actor Actor, req *pb.ListContentCatalogRequest) (*pb.ListContentCatalogResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListContentCatalog)
}

func (c *Client) Get(ctx context.Context, actor Actor, req *pb.GetContentItemRequest) (*pb.GetContentItemResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetContentItem)
}

func (c *Client) GetPublished(ctx context.Context, actor Actor, req *pb.GetPublishedContentItemRequest) (*pb.GetPublishedContentItemResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetPublishedContentItem)
}

func (c *Client) DownloadPublishedAttachment(ctx context.Context, actor Actor, req *pb.GetPublishedAttachmentDownloadRequest) (*pb.GetPublishedAttachmentDownloadResponse, error) {
	return call(ctx, c, actor, req, c.rpc.GetPublishedAttachmentDownload)
}

func (c *Client) Revisions(ctx context.Context, actor Actor, req *pb.ListContentRevisionHistoryRequest) (*pb.ListContentRevisionHistoryResponse, error) {
	return call(ctx, c, actor, req, c.rpc.ListContentRevisionHistory)
}

func (c *Client) CreateDraft(ctx context.Context, actor Actor, req *pb.CreateContentItemRequest) (*pb.CreateContentItemResponse, error) {
	return call(ctx, c, actor, req, c.rpc.CreateContentItem)
}

func (c *Client) SaveDraft(ctx context.Context, actor Actor, req *pb.CreateContentRevisionRequest, draft *pb.DraftMetadata) (*pb.ContentRevision, error) {
	if draft == nil {
		result, err := call(ctx, c, actor, req, c.rpc.CreateContentRevision)

		return result.GetRevision(), err
	}
	result, err := call(ctx, c, actor, &pb.SaveContentDraftRequest{Revision: req, DraftMetadata: draft}, c.rpc.SaveContentDraft)

	return result.GetRevision(), err
}

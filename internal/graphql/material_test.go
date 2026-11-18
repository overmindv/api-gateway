package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/config"
	"github.com/overmindv/api-gateway/internal/middleware"
	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	materialOwner        = "00000000-0000-4000-8000-000000000101"
	materialReader       = "00000000-0000-4000-8000-000000000102"
	materialFixtureID    = "00000000-0000-4000-8000-000000000103"
	materialDraftID      = "00000000-0000-4000-8000-000000000104"
	materialPublishedID  = "00000000-0000-4000-8000-000000000105"
	materialMissingID    = "00000000-0000-4000-8000-000000000106"
	materialSlowID       = "00000000-0000-4000-8000-000000000107"
	materialFixtureToken = "fixture-material-service-credential"
)

type materialRPCFixture struct {
	pb.UnimplementedContentServiceServer
}

func (materialRPCFixture) ListContentCatalog(_ context.Context, req *pb.ListContentCatalogRequest) (*pb.ListContentCatalogResponse, error) {
	if req.PageSize > 50 {
		return nil, status.Error(codes.Internal, "unexpected list input")
	}
	return &pb.ListContentCatalogResponse{NextCursor: "next-page", Entries: []*pb.ContentCatalogEntry{
		{Id: materialFixtureID, Title: "Go", Slug: "go", Status: "published", RevisionId: materialPublishedID},
	}}, nil
}

func (materialRPCFixture) GetContentItem(ctx context.Context, req *pb.GetContentItemRequest) (*pb.GetContentItemResponse, error) {
	if req.Id == materialSlowID {
		<-ctx.Done()
		return nil, status.Error(codes.DeadlineExceeded, "private upstream details")
	}
	if req.Id == materialMissingID {
		return nil, status.Error(codes.NotFound, "private upstream details")
	}
	published := materialRevisionFixture(materialPublishedID, "Public text")
	if materialRPCActor(ctx) == materialOwner {
		published = materialRevisionFixture(materialDraftID, "Private draft")
	}
	return &pb.GetContentItemResponse{Item: materialFixture("published", published)}, nil
}

func (materialRPCFixture) GetPublishedContentItem(_ context.Context, req *pb.GetPublishedContentItemRequest) (*pb.GetPublishedContentItemResponse, error) {
	if req.Id == materialMissingID {
		return nil, status.Error(codes.NotFound, "private upstream details")
	}
	revision := materialRevisionFixture(materialPublishedID, "Public text")
	if req.Id == materialSlowID {
		revision.Id = materialDraftID
	}
	return &pb.GetPublishedContentItemResponse{Item: materialFixture("published", revision)}, nil
}

func (materialRPCFixture) ListContentRevisionHistory(ctx context.Context, req *pb.ListContentRevisionHistoryRequest) (*pb.ListContentRevisionHistoryResponse, error) {
	if materialRPCActor(ctx) != materialOwner {
		return nil, status.Error(codes.NotFound, "private upstream details")
	}
	if req.BeforeRevision != 3 || req.PageSize != 2 {
		return nil, status.Error(codes.InvalidArgument, "wrong history pagination")
	}
	return &pb.ListContentRevisionHistoryResponse{NextBeforeRevision: 1, Items: []*pb.ContentRevisionSummary{
		{Id: materialPublishedID, Revision: 2, Format: "markdown"},
	}}, nil
}

func (materialRPCFixture) CreateContentItem(ctx context.Context, req *pb.CreateContentItemRequest) (*pb.CreateContentItemResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.CreatedBy != "" || req.Status != "draft" ||
		req.Source != "# New material" || req.GetLearning().GetEstimatedMinutes() != 25 {
		return nil, status.Error(codes.InvalidArgument, "unexpected create input")
	}
	return &pb.CreateContentItemResponse{Item: materialFixture("draft", materialRevisionFixture(materialDraftID, req.Source))}, nil
}

func (materialRPCFixture) CreateContentRevision(ctx context.Context, req *pb.CreateContentRevisionRequest) (*pb.CreateContentRevisionResponse, error) {
	if materialRPCActor(ctx) != materialOwner {
		return nil, status.Error(codes.NotFound, "private upstream details")
	}
	if req.ExpectedRevisionId != materialDraftID {
		return nil, status.Error(codes.Aborted, "private upstream details")
	}
	empty, inherited := req.Attachments != nil && len(req.Attachments.Assets) == 0, req.Attachments == nil
	if req.CreatedBy != "" || (req.Source == "Clear" && !empty) || (req.Source == "Keep" && !inherited) {
		return nil, status.Error(codes.InvalidArgument, "incorrect attachment presence")
	}
	return &pb.CreateContentRevisionResponse{Revision: materialRevisionFixture(materialDraftID, req.Source)}, nil
}

func (materialRPCFixture) PublishContentItem(ctx context.Context, req *pb.PublishContentItemRequest) (*pb.PublishContentItemResponse, error) {
	if materialRPCActor(ctx) != materialOwner {
		return nil, status.Error(codes.NotFound, "private upstream details")
	}
	if req.ExpectedRevisionId != materialDraftID {
		return nil, status.Error(codes.Aborted, "private upstream details")
	}
	return &pb.PublishContentItemResponse{Item: materialFixture("published", materialRevisionFixture(materialDraftID, "# Published"))}, nil
}

func materialRPCActor(ctx context.Context) string {
	return strings.Join(metadata.ValueFromIncomingContext(ctx, "x-user-id"), "")
}

func materialRevisionFixture(id, source string) *pb.ContentRevision {
	return &pb.ContentRevision{Id: id, Revision: 2, Format: "markdown", Source: source, CreatedBy: materialOwner}
}

func materialFixture(state string, revision *pb.ContentRevision) *pb.ContentItem {
	item := &pb.ContentItem{
		Id: materialFixtureID, Title: "Go", Slug: "go", Status: state, Type: "note", CreatedBy: materialOwner,
		CurrentRevision: revision,
	}
	if state == "published" {
		item.PublishedRevisionId = materialPublishedID
	}
	return item
}

func startMaterialRPC(t testing.TB, interceptor grpc.UnaryServerInterceptor, timeout time.Duration) *contentclient.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	pb.RegisterContentServiceServer(server, materialRPCFixture{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	client, err := contentclient.New(config.Content{
		Address: listener.Addr().String(), ServiceToken: materialFixtureToken, Timeout: timeout, Insecure: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestMaterialGraphQLThroughGRPC(t *testing.T) {
	client := startMaterialRPC(t, func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		actor := materialRPCActor(ctx)
		if actor != materialOwner && actor != materialReader {
			t.Errorf("unexpected forwarded actor %q", actor)
		}
		token := metadata.ValueFromIncomingContext(ctx, "authorization")
		if len(token) != 1 || token[0] != "Bearer "+materialFixtureToken ||
			len(metadata.ValueFromIncomingContext(ctx, "x-user-roles")) != 0 {
			t.Error("unexpected credential or roles sent to Content")
		}
		if request := metadata.ValueFromIncomingContext(ctx, "x-request-id"); len(request) != 1 || request[0] != "material-fixture" {
			t.Error("missing request ID")
		}
		if deadline, present := ctx.Deadline(); !present || time.Until(deadline) > time.Second {
			t.Error("missing bounded upstream deadline")
		}
		return next(ctx, req)
	}, time.Second)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	auth := middleware.NewJWTAuthenticator("fixture-gateway-jwt", "users")
	gateway := HandlerWithRecommendationsAndMetrics(nil, nil, nil, nil, nil, nil, nil, log, nil, HandlerOptions{Content: client})
	handler := middleware.RequestIDMiddleware(middleware.JWT(auth, gateway))

	run := func(actor, query string, want ...string) {
		t.Helper()
		body, err := json.Marshal(map[string]string{"query": query})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-User-ID", materialOwner)
		request.Header.Set("X-User-Roles", "admin")
		request.Header.Set("X-Request-ID", "material-fixture")
		if actor != "" {
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
				Issuer: "users", Subject: actor, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
			}).SignedString([]byte("fixture-gateway-jwt"))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		payload := response.Body.String()
		for _, part := range want {
			if !strings.Contains(payload, part) {
				t.Errorf("response missing %q: %s", part, payload)
			}
		}
		if strings.Contains(payload, "private upstream details") || (actor != materialOwner && strings.Contains(payload, "Private draft")) {
			t.Error("response exposed upstream error or a private draft")
		}
	}

	get := fmt.Sprintf(`{ material(id:%q) { revision { source } } }`, materialFixtureID)
	run("", get, `"code":"UNAUTHENTICATED"`)
	run(materialReader, get, "Public text")
	run(materialOwner, get, "Private draft")
	run(materialOwner, fmt.Sprintf(`{ publishedMaterial(id:%q) { revision { source } } }`, materialFixtureID), "Public text")
	run(materialOwner, fmt.Sprintf(`{ publishedMaterial(id:%q) { revision { source } } }`, materialSlowID), `"code":"NOT_FOUND"`)
	run(materialReader, fmt.Sprintf(`{ material(id:%q) { id } __typename }`, materialMissingID),
		`"code":"NOT_FOUND"`, `"material":null`, `"__typename":"Query"`)
	run(materialReader, `{ materials(limit:2) { items { id } nextCursor } }`, `"nextCursor":"next-page"`)
	history := fmt.Sprintf(`{ materialRevisions(id:%q, limit:2, beforeRevision:3) { items { number } nextBeforeRevision } }`, materialFixtureID)
	run(materialReader, history, `"code":"NOT_FOUND"`)
	run(materialOwner, history, `"number":2`, `"nextBeforeRevision":1`)
	run(materialOwner, `{ materials(limit:51) { nextCursor } }`, `"code":"INVALID_ARGUMENT"`)
	run(materialOwner, `mutation { createMaterialDraft(input:{type:note,title:"Go",slug:"go",source:"# New material",learning:{estimatedMinutes:25}}) { status } }`,
		`"status":"draft"`)
	save := `mutation { saveMaterialDraft(input:{materialId:%q,expectedRevisionId:%q,source:%q%s}) { source } }`
	run(materialReader, fmt.Sprintf(save, materialFixtureID, materialDraftID, "Keep", ""), `"code":"NOT_FOUND"`)
	run(materialOwner, fmt.Sprintf(save, materialFixtureID, materialPublishedID, "Keep", ""), `"code":"REVISION_CONFLICT"`)
	run(materialOwner, fmt.Sprintf(save, materialFixtureID, materialDraftID, "Clear", ",attachments:[]"), `"source":"Clear"`)
	run(materialOwner, fmt.Sprintf(save, materialFixtureID, materialDraftID, "Keep", ""), `"source":"Keep"`)
	publish := fmt.Sprintf(`mutation { publishMaterial(id:%q, expectedRevisionId:%q) { status } }`, materialFixtureID, materialDraftID)
	run(materialReader, publish, `"code":"NOT_FOUND"`)
	run(materialOwner, publish, `"status":"published"`)
	run(materialReader, fmt.Sprintf(`{ material(id:%q) { id } __typename }`, materialSlowID),
		`"code":"DEADLINE_EXCEEDED"`, `"__typename":"Query"`)
	run(materialReader, fmt.Sprintf(`{ material(id:%q) { revision { a:source b:source c:source d:source e:source f:source g:source } } }`, materialFixtureID),
		`"code":"COMPLEXITY_LIMIT_EXCEEDED"`)
}

func BenchmarkMaterialGateway(b *testing.B) {
	client := startMaterialRPC(b, nil, time.Second)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	gateway := HandlerWithRecommendationsAndMetrics(nil, nil, nil, nil, nil, nil, nil, log, nil, HandlerOptions{Content: client})
	ctx := middleware.ContextWithAuth(b.Context(), middleware.AuthInfo{UserID: materialOwner, Token: "fixture-gateway-jwt"})
	req := &pb.ListContentCatalogRequest{PageSize: 20}
	b.Run("grpc", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := client.List(ctx, contentclient.Actor{UserID: materialOwner}, req); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("graphql", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/graphql",
				strings.NewReader(`{"query":"{ materials { items { id title } nextCursor } }"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			gateway.ServeHTTP(response, request)
			if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(materialFixtureID)) {
				b.Fatal("catalog request failed")
			}
		}
	})
}

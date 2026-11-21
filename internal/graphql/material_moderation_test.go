package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/overmindv/api-gateway/internal/apperror"
	"github.com/overmindv/api-gateway/internal/middleware"
	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	moderationFixtureToken = "fixture-moderation-service-credential"
	moderatorSession       = "moderator-session"
	olderPublication       = "00000000-0000-4000-8000-000000000108"
	restoredPublication    = "00000000-0000-4000-8000-000000000109"
	moderationConflict     = "MODERATION_CONFLICT"
)

func (materialRPCFixture) RestorePublishedRevision(ctx context.Context, req *pb.RestorePublishedRevisionRequest) (*pb.RestorePublishedRevisionResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetContentItemId() != materialFixtureID || req.GetSourceRevisionId() != olderPublication {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	if req.GetExpectedPublishedRevisionId() != materialPublishedID {
		return nil, status.Error(codes.Aborted, proposalDetail)
	}
	revision := materialRevisionFixture(restoredPublication, "Restored")
	revision.ContentItemId = materialFixtureID
	revision.RestoredFromRevisionId = olderPublication

	return &pb.RestorePublishedRevisionResponse{Revision: revision}, nil
}

func (materialRPCFixture) GetMaterialModeration(_ context.Context, req *pb.GetMaterialModerationRequest) (*pb.GetMaterialModerationResponse, error) {
	if req.GetMaterialId() != materialFixtureID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}

	return &pb.GetMaterialModerationResponse{State: moderationFixtureState(false)}, nil
}

func (materialRPCFixture) SetMaterialModeration(_ context.Context, req *pb.SetMaterialModerationRequest) (*pb.SetMaterialModerationResponse, error) {
	if req.ExpectedModerationVersion == nil || req.GetExpectedModerationVersion() != reviewThreadID ||
		req.GetExpectedPublishedRevisionId() != materialPublishedID || req.GetMaterialId() != materialFixtureID {
		return nil, status.Error(codes.Aborted, moderationConflict)
	}
	if req.GetReason() != "Duplicate" {
		return nil, status.Error(codes.InvalidArgument, proposalDetail)
	}
	state := moderationFixtureState(req.GetAction() == pb.MaterialModerationAction_MATERIAL_MODERATION_ACTION_HIDE)
	state.Version++

	return &pb.SetMaterialModerationResponse{State: state}, nil
}

func moderationFixtureState(hidden bool) *pb.MaterialModerationState {
	return &pb.MaterialModerationState{MaterialId: materialFixtureID, PublishedRevisionId: materialPublishedID, Version: reviewThreadID, Hidden: hidden}
}

type moderatorAuthority struct {
	admin   atomic.Bool
	revoked atomic.Bool
	checks  atomic.Int64
}

func (s *moderatorAuthority) CheckSession(context.Context, string) (middleware.SessionState, error) {
	s.checks.Add(1)
	if s.revoked.Load() {
		return middleware.SessionState{}, apperror.ErrUnauthenticated
	}
	state := middleware.SessionState{UserID: materialReader, SessionID: moderatorSession, ExpiresAt: time.Now().Add(time.Minute)}
	if s.admin.Load() {
		state.Roles = []string{"admin"}
	}

	return state, nil
}

func (s *moderatorAuthority) RevokeSession(context.Context, string) error {
	s.revoked.Store(true)

	return nil
}

func newModerationGateway(t *testing.T, authority *moderatorAuthority, moderationKey string, calls *atomic.Int64) http.Handler {
	t.Helper()
	client := startMaterialRPCWithModeration(t, func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		wantToken, actor := materialFixtureToken, materialOwner
		switch info.FullMethod {
		case pb.ContentService_GetMaterialModeration_FullMethodName, pb.ContentService_SetMaterialModeration_FullMethodName:
			wantToken, actor = moderationFixtureToken, materialReader
			calls.Add(1)
		}
		values := metadata.ValueFromIncomingContext(ctx, "authorization")
		if len(values) != 1 || values[0] != "Bearer "+wantToken || materialRPCActor(ctx) != actor ||
			len(metadata.ValueFromIncomingContext(ctx, "x-user-roles")) != 0 {
			t.Error("wrong actor or credential in Content request")
		}
		if request := metadata.ValueFromIncomingContext(ctx, "x-request-id"); len(request) != 1 || request[0] != "moderation-fixture" {
			t.Error("missing request ID")
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > time.Second {
			t.Error("missing bounded Content deadline")
		}

		return next(ctx, req)
	}, time.Second, moderationKey)
	auth := middleware.NewJWTAuthenticator("fixture-moderation-jwt", "users")
	if authority != nil {
		auth.SetSessionVerifier(authority)
	}
	log := slog.New(slog.DiscardHandler)
	api := HandlerWithRecommendationsAndMetrics(nil, nil, nil, nil, nil, nil, nil, log, nil, HandlerOptions{Content: client})

	return middleware.RequestIDMiddleware(middleware.JWT(auth, api))
}

func runModerationGraphQL(t *testing.T, handler http.Handler, actor, query, want string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", materialOwner)
	req.Header.Set("X-User-Roles", "admin")
	req.Header.Set("X-Request-ID", "moderation-fixture")
	if actor != "" {
		token, signErr := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": "users", "sub": actor, "jti": moderatorSession, "exp": time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), "role": "admin",
		}).SignedString([]byte("fixture-moderation-jwt"))
		if signErr != nil {
			t.Fatal(signErr)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	raw := w.Body.String()
	if !strings.Contains(raw, want) || strings.Contains(raw, proposalDetail) || strings.Contains(raw, moderationFixtureToken) {
		t.Error("unexpected GraphQL response", raw)
	}
}

func TestMaterialModerationRequiresVerifiedRole(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	query := fmt.Sprintf(`{ materialModeration(id:%q) { version } }`, materialFixtureID)
	mutation := `mutation { setMaterialModeration(input:{materialId:%q,expectedPublishedRevisionId:%q,expectedVersion:%q,action:%s,reason:%q}) { hidden version } }`
	operation := func(version, action, reason string) string {
		return fmt.Sprintf(mutation, materialFixtureID, materialPublishedID, version, action, reason)
	}
	noUsers := newModerationGateway(t, nil, moderationFixtureToken, &calls)
	runModerationGraphQL(t, noUsers, materialReader, query, `"code":"UNAUTHENTICATED"`)
	runModerationGraphQL(t, noUsers, "", operation(reviewThreadText, "hide", "Duplicate"), `"code":"UNAUTHENTICATED"`)

	authority := &moderatorAuthority{}
	handler := newModerationGateway(t, authority, moderationFixtureToken, &calls)
	runModerationGraphQL(t, handler, materialReader, query, `"code":"PERMISSION_DENIED"`)
	authority.admin.Store(true)
	runModerationGraphQL(t, handler, materialReader, query, `"version":"`+reviewThreadText+`"`)
	authority.admin.Store(false)
	runModerationGraphQL(t, handler, materialReader, query, `"code":"PERMISSION_DENIED"`)
	if calls.Load() != 1 {
		t.Error("a denied request reached Content", calls.Load())
	}
	authority.admin.Store(true)
	before := authority.checks.Load()
	runModerationGraphQL(t, handler, materialReader, operation(reviewThreadText, "hide", "Duplicate"), `"hidden":true`)
	if authority.checks.Load() != before+1 {
		t.Error("mutation did not reuse the role check from the same request")
	}
	runModerationGraphQL(t, handler, materialReader, operation(reviewThreadText, "restore", "Duplicate"), `"hidden":false`)
	runModerationGraphQL(t, handler, materialReader, operation("0", "hide", "Duplicate"), `"code":"MODERATION_CONFLICT"`)
	before = calls.Load()
	for _, version := range []string{"-1", "+1", "01", "9223372036854775808"} {
		runModerationGraphQL(t, handler, materialReader, operation(version, "hide", "Duplicate"), `"code":"INVALID_ARGUMENT"`)
	}
	runModerationGraphQL(t, handler, materialReader, operation("0", "hide", strings.Repeat("x", 1025)), `"code":"INVALID_ARGUMENT"`)
	disabled := newModerationGateway(t, authority, "", &calls)
	runModerationGraphQL(t, disabled, materialReader, query, `"code":"SERVICE_UNAVAILABLE"`)
	authority.revoked.Store(true)
	runModerationGraphQL(t, handler, materialReader, operation(reviewThreadText, "hide", "Duplicate"), `"code":"UNAUTHENTICATED"`)
	if calls.Load() != before {
		t.Error("invalid or disabled moderation reached Content", calls.Load())
	}
}

func TestMaterialRollbackKeepsOwnerAuthority(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	handler := newModerationGateway(t, nil, moderationFixtureToken, &calls)
	mutation := `mutation { restoreMaterialRevision(materialId:%q,expectedPublishedRevisionId:%q,sourceRevisionId:%q) { restoredFromRevisionId } }`
	operation := fmt.Sprintf(mutation, materialFixtureID, materialPublishedID, olderPublication)
	runModerationGraphQL(t, handler, materialOwner, operation, `"restoredFromRevisionId":"`+olderPublication+`"`)
	runModerationGraphQL(t, handler, materialOwner, fmt.Sprintf(mutation, materialFixtureID, materialDraftID, olderPublication), `"code":"REVISION_CONFLICT"`)
	runModerationGraphQL(t, handler, materialOwner, fmt.Sprintf(mutation, materialFixtureID, materialPublishedID, materialDraftID), `"code":"NOT_FOUND"`)
	if calls.Load() != 0 {
		t.Error("a rollback used the moderation RPC")
	}
}

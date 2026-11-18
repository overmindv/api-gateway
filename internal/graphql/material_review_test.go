package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/overmindv/api-gateway/internal/middleware"
	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	changeProposalID       = "00000000-0000-4000-8000-000000000201"
	otherProposalID        = "00000000-0000-4000-8000-000000000202"
	reviewThreadID         = 9007199254740993
	reviewThreadText       = "9007199254740993"
	moderationFixtureToken = "fixture-moderation-service-credential"
	reviewJWTFixture       = "fixture-review-jwt"
)

type materialReviewRPC struct {
	pb.UnimplementedContentServiceServer
	merged     atomic.Bool
	moderated  atomic.Int32
	unresolved *bool
}

func reviewProposalFixture() *pb.ChangeProposal {
	return &pb.ChangeProposal{
		Id: changeProposalID, ContentItemId: materialFixtureID, BaseRevisionId: materialPublishedID, AuthorId: materialReader,
		Status: "open", Head: &pb.ChangeProposalRevision{Id: materialDraftID, Ordinal: 2, Source: "# Proposed text"},
	}
}

func (s *materialReviewRPC) OpenChangeProposal(_ context.Context, req *pb.OpenChangeProposalRequest) (*pb.OpenChangeProposalResponse, error) {
	if req.ContentItemId != materialFixtureID || req.BaseRevisionId != materialPublishedID || req.Attachments != nil {
		return nil, status.Error(codes.InvalidArgument, "incorrect proposal fields")
	}
	return &pb.OpenChangeProposalResponse{Proposal: reviewProposalFixture()}, nil
}

func (s *materialReviewRPC) GetChangeProposal(_ context.Context, req *pb.GetChangeProposalRequest) (*pb.GetChangeProposalResponse, error) {
	if req.ProposalId != changeProposalID {
		return nil, status.Error(codes.NotFound, "internal proposal details")
	}
	return &pb.GetChangeProposalResponse{Proposal: reviewProposalFixture()}, nil
}

func (s *materialReviewRPC) AppendChangeProposalRevision(_ context.Context, req *pb.AppendChangeProposalRevisionRequest) (*pb.AppendChangeProposalRevisionResponse, error) {
	if req.ExpectedHeadId != materialDraftID || req.Attachments == nil || len(req.Attachments.Assets) > 0 {
		return nil, status.Error(codes.InvalidArgument, "expected an explicit empty attachment set")
	}
	return &pb.AppendChangeProposalRevisionResponse{Proposal: reviewProposalFixture()}, nil
}

func (s *materialReviewRPC) RebaseChangeProposal(_ context.Context, req *pb.RebaseChangeProposalRequest) (*pb.RebaseChangeProposalResponse, error) {
	if req.BaseRevisionId != materialPublishedID || req.ExpectedHeadId != materialDraftID {
		return nil, status.Error(codes.InvalidArgument, "expected both revisions")
	}
	return &pb.RebaseChangeProposalResponse{Proposal: reviewProposalFixture()}, nil
}

func (s *materialReviewRPC) CloseChangeProposal(context.Context, *pb.CloseChangeProposalRequest) (*pb.CloseChangeProposalResponse, error) {
	proposal := reviewProposalFixture()
	proposal.Status = "closed"
	return &pb.CloseChangeProposalResponse{Proposal: proposal}, nil
}

func (s *materialReviewRPC) ListChangeProposalHistory(_ context.Context, req *pb.ListChangeProposalHistoryRequest) (*pb.ListChangeProposalHistoryResponse, error) {
	if req.BeforeOrdinal != 3 || req.PageSize != 2 {
		return nil, status.Error(codes.InvalidArgument, "incorrect history cursor")
	}
	return &pb.ListChangeProposalHistoryResponse{
		NextBeforeOrdinal: 2, Entries: []*pb.ChangeProposalHistoryEntry{{Id: materialDraftID, Ordinal: 2}},
	}, nil
}

func (s *materialReviewRPC) GetChangeProposalDiff(_ context.Context, req *pb.GetChangeProposalDiffRequest) (*pb.GetChangeProposalDiffResponse, error) {
	if req.RevisionId != materialDraftID {
		return nil, status.Error(codes.InvalidArgument, "incorrect diff revision")
	}
	return &pb.GetChangeProposalDiffResponse{
		BaseRevisionId: materialPublishedID, RevisionId: materialDraftID, UnifiedDiff: "+\u041f\u0440\u0430\u0432\u043a\u0430\n", Truncated: true,
	}, nil
}

func (s *materialReviewRPC) GetReviewRevisionStatus(_ context.Context, req *pb.GetReviewRevisionStatusRequest) (*pb.GetReviewRevisionStatusResponse, error) {
	return &pb.GetReviewRevisionStatusResponse{
		RevisionId: req.RevisionId, Current: true, Outcome: "waiting_candidates",
		WaitReason: "tiebreak_reviewer", Assignments: []*pb.ReviewAssignment{{ReviewerId: materialOwner, Slot: 1}},
		HasUnresolvedDiscussions: s.unresolved,
	}, nil
}

func (s *materialReviewRPC) SubmitReviewDecision(ctx context.Context, req *pb.SubmitReviewDecisionRequest) (*pb.SubmitReviewDecisionResponse, error) {
	if materialRPCActor(ctx) == materialReader {
		return nil, status.Error(codes.PermissionDenied, "internal self-review details")
	}
	if req.RevisionId != materialDraftID {
		return nil, status.Error(codes.Aborted, "internal stale-review details")
	}
	if req.Decision == "request_changes" && req.Reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason required")
	}
	return &pb.SubmitReviewDecisionResponse{Decision: &pb.ReviewDecision{
		ReviewerId: materialOwner, Decision: req.Decision, Reason: req.Reason,
	}}, nil
}

func reviewThreadFixture() *pb.ReviewThread {
	return &pb.ReviewThread{Id: reviewThreadID, RevisionId: materialDraftID, AuthorId: materialReader, Body: "Question"}
}

func (s *materialReviewRPC) CreateReviewThread(_ context.Context, req *pb.CreateReviewThreadRequest) (*pb.CreateReviewThreadResponse, error) {
	if req.RevisionId != materialDraftID || req.LineNumber != 7 {
		return nil, status.Error(codes.InvalidArgument, "incorrect thread anchor")
	}
	return &pb.CreateReviewThreadResponse{Thread: reviewThreadFixture()}, nil
}

func (s *materialReviewRPC) ListReviewThreads(_ context.Context, req *pb.ListReviewThreadsRequest) (*pb.ListReviewThreadsResponse, error) {
	if req.AfterId != reviewThreadID-1 || req.PageSize != 2 {
		return nil, status.Error(codes.InvalidArgument, "incorrect thread cursor")
	}
	return &pb.ListReviewThreadsResponse{Threads: []*pb.ReviewThread{reviewThreadFixture()}, NextAfterId: reviewThreadID}, nil
}

func (s *materialReviewRPC) ReplyToReviewThread(_ context.Context, req *pb.ReplyToReviewThreadRequest) (*pb.ReplyToReviewThreadResponse, error) {
	if req.ThreadId != reviewThreadID {
		return nil, status.Error(codes.InvalidArgument, "incorrect thread ID")
	}
	return &pb.ReplyToReviewThreadResponse{Reply: &pb.ReviewReply{Id: reviewThreadID + 1, Body: req.Body}}, nil
}

func (s *materialReviewRPC) ListReviewReplies(_ context.Context, req *pb.ListReviewRepliesRequest) (*pb.ListReviewRepliesResponse, error) {
	if req.ThreadId != reviewThreadID || req.AfterId != reviewThreadID+1 || req.PageSize != 2 {
		return nil, status.Error(codes.InvalidArgument, "incorrect reply cursor")
	}
	return &pb.ListReviewRepliesResponse{Replies: []*pb.ReviewReply{{Id: reviewThreadID + 2, Body: "Answer"}}, NextAfterId: reviewThreadID + 2}, nil
}

func (s *materialReviewRPC) SetReviewThreadResolved(_ context.Context, req *pb.SetReviewThreadResolvedRequest) (*pb.SetReviewThreadResolvedResponse, error) {
	if req.ThreadId != reviewThreadID {
		return nil, status.Error(codes.InvalidArgument, "incorrect thread ID")
	}
	thread := reviewThreadFixture()
	thread.Resolved = req.Resolved
	return &pb.SetReviewThreadResolvedResponse{Thread: thread}, nil
}

func (s *materialReviewRPC) MergeChangeProposal(_ context.Context, req *pb.MergeChangeProposalRequest) (*pb.MergeChangeProposalResponse, error) {
	if req.ExpectedHeadId != materialDraftID {
		return nil, status.Error(codes.Aborted, "internal stale-head details")
	}
	if s.unresolved != nil && *s.unresolved {
		return nil, status.Error(codes.FailedPrecondition, "UNRESOLVED_DISCUSSIONS")
	}
	if !s.merged.CompareAndSwap(false, true) {
		return nil, status.Error(codes.Aborted, "MERGE_CONFLICT")
	}
	return &pb.MergeChangeProposalResponse{ContentItemId: materialFixtureID, PublishedRevisionId: materialPublishedID}, nil
}

func (s *materialReviewRPC) RestorePublishedRevision(_ context.Context, req *pb.RestorePublishedRevisionRequest) (*pb.RestorePublishedRevisionResponse, error) {
	if req.ExpectedPublishedRevisionId != materialPublishedID || req.SourceRevisionId != materialDraftID {
		return nil, status.Error(codes.InvalidArgument, "incorrect rollback revisions")
	}
	rev := materialRevisionFixture(materialPublishedID, "# Restored")
	rev.RestoredFromRevisionId = req.SourceRevisionId
	return &pb.RestorePublishedRevisionResponse{Revision: rev}, nil
}

func (s *materialReviewRPC) GetMaterialModeration(context.Context, *pb.GetMaterialModerationRequest) (*pb.MaterialModerationState, error) {
	s.moderated.Add(1)
	return &pb.MaterialModerationState{MaterialId: materialFixtureID, PublishedRevisionId: materialPublishedID, Version: reviewThreadID}, nil
}

func (s *materialReviewRPC) SetMaterialModeration(_ context.Context, req *pb.SetMaterialModerationRequest) (*pb.MaterialModerationState, error) {
	s.moderated.Add(1)
	if req.ExpectedModerationVersion == nil || *req.ExpectedModerationVersion != reviewThreadID ||
		req.ExpectedPublishedRevisionId != materialPublishedID ||
		req.Action != pb.MaterialModerationAction_MATERIAL_MODERATION_ACTION_HIDE || req.Reason != "Duplicate" {
		return nil, status.Error(codes.InvalidArgument, "incorrect moderation request")
	}
	return &pb.MaterialModerationState{
		MaterialId: materialFixtureID, PublishedRevisionId: materialPublishedID, Version: reviewThreadID + 1, Hidden: true,
	}, nil
}

type reviewSessions struct {
	admin atomic.Bool
}

func (s *reviewSessions) CheckSession(_ context.Context, token string) (middleware.SessionState, error) {
	claims := &jwt.RegisteredClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		return middleware.SessionState{}, err
	}
	roles := []string{"user"}
	if claims.Subject == materialOwner && s.admin.Load() {
		roles = []string{"admin"}
	}
	return middleware.SessionState{UserID: claims.Subject, SessionID: claims.ID, Roles: roles}, nil
}

func (s *reviewSessions) RevokeSession(context.Context, string) error { return nil }

func reviewGateway(t *testing.T, fixture *materialReviewRPC, sessions middleware.SessionVerifier, modToken string) http.Handler {
	t.Helper()
	client := startContentFixture(t, fixture, func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		actor := materialRPCActor(ctx)
		wantToken := materialFixtureToken
		switch info.FullMethod {
		case pb.ContentService_GetMaterialModeration_FullMethodName, pb.ContentService_SetMaterialModeration_FullMethodName:
			wantToken = moderationFixtureToken
			if actor != materialOwner {
				t.Error("moderation actor did not come from Users")
			}
		}
		values := metadata.ValueFromIncomingContext(ctx, "authorization")
		if len(values) != 1 || values[0] != "Bearer "+wantToken || len(metadata.ValueFromIncomingContext(ctx, "x-user-roles")) != 0 {
			t.Error("incorrect upstream credential or unexpected role headers")
		}
		return next(ctx, req)
	}, 2*time.Second, modToken)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	gateway := HandlerWithRecommendationsAndMetrics(nil, nil, nil, nil, nil, nil, nil, logger, nil, HandlerOptions{Content: client})
	auth := middleware.NewJWTAuthenticator(reviewJWTFixture, "users")
	if sessions != nil {
		auth.SetSessionVerifier(sessions)
	}
	return middleware.JWT(auth, gateway)
}

func reviewToken(t *testing.T, actor string) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": actor, "iss": "users", "jti": actor, "exp": time.Now().Add(time.Minute).Unix(), "roles": []string{"admin"}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(reviewJWTFixture))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func reviewQuery(ctx context.Context, handler http.Handler, token, query string) string {
	body, _ := json.Marshal(struct {
		Query string `json:"query"`
	}{Query: query})
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/graphql", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("X-User-ID", materialOwner)
	request.Header.Set("X-User-Roles", "admin")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, request)
	return resp.Body.String()
}

func assertReviewPayload(t *testing.T, payload string, want ...string) {
	t.Helper()
	for _, value := range want {
		if !strings.Contains(payload, value) {
			t.Errorf("response missing %q: %s", value, payload)
		}
	}
	if strings.Contains(payload, "internal ") {
		t.Error("upstream error leaked to GraphQL")
	}
}

func TestMaterialReviewThroughGRPC(t *testing.T) {
	fixture := &materialReviewRPC{}
	handler := reviewGateway(t, fixture, &reviewSessions{}, "")
	author, reviewer := reviewToken(t, materialReader), reviewToken(t, materialOwner)
	run := func(token, query string, want ...string) {
		t.Helper()
		assertReviewPayload(t, reviewQuery(t.Context(), handler, token, query), want...)
	}
	run(author, fmt.Sprintf(`mutation { openMaterialChange(input:{materialId:%q,baseRevisionId:%q,source:"# Edit"}) { status } }`, materialFixtureID, materialPublishedID), `"status":"open"`)
	run(reviewer, fmt.Sprintf(`{ materialChangeProposal(id:%q) { authorId head { source } } }`, changeProposalID), materialReader, "Proposed text")
	run(author, fmt.Sprintf(`mutation { appendMaterialChange(input:{proposalId:%q,expectedHeadId:%q,source:"# Edit",attachments:[]}) { id } }`, changeProposalID, materialDraftID), changeProposalID)
	run(author, fmt.Sprintf(`mutation { rebaseMaterialChange(input:{proposalId:%q,expectedHeadId:%q,baseRevisionId:%q,source:"# Rebase"}) { id } }`, changeProposalID, materialDraftID, materialPublishedID), changeProposalID)
	run(reviewer, fmt.Sprintf(`{ materialChangeHistory(proposalId:%q,limit:2,beforeOrdinal:3) { items { ordinal } nextBeforeOrdinal } }`, changeProposalID), `"nextBeforeOrdinal":2`)
	run(reviewer, fmt.Sprintf(`{ materialChangeDiff(proposalId:%q, revisionId:%q) { unifiedDiff truncated } }`, changeProposalID, materialDraftID), `"truncated":true`, "\u041f\u0440\u0430\u0432\u043a\u0430")
	run(reviewer, fmt.Sprintf(`{ materialReviewStatus(proposalId:%q,revisionId:%q) { outcome assignments { slot } } }`, changeProposalID, materialDraftID), `"outcome":"waiting_candidates"`, `"slot":1`)
	vote := `mutation { submitMaterialReview(input:{proposalId:%q,revisionId:%q,decision:%s,reason:"Needs an example"}) { decision } }`
	run(author, fmt.Sprintf(vote, changeProposalID, materialDraftID, "approve"), `"code":"PERMISSION_DENIED"`)
	run(reviewer, fmt.Sprintf(vote, changeProposalID, materialPublishedID, "approve"), `"code":"REVISION_CONFLICT"`)
	run(reviewer, fmt.Sprintf(vote, changeProposalID, materialDraftID, "request_changes"), `"decision":"request_changes"`)
	run(reviewer, fmt.Sprintf(vote, changeProposalID, materialDraftID, "approve"), `"decision":"approve"`)
	run(author, fmt.Sprintf(`mutation { createMaterialReviewThread(input:{proposalId:%q,revisionId:%q,lineNumber:7,body:"Question"}) { id } }`, changeProposalID, materialDraftID), reviewThreadText)
	run(reviewer, fmt.Sprintf(`{ materialReviewThreads(proposalId:%q,limit:2,afterId:"9007199254740992") { items { id } nextAfterId } }`, changeProposalID), `"nextAfterId":"9007199254740993"`)
	run(reviewer, fmt.Sprintf(`mutation { replyToMaterialReviewThread(proposalId:%q,threadId:%q,body:"Reply") { id } }`, changeProposalID, reviewThreadText), "9007199254740994")
	run(reviewer, fmt.Sprintf(`{ materialReviewReplies(proposalId:%q,threadId:%q,limit:2,afterId:"9007199254740994") { items { body } nextAfterId } }`, changeProposalID, reviewThreadText), "Answer", `"nextAfterId":"9007199254740995"`)
	run(reviewer, fmt.Sprintf(`mutation { setMaterialReviewThreadResolved(proposalId:%q,threadId:%q,resolved:true) { resolved } }`, changeProposalID, reviewThreadText), `"resolved":true`)
	run(author, fmt.Sprintf(`mutation { closeMaterialChange(proposalId:%q) { status } }`, changeProposalID), `"status":"closed"`)
	run(reviewer, fmt.Sprintf(`mutation { restoreMaterialRevision(materialId:%q,expectedPublishedRevisionId:%q,sourceRevisionId:%q) { restoredFromRevisionId } }`, materialFixtureID, materialPublishedID, materialDraftID), materialDraftID)
	run(reviewer, fmt.Sprintf(`{ materialReviewThreads(proposalId:%q,afterId:"9223372036854775808") { nextAfterId } }`, changeProposalID), `"code":"INVALID_ARGUMENT"`)

	merge := `mutation { mergeMaterialChange(proposalId:%q,expectedHeadId:%q) { publishedRevisionId } }`
	run(reviewer, fmt.Sprintf(merge, changeProposalID, materialPublishedID), `"code":"REVISION_CONFLICT"`)
	results := make(chan string, 2)
	ctx := t.Context()
	for _, id := range []string{changeProposalID, otherProposalID} {
		go func() { results <- reviewQuery(ctx, handler, reviewer, fmt.Sprintf(merge, id, materialDraftID)) }()
	}
	a, b := <-results, <-results
	if strings.Contains(a, "MERGE_CONFLICT") {
		a, b = b, a
	}
	assertReviewPayload(t, a, `"publishedRevisionId":"`+materialPublishedID+`"`)
	assertReviewPayload(t, b, `"code":"MERGE_CONFLICT"`)
}

func TestMaterialReviewDiscussionStatus(t *testing.T) {
	query := fmt.Sprintf(`{ materialReviewStatus(proposalId:%q,revisionId:%q) { hasUnresolvedDiscussions } }`, changeProposalID, materialDraftID)
	token := reviewToken(t, materialOwner)
	legacy := reviewGateway(t, &materialReviewRPC{}, &reviewSessions{}, "")
	assertReviewPayload(t, reviewQuery(t.Context(), legacy, token, query), `"hasUnresolvedDiscussions":null`)

	unresolved := true
	current := reviewGateway(t, &materialReviewRPC{unresolved: &unresolved}, &reviewSessions{}, "")
	assertReviewPayload(t, reviewQuery(t.Context(), current, token, query), `"hasUnresolvedDiscussions":true`)
	merge := fmt.Sprintf(`mutation { mergeMaterialChange(proposalId:%q,expectedHeadId:%q) { publishedRevisionId } }`, changeProposalID, materialDraftID)
	assertReviewPayload(t, reviewQuery(t.Context(), current, token, merge), `"code":"UNRESOLVED_DISCUSSIONS"`)
}

func TestMaterialModerationChecksRolesBeforeUsingSeparateToken(t *testing.T) {
	fixture, sessions := &materialReviewRPC{}, &reviewSessions{}
	handler := reviewGateway(t, fixture, sessions, moderationFixtureToken)
	owner, reader := reviewToken(t, materialOwner), reviewToken(t, materialReader)
	query := fmt.Sprintf(`{ materialModeration(id:%q) { version } __typename }`, materialFixtureID)
	change := fmt.Sprintf(`mutation { setMaterialModeration(input:{materialId:%q,expectedPublishedRevisionId:%q,expectedVersion:%q,action:hide,reason:"Duplicate"}) { hidden version } }`, materialFixtureID, materialPublishedID, reviewThreadText)

	assertReviewPayload(t, reviewQuery(t.Context(), handler, reader, query), `"code":"PERMISSION_DENIED"`, `"__typename":"Query"`)
	assertReviewPayload(t, reviewQuery(t.Context(), reviewGateway(t, fixture, nil, moderationFixtureToken), owner, query), `"code":"UNAUTHENTICATED"`)
	sessions.admin.Store(true)
	assertReviewPayload(t, reviewQuery(t.Context(), handler, owner, query), `"version":"9007199254740993"`)
	assertReviewPayload(t, reviewQuery(t.Context(), reviewGateway(t, fixture, sessions, ""), owner, query), `"code":"SERVICE_UNAVAILABLE"`)
	assertReviewPayload(t, reviewQuery(t.Context(), handler, owner, change), `"hidden":true`, `"version":"9007199254740994"`)
	sessions.admin.Store(false)
	assertReviewPayload(t, reviewQuery(t.Context(), handler, owner, change), `"code":"PERMISSION_DENIED"`)
	assertReviewPayload(t, reviewQuery(t.Context(), handler, owner, query), `"code":"PERMISSION_DENIED"`)
	if calls := fixture.moderated.Load(); calls != 2 {
		t.Errorf("expected two authorized Content moderation calls, got %d", calls)
	}
}

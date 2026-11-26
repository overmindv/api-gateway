package graphql

import (
	"context"
	"fmt"
	"strings"
	"testing"

	pb "github.com/overmindv/content/pkg/api/content"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	proposalFixtureID = "00000000-0000-4000-8000-000000000108"
	proposalMergedID  = "00000000-0000-4000-8000-000000000118"
	proposalHeadID    = "00000000-0000-4000-8000-000000000109"
	proposalStranger  = "00000000-0000-4000-8000-000000000110"
	proposalDetail    = "private proposal context"
	proposalText      = "# Поиск в ширину"
	proposalBaseText  = "Published proposal baseline"
)

func proposalFixture(source string) *pb.ChangeProposal {
	return &pb.ChangeProposal{
		Id: proposalFixtureID, ContentItemId: materialFixtureID, BaseRevisionId: materialPublishedID,
		AuthorId: materialReader, Status: "open",
		Head: &pb.ChangeProposalRevision{Id: proposalHeadID, Ordinal: 1, Source: source},
	}
}

func (materialRPCFixture) SubmitInitialPublication(ctx context.Context, req *pb.SubmitInitialPublicationRequest) (*pb.SubmitInitialPublicationResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetContentItemId() != materialFixtureID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	if req.GetExpectedRevisionId() != materialDraftID {
		return nil, status.Error(codes.Aborted, proposalDetail)
	}
	proposal := proposalFixture(proposalText)
	proposal.AuthorId = materialOwner
	proposal.BaseRevisionId = materialDraftID
	proposal.InitialPublication = true

	return &pb.SubmitInitialPublicationResponse{Proposal: proposal}, nil
}

func (materialRPCFixture) OpenChangeProposal(ctx context.Context, req *pb.OpenChangeProposalRequest) (*pb.OpenChangeProposalResponse, error) {
	if materialRPCActor(ctx) != materialReader || req.GetContentItemId() != materialFixtureID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	if req.GetBaseRevisionId() != materialPublishedID {
		return nil, status.Error(codes.Aborted, proposalDetail)
	}
	if req.GetSource() == proposalBaseText {
		return nil, status.Error(codes.InvalidArgument, proposalDetail)
	}

	return &pb.OpenChangeProposalResponse{Proposal: proposalFixture(req.GetSource())}, nil
}

func (materialRPCFixture) GetChangeProposal(ctx context.Context, req *pb.GetChangeProposalRequest) (*pb.GetChangeProposalResponse, error) {
	actor := materialRPCActor(ctx)
	if actor != materialReader && actor != materialOwner {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	proposal := proposalFixture(proposalText)
	switch req.GetProposalId() {
	case proposalFixtureID:
		proposal.BaseOutdated = true
	case proposalMergedID:
		proposal.Id, proposal.Status = proposalMergedID, "merged"
	default:
		return nil, status.Error(codes.NotFound, proposalDetail)
	}

	return &pb.GetChangeProposalResponse{Proposal: proposal}, nil
}

func (materialRPCFixture) AppendChangeProposalRevision(ctx context.Context, req *pb.AppendChangeProposalRevisionRequest) (*pb.AppendChangeProposalRevisionResponse, error) {
	if materialRPCActor(ctx) != materialReader || req.GetProposalId() != proposalFixtureID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	if req.GetExpectedHeadId() != proposalHeadID {
		return nil, status.Error(codes.Aborted, proposalDetail)
	}
	proposal := proposalFixture(req.GetSource())
	proposal.Head.Ordinal, proposal.Head.ParentRevisionId = 2, proposalHeadID
	proposal.Head.Id = materialDraftID

	return &pb.AppendChangeProposalRevisionResponse{Proposal: proposal}, nil
}

func (materialRPCFixture) CloseChangeProposal(ctx context.Context, req *pb.CloseChangeProposalRequest) (*pb.CloseChangeProposalResponse, error) {
	if materialRPCActor(ctx) != materialReader || req.GetProposalId() != proposalFixtureID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	proposal := proposalFixture(proposalText)
	proposal.Status = "closed"

	return &pb.CloseChangeProposalResponse{Proposal: proposal}, nil
}

func (materialRPCFixture) ListChangeProposalHistory(ctx context.Context, req *pb.ListChangeProposalHistoryRequest) (*pb.ListChangeProposalHistoryResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetProposalId() != proposalFixtureID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}
	if req.GetBeforeOrdinal() != 3 || req.GetPageSize() != 2 {
		return nil, status.Error(codes.Internal, proposalDetail)
	}

	return &pb.ListChangeProposalHistoryResponse{NextBeforeOrdinal: 1, Entries: []*pb.ChangeProposalHistoryEntry{
		{Id: proposalHeadID, BaseRevisionId: materialPublishedID, Ordinal: 2, AuthorId: materialReader},
	}}, nil
}

func (materialRPCFixture) GetChangeProposalDiff(ctx context.Context, req *pb.GetChangeProposalDiffRequest) (*pb.GetChangeProposalDiffResponse, error) {
	if materialRPCActor(ctx) != materialOwner || req.GetProposalId() != proposalFixtureID || req.GetRevisionId() != proposalHeadID {
		return nil, status.Error(codes.NotFound, proposalDetail)
	}

	return &pb.GetChangeProposalDiffResponse{BaseRevisionId: materialPublishedID, RevisionId: proposalHeadID, UnifiedDiff: "+" + proposalText}, nil
}

func checkProposalGraphQL(t *testing.T, run func(string, string, ...string)) {
	t.Helper()

	get := fmt.Sprintf(`{ materialChangeProposal(id:%q){ baseOutdated initialPublication head{ source } } }`, proposalFixtureID)
	run("", get, `"code":"UNAUTHENTICATED"`)
	run(proposalStranger, get, `"code":"NOT_FOUND"`)
	run(materialOwner, get, `"baseOutdated":true`, `"initialPublication":false`, proposalText)
	merged := fmt.Sprintf(`{ materialChangeProposal(id:%q){ status } }`, proposalMergedID)
	run(materialOwner, merged, `"status":"merged"`)
	run(proposalStranger, merged, `"code":"NOT_FOUND"`)
	open := `mutation { openMaterialChange(input:{materialId:%q,baseRevisionId:%q,source:%q}){ id } }`
	run(materialReader, fmt.Sprintf(open, materialFixtureID, materialDraftID, proposalText), `"code":"REVISION_CONFLICT"`)
	run(materialReader, fmt.Sprintf(open, materialFixtureID, materialPublishedID, proposalBaseText), `"code":"INVALID_ARGUMENT"`)
	run(materialReader, fmt.Sprintf(open, materialFixtureID, materialPublishedID, proposalText), proposalFixtureID)
	appendRevision := `mutation { appendMaterialChange(input:{proposalId:%q,expectedHeadId:%q,source:"# Another version"}){ head{ ordinal } } }`
	run(materialReader, fmt.Sprintf(appendRevision, proposalFixtureID, materialDraftID), `"code":"REVISION_CONFLICT"`)
	run(materialOwner, fmt.Sprintf(appendRevision, proposalFixtureID, proposalHeadID), `"code":"NOT_FOUND"`)
	run(materialReader, fmt.Sprintf(appendRevision, proposalFixtureID, proposalHeadID), `"ordinal":2`)
	closeProposal := fmt.Sprintf(`mutation { closeMaterialChange(proposalId:%q){ status } }`, proposalFixtureID)
	run(materialOwner, closeProposal, `"code":"NOT_FOUND"`)
	run(materialReader, closeProposal, `"status":"closed"`)
	history := `{ materialChangeHistory(proposalId:%q,limit:%d,beforeOrdinal:3){ items{ baseRevisionId } nextBeforeOrdinal } }`
	run(materialOwner, fmt.Sprintf(history, proposalFixtureID, 2), materialPublishedID, `"nextBeforeOrdinal":1`)
	run(materialOwner, fmt.Sprintf(history, proposalFixtureID, 51), `"code":"INVALID_ARGUMENT"`)
	diff := fmt.Sprintf(`materialChangeDiff(proposalId:%q,revisionId:%q)`, proposalFixtureID, proposalHeadID)
	run(materialOwner, `{ `+diff+`{ unifiedDiff } }`, "+"+proposalText)

	var aliases []string
	for i := range 4 {
		aliases = append(aliases, fmt.Sprintf("part%d: %s{ revisionId }", i, diff))
	}
	run(materialOwner, "{ "+strings.Join(aliases, " ")+" }", `"code":"COMPLEXITY_LIMIT_EXCEEDED"`)
	aliases = aliases[:0]
	for i := range 7 {
		aliases = append(aliases, fmt.Sprintf("part%d: materialChangeProposal(id:%q){ id }", i, proposalFixtureID))
	}
	run(materialOwner, "{ "+strings.Join(aliases, " ")+" }", `"code":"COMPLEXITY_LIMIT_EXCEEDED"`)
}

func checkInitialPublicationGraphQL(t *testing.T, run func(string, string, ...string)) {
	t.Helper()

	const field = `submitMaterialPublication(id:%q,expectedRevisionId:%q){ id authorId baseRevisionId initialPublication }`
	submit := func(id, revision string) string { return "mutation { " + fmt.Sprintf(field, id, revision) + " }" }
	valid := submit(materialFixtureID, materialDraftID)
	run("", valid, `"code":"UNAUTHENTICATED"`)
	run(materialReader, valid, `"code":"NOT_FOUND"`)
	run(materialOwner, submit("invalid", materialDraftID), `"code":"INVALID_ARGUMENT"`)
	run(materialOwner, submit(materialFixtureID, "invalid"), `"code":"INVALID_ARGUMENT"`)
	run(materialOwner, submit(materialFixtureID, materialPublishedID), `"code":"REVISION_CONFLICT"`)
	run(materialOwner, valid, proposalFixtureID, materialOwner, materialDraftID, `"initialPublication":true`)

	aliases := make([]string, 0, 5)
	for i := range 5 {
		aliases = append(aliases, fmt.Sprintf("part%d: ", i)+fmt.Sprintf(field, materialFixtureID, materialDraftID))
	}
	run(materialOwner, "mutation { "+strings.Join(aliases, " ")+" }", `"code":"COMPLEXITY_LIMIT_EXCEEDED"`)
}

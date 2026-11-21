package graphql

import (
	"context"
	"math"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	pb "github.com/overmindv/content/pkg/api/content"
)

func (r *queryResolver) MaterialChangeProposal(ctx context.Context, id string) (*model.MaterialChangeProposal, error) {
	id, err := materialID(id)

	return resolveProposal(ctx, r.Resolver, &pb.GetChangeProposalRequest{ProposalId: id}, err,
		contentclient.ProposalService.GetProposal, func(resp *pb.GetChangeProposalResponse) *model.MaterialChangeProposal {
			return proposalModel(resp.GetProposal())
		})
}

func (r *queryResolver) MaterialChangeHistory(ctx context.Context, proposalID string, limit int, beforeOrdinal *int) (*model.MaterialChangeHistoryPage, error) {
	id, err := materialID(proposalID)
	req := &pb.ListChangeProposalHistoryRequest{ProposalId: id}
	if limit < 1 || limit > 50 {
		err = invalidMaterialInput()
	} else {
		req.PageSize = int32(limit)
	}
	if beforeOrdinal != nil {
		if *beforeOrdinal < 1 || *beforeOrdinal > math.MaxInt32 {
			err = invalidMaterialInput()
		} else {
			req.BeforeOrdinal = int32(*beforeOrdinal)
		}
	}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.ProposalHistory, proposalHistoryModel)
}

func (r *queryResolver) MaterialChangeDiff(ctx context.Context, proposalID string, revisionID *string) (*model.MaterialChangeDiff, error) {
	id, err := materialID(proposalID)
	req := &pb.GetChangeProposalDiffRequest{ProposalId: id}
	if err == nil && revisionID != nil {
		req.RevisionId, err = materialID(*revisionID)
	}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.ProposalDiff, proposalDiffModel)
}

func (r *mutationResolver) OpenMaterialChange(ctx context.Context, input model.OpenMaterialChangeInput) (*model.MaterialChangeProposal, error) {
	id, revision, err := proposalWriteInput(input.MaterialID, input.BaseRevisionID, input.Source, input.Message)
	req := &pb.OpenChangeProposalRequest{
		ContentItemId: id, BaseRevisionId: revision, Source: input.Source, Message: input.Message,
	}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.OpenProposal,
		func(resp *pb.OpenChangeProposalResponse) *model.MaterialChangeProposal {
			return proposalModel(resp.GetProposal())
		})
}

func (r *mutationResolver) AppendMaterialChange(ctx context.Context, input model.AppendMaterialChangeInput) (*model.MaterialChangeProposal, error) {
	id, revision, err := proposalWriteInput(input.ProposalID, input.ExpectedHeadID, input.Source, input.Message)
	req := &pb.AppendChangeProposalRevisionRequest{
		ProposalId: id, ExpectedHeadId: revision, Source: input.Source, Message: input.Message,
	}

	return resolveProposal(ctx, r.Resolver, req, err, contentclient.ProposalService.AppendProposal,
		func(resp *pb.AppendChangeProposalRevisionResponse) *model.MaterialChangeProposal {
			return proposalModel(resp.GetProposal())
		})
}

func (r *mutationResolver) CloseMaterialChange(ctx context.Context, proposalID string) (*model.MaterialChangeProposal, error) {
	id, err := materialID(proposalID)

	return resolveProposal(ctx, r.Resolver, &pb.CloseChangeProposalRequest{ProposalId: id}, err,
		contentclient.ProposalService.CloseProposal, func(resp *pb.CloseChangeProposalResponse) *model.MaterialChangeProposal {
			return proposalModel(resp.GetProposal())
		})
}

func resolveProposal[Req, Resp, View any](ctx context.Context, r *Resolver, req *Req, invalid error,
	call func(contentclient.ProposalService, context.Context, contentclient.Actor, *Req) (*Resp, error),
	view func(*Resp) *View) (*View, error) {
	client, actor, err := r.materialService(ctx)
	if err != nil {
		return nil, err
	}
	if invalid != nil {
		return nil, invalid
	}
	result, err := call(client, ctx, actor, req)
	if err != nil {
		return nil, err
	}
	payload := view(result)
	if payload == nil {
		return nil, &contentclient.Error{Code: "SERVICE_UNAVAILABLE"}
	}

	return payload, nil
}

func proposalWriteInput(entityID, revisionID, source, message string) (string, string, error) {
	id, err := materialID(entityID)
	if err != nil {
		return "", "", err
	}
	revision, err := materialID(revisionID)
	if err != nil || !validDraftText(source, message) {
		return "", "", invalidMaterialInput()
	}

	return id, revision, nil
}

func proposalModel(item *pb.ChangeProposal) *model.MaterialChangeProposal {
	head := item.GetHead()
	if head == nil {
		return nil
	}

	return &model.MaterialChangeProposal{
		ID: item.GetId(), MaterialID: item.GetContentItemId(), BaseRevisionID: item.GetBaseRevisionId(), AuthorID: item.GetAuthorId(),
		Status: model.MaterialChangeStatus(item.GetStatus()), CreatedAt: materialTimestamp(item.GetCreatedAt()),
		ClosedAt: materialOptional(materialTimestamp(item.GetClosedAt())), BaseOutdated: item.GetBaseOutdated(),
		Head: &model.MaterialChangeRevision{
			ID: head.GetId(), ParentRevisionID: materialOptional(head.GetParentRevisionId()), Ordinal: int(head.GetOrdinal()),
			Source: head.GetSource(), Message: head.GetMessage(), CreatedAt: materialTimestamp(head.GetCreatedAt()),
		},
	}
}

func proposalHistoryModel(result *pb.ListChangeProposalHistoryResponse) *model.MaterialChangeHistoryPage {
	if result == nil {
		return nil
	}
	items := make([]*model.MaterialChangeHistoryEntry, 0, len(result.GetEntries()))
	for _, entry := range result.GetEntries() {
		items = append(items, &model.MaterialChangeHistoryEntry{
			ID: entry.GetId(), ParentRevisionID: materialOptional(entry.GetParentRevisionId()), BaseRevisionID: entry.GetBaseRevisionId(),
			Ordinal: int(entry.GetOrdinal()), AuthorID: entry.GetAuthorId(), Message: entry.GetMessage(), CreatedAt: materialTimestamp(entry.GetCreatedAt()),
		})
	}
	page := &model.MaterialChangeHistoryPage{Items: items}
	if cursor := result.GetNextBeforeOrdinal(); cursor > 0 {
		next := int(cursor)
		page.NextBeforeOrdinal = &next
	}

	return page
}

func proposalDiffModel(result *pb.GetChangeProposalDiffResponse) *model.MaterialChangeDiff {
	if result == nil {
		return nil
	}

	return &model.MaterialChangeDiff{
		BaseRevisionID: result.GetBaseRevisionId(), RevisionID: result.GetRevisionId(),
		UnifiedDiff: result.GetUnifiedDiff(), Truncated: result.GetTruncated(),
	}
}

package graphql

import (
	"context"

	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/graphql/model"
)

// ProblemLists возвращает журнал списков задач с числом опубликованных задач.
func (r *queryResolver) ProblemLists(ctx context.Context, pagination *model.PaginationInput) (*model.ProblemListPage, error) {
	actor, err := tasksActor(ctx, true)
	if err != nil {
		return nil, err
	}
	limit, offset := 20, 0
	if pagination != nil {
		limit = intValue(pagination.Limit, 20)
		offset = intValue(pagination.Offset, 0)
	}
	items, err := r.Candidates.ListProblemLists(ctx, limit, offset, actor)
	if err != nil {
		return nil, err
	}
	result := make([]*model.ProblemList, 0, len(items.Items))
	for _, item := range items.Items {
		result = append(result, problemListSummaryModel(item))
	}

	return &model.ProblemListPage{Items: result, Limit: items.Limit, Offset: items.Offset}, nil
}

// ProblemList возвращает детали списка вместе с опубликованными задачами.
func (r *queryResolver) ProblemList(ctx context.Context, id string) (*model.ProblemList, error) {
	actor, err := tasksActor(ctx, true)
	if err != nil {
		return nil, err
	}
	detail, err := r.Candidates.GetProblemList(ctx, id, actor)
	if err != nil {
		return nil, err
	}

	return problemListDetailModel(detail), nil
}

// problemListSummaryModel преобразует журнальный элемент tasks в GraphQL-модель.
func problemListSummaryModel(item tasks.ProblemListSummary) *model.ProblemList {
	return &model.ProblemList{
		ID: item.ID, Name: item.Name, SourceID: item.SourceID, SourceURL: item.SourceURL,
		MemberCount: item.MemberCount, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

// problemListDetailModel преобразует детали списка с членами в GraphQL-модель.
func problemListDetailModel(item tasks.ProblemListDetail) *model.ProblemList {
	members := make([]*model.ProblemListMember, 0, len(item.Members))
	for _, member := range item.Members {
		members = append(members, &model.ProblemListMember{
			TaskID:     member.TaskID,
			Title:      member.Title,
			Difficulty: model.ITTaskDifficulty(member.Difficulty),
			Status:     model.ITTaskStatus(member.Status),
		})
	}
	summary := problemListSummaryModel(item.ProblemListSummary)
	summary.PendingCandidates = item.PendingCandidates
	summary.Members = members

	return summary
}

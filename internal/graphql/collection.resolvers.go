package graphql

import (
	"context"

	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasksit"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// StartTaskCollection ставит ручной сбор в персистентную очередь task-hunter.
func (r *mutationResolver) StartTaskCollection(ctx context.Context, input model.StartTaskCollectionInput) (*model.TaskCollectionJob, error) {
	actor, err := collectionActor(ctx)
	if err != nil {
		return nil, err
	}
	job, err := r.TaskHunter.StartJob(ctx, taskhunter.CreateJobInput{
		IdempotencyKey:    input.IdempotencyKey,
		TelegramChannels:  input.TelegramChannels,
		PublishedFrom:     input.PublishedFrom,
		PublishedTo:       input.PublishedTo,
		WebsiteURLs:       input.WebsiteUrls,
		MaxItemsPerSource: intValue(input.MaxItemsPerSource, 100),
	}, actor)
	if err != nil {
		return nil, err
	}

	return collectionJobModel(job), nil
}

// AcknowledgeTaskCollectionJob закрывает уведомление инициатора manual job.
func (r *mutationResolver) AcknowledgeTaskCollectionJob(ctx context.Context, id string) (bool, error) {
	actor, err := collectionActor(ctx)
	if err != nil {
		return false, err
	}
	if err := r.TaskHunter.Acknowledge(ctx, id, actor); err != nil {
		return false, err
	}

	return true, nil
}

// UpdateTaskCandidate сохраняет правки pending-кандидата.
func (r *mutationResolver) UpdateTaskCandidate(ctx context.Context, id string, input model.TaskCandidateReviewInput) (*model.TaskCandidate, error) {
	actor, err := tasksActor(ctx, true)
	if err != nil {
		return nil, err
	}
	item, err := r.Candidates.UpdateCandidate(ctx, id, candidateReviewInput(input), actor)
	if err != nil {
		return nil, err
	}

	return candidateModel(item), nil
}

// ApproveTaskCandidate публикует programming-задачу с итоговым payload.
func (r *mutationResolver) ApproveTaskCandidate(ctx context.Context, id string, input model.TaskCandidateReviewInput) (*model.ITTask, error) {
	actor, err := tasksActor(ctx, true)
	if err != nil {
		return nil, err
	}
	item, err := r.Candidates.ApproveCandidate(ctx, id, candidateReviewInput(input), actor)
	if err != nil {
		return nil, err
	}

	return taskModel(item), nil
}

// RejectTaskCandidate завершает модерацию без создания задачи.
func (r *mutationResolver) RejectTaskCandidate(ctx context.Context, id string, expectedRevision int, reason *string) (*model.TaskCandidate, error) {
	actor, err := tasksActor(ctx, true)
	if err != nil {
		return nil, err
	}
	item, err := r.Candidates.RejectCandidate(ctx, id, expectedRevision, stringValue(reason), actor)
	if err != nil {
		return nil, err
	}

	return candidateModel(item), nil
}

// TaskCollectionSources возвращает серверный allowlist источников.
func (r *queryResolver) TaskCollectionSources(ctx context.Context) (*model.TaskCollectionSources, error) {
	actor, err := collectionActor(ctx)
	if err != nil {
		return nil, err
	}
	sources, err := r.TaskHunter.Sources(ctx, actor)
	if err != nil {
		return nil, err
	}

	return &model.TaskCollectionSources{TelegramChannels: sources.TelegramChannels, WebsiteSources: sources.WebsiteSources}, nil
}

// TaskCollectionJobs возвращает журнал или непрочитанные terminal jobs инициатора.
func (r *queryResolver) TaskCollectionJobs(ctx context.Context, unreadOnly *bool, pagination *model.PaginationInput) (*model.TaskCollectionJobList, error) {
	actor, err := collectionActor(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := 20, 0
	if pagination != nil {
		limit = intValue(pagination.Limit, 20)
		offset = intValue(pagination.Offset, 0)
	}
	items, err := r.TaskHunter.ListJobs(ctx, boolValue(unreadOnly), limit, offset, actor)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TaskCollectionJob, 0, len(items.Items))
	for _, item := range items.Items {
		result = append(result, collectionJobModel(item))
	}

	return &model.TaskCollectionJobList{Items: result, Limit: items.Limit, Offset: items.Offset}, nil
}

// TaskCollectionJob возвращает безопасные детали выполнения источников.
func (r *queryResolver) TaskCollectionJob(ctx context.Context, id string) (*model.TaskCollectionJob, error) {
	actor, err := collectionActor(ctx)
	if err != nil {
		return nil, err
	}
	job, err := r.TaskHunter.GetJob(ctx, id, actor)
	if err != nil {
		return nil, err
	}

	return collectionJobModel(job), nil
}

// TaskCandidates возвращает очередь модерации tasks-it.
func (r *queryResolver) TaskCandidates(ctx context.Context, filter *model.TaskCandidateFilter, pagination *model.PaginationInput) (*model.TaskCandidateList, error) {
	actor, err := tasksActor(ctx, true)
	if err != nil {
		return nil, err
	}
	input := tasksit.CandidateFilter{Limit: 20}
	if pagination != nil {
		input.Limit = intValue(pagination.Limit, 20)
		input.Offset = intValue(pagination.Offset, 0)
	}
	if filter != nil {
		if filter.Status != nil {
			input.Status = filter.Status.String()
		}
		input.SourceID = stringValue(filter.SourceID)
		if filter.Difficulty != nil {
			input.Difficulty = filter.Difficulty.String()
		}
	}
	items, err := r.Candidates.ListCandidates(ctx, input, actor)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TaskCandidate, 0, len(items.Items))
	for _, item := range items.Items {
		result = append(result, candidateModel(item))
	}

	return &model.TaskCandidateList{Items: result, Limit: items.Limit, Offset: items.Offset}, nil
}

// TaskCandidate возвращает кандидата вместе с immutable provenance.
func (r *queryResolver) TaskCandidate(ctx context.Context, id string) (*model.TaskCandidate, error) {
	actor, err := tasksActor(ctx, true)
	if err != nil {
		return nil, err
	}
	item, err := r.Candidates.GetCandidate(ctx, id, actor)
	if err != nil {
		return nil, err
	}

	return candidateModel(item), nil
}

func collectionActor(ctx context.Context) (taskhunter.Actor, error) {
	auth, err := middleware.RequireAdmin(ctx)
	if err != nil {
		return taskhunter.Actor{}, err
	}

	return taskhunter.Actor{UserID: auth.UserID, Roles: auth.Roles}, nil
}

func candidateReviewInput(input model.TaskCandidateReviewInput) tasksit.CandidateReview {
	examples := make([]tasksit.TaskExample, 0, len(input.Examples))
	for _, example := range input.Examples {
		if example == nil {
			continue
		}
		examples = append(examples, tasksit.TaskExample{Input: example.Input, Output: example.Output, Explanation: stringValue(example.Explanation)})
	}

	return tasksit.CandidateReview{
		ExpectedRevision: input.ExpectedRevision, TopicID: input.TopicID, Title: input.Title,
		Statement: input.Statement, Difficulty: input.Difficulty.String(), Tags: input.Tags,
		Examples: examples, Constraints: input.Constraints,
	}
}

func candidateModel(item tasksit.Candidate) *model.TaskCandidate {
	examples := make([]*model.ITTaskExample, 0, len(item.Examples))
	for _, example := range item.Examples {
		examples = append(examples, &model.ITTaskExample{Input: example.Input, Output: example.Output, Explanation: example.Explanation})
	}

	return &model.TaskCandidate{
		ID: item.ID, Status: model.TaskCandidateStatus(item.Status), Revision: item.Revision,
		ExternalID: item.ExternalID, SourceID: item.SourceID, SourceName: item.SourceName,
		SourceURL: item.SourceURL, SourcePublishedAt: item.SourcePublishedAt, RetrievedAt: item.RetrievedAt,
		CollectionJobID: item.CollectionJobID, TopicID: item.TopicID, Title: item.Title,
		Statement: item.Statement, Difficulty: model.ITTaskDifficulty(item.Difficulty), Tags: item.Tags,
		Examples: examples, Constraints: item.Constraints, ApprovedTaskID: item.ApprovedTaskID,
		RejectionReason: item.RejectionReason, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func collectionJobModel(item taskhunter.Job) *model.TaskCollectionJob {
	sources := make([]*model.TaskCollectionSource, 0, len(item.Sources))
	for _, source := range item.Sources {
		sources = append(sources, &model.TaskCollectionSource{
			ID: source.ID, Kind: source.Kind, SourceID: source.SourceID, URL: source.URL,
			Status: model.TaskCollectionSourceStatus(source.Status), CollectedTotal: source.CollectedTotal,
			ImportedTotal: source.ImportedTotal, DuplicatesTotal: source.DuplicatesTotal,
			InvalidTotal: source.InvalidTotal, ErrorMessage: source.ErrorMessage,
		})
	}

	return &model.TaskCollectionJob{
		ID: item.ID, Trigger: item.Trigger, RequestedBy: item.RequestedBy, IdempotencyKey: item.IdempotencyKey,
		PublishedFrom: item.PublishedFrom, PublishedTo: item.PublishedTo, MaxItemsPerSource: item.MaxItemsPerSource,
		Status: model.TaskCollectionJobStatus(item.Status), CollectedTotal: item.CollectedTotal,
		ImportedTotal: item.ImportedTotal, DuplicatesTotal: item.DuplicatesTotal, InvalidTotal: item.InvalidTotal,
		ErrorCount: item.ErrorCount, ErrorMessage: item.ErrorMessage,
		NotificationAcknowledged: item.NotificationAcknowledged, StartedAt: item.StartedAt,
		FinishedAt: item.FinishedAt, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Sources: sources,
	}
}

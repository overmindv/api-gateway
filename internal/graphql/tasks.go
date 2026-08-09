package graphql

import (
	"context"

	"github.com/overmindv/api-gateway/internal/client/tasksit"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// tasksActor возвращает проверенного пользователя для actor headers.
func tasksActor(ctx context.Context, admin bool) (tasksit.Actor, error) {
	var auth middleware.AuthInfo
	var err error
	if admin {
		auth, err = middleware.RequireAdmin(ctx)
	} else {
		auth, err = middleware.RequireAuth(ctx)
	}
	if err != nil {
		return tasksit.Actor{}, err
	}

	return tasksit.Actor{
		UserID: auth.UserID,
		Roles:  auth.Roles,
	}, nil
}

// publicTaskFilter преобразует публичные фильтры GraphQL в HTTP-параметры.
func publicTaskFilter(filter *model.ITTaskFilter, pagination *model.PaginationInput) tasksit.TaskFilter {
	result := tasksit.TaskFilter{
		Limit:  50,
		Offset: 0,
	}
	if filter != nil {
		result.TopicID = filter.TopicID
		if filter.TaskType != nil {
			result.TaskType = filter.TaskType.String()
		}
		if filter.Difficulty != nil {
			result.Difficulty = filter.Difficulty.String()
		}
	}
	applyTaskPagination(&result, pagination)

	return result
}

// adminTaskFilter преобразует административные фильтры GraphQL в HTTP-параметры.
func adminTaskFilter(filter *model.ITAdminTaskFilter, pagination *model.PaginationInput) tasksit.TaskFilter {
	result := tasksit.TaskFilter{
		Limit:  50,
		Offset: 0,
	}
	if filter != nil {
		result.TopicID = filter.TopicID
		if filter.Status != nil {
			result.Status = filter.Status.String()
		}
		if filter.TaskType != nil {
			result.TaskType = filter.TaskType.String()
		}
		if filter.Difficulty != nil {
			result.Difficulty = filter.Difficulty.String()
		}
	}
	applyTaskPagination(&result, pagination)

	return result
}

// applyTaskPagination применяет общие limit и offset к фильтру tasks-it.
func applyTaskPagination(filter *tasksit.TaskFilter, pagination *model.PaginationInput) {
	if pagination == nil {
		return
	}
	filter.Limit = intValue(pagination.Limit, 50)
	filter.Offset = intValue(pagination.Offset, 0)
}

// tasksInput преобразует GraphQL-ввод теста в DTO tasks-it.
func tasksInput(input model.ITTaskInput) tasksit.TaskInput {
	options := make([]tasksit.TaskOptionInput, 0, len(input.Options))
	for _, option := range input.Options {
		if option == nil {
			continue
		}
		options = append(options, tasksit.TaskOptionInput{
			Text:      option.Text,
			IsCorrect: option.IsCorrect,
		})
	}
	difficulty := model.ITTaskDifficultyEasy.String()
	if input.Difficulty != nil {
		difficulty = input.Difficulty.String()
	}

	return tasksit.TaskInput{
		TopicID:    input.TopicID,
		Title:      input.Title,
		Statement:  input.Statement,
		TaskType:   input.TaskType.String(),
		Difficulty: difficulty,
		Options:    options,
	}
}

// submissionInput преобразует GraphQL-ответ пользователя в DTO tasks-it.
func submissionInput(input model.ITSubmissionInput) tasksit.SubmissionInput {
	return tasksit.SubmissionInput{
		TaskVersionID:     input.TaskVersionID,
		IdempotencyKey:    input.IdempotencyKey,
		SelectedOptionIDs: input.SelectedOptionIds,
	}
}

// taskModel преобразует полную задачу tasks-it в GraphQL-модель.
func taskModel(item tasksit.Task) *model.ITTask {
	options := make([]*model.ITTaskOption, 0, len(item.Options))
	for _, option := range item.Options {
		options = append(options, &model.ITTaskOption{
			ID:        option.ID,
			Text:      option.Text,
			Position:  option.Position,
			IsCorrect: option.IsCorrect,
		})
	}

	return &model.ITTask{
		ID:            item.ID,
		Status:        model.ITTaskStatus(item.Status),
		TaskVersionID: item.TaskVersionID,
		VersionNumber: item.VersionNumber,
		TopicID:       item.TopicID,
		Title:         item.Title,
		Statement:     item.Statement,
		TaskType:      model.ITTaskType(item.TaskType),
		Difficulty:    model.ITTaskDifficulty(item.Difficulty),
		Options:       options,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

// publicTaskModel принудительно скрывает признаки правильных вариантов.
func publicTaskModel(item tasksit.Task) *model.ITTask {
	result := taskModel(item)
	for _, option := range result.Options {
		option.IsCorrect = nil
	}

	return result
}

// taskListModel преобразует список задач с pagination metadata.
func taskListModel(list tasksit.TaskList) *model.ITTaskList {
	items := make([]*model.ITTaskSummary, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, &model.ITTaskSummary{
			ID:            item.ID,
			Status:        model.ITTaskStatus(item.Status),
			TaskVersionID: item.TaskVersionID,
			VersionNumber: item.VersionNumber,
			TopicID:       item.TopicID,
			Title:         item.Title,
			TaskType:      model.ITTaskType(item.TaskType),
			Difficulty:    model.ITTaskDifficulty(item.Difficulty),
			CreatedAt:     item.CreatedAt,
			UpdatedAt:     item.UpdatedAt,
		})
	}

	return &model.ITTaskList{
		Items:  items,
		Limit:  list.Limit,
		Offset: list.Offset,
	}
}

// submissionModel преобразует сохранённый результат в GraphQL-модель.
func submissionModel(item tasksit.Submission) *model.ITSubmission {
	return &model.ITSubmission{
		ID:                  item.ID,
		UserID:              item.UserID,
		TaskID:              item.TaskID,
		TaskVersionID:       item.TaskVersionID,
		TaskVersionNumber:   item.TaskVersionNumber,
		SelectedOptionIds:   item.SelectedOptionIDs,
		CorrectOptionIds:    item.CorrectOptionIDs,
		Correct:             item.Correct,
		Verdict:             model.ITSubmissionVerdict(item.Verdict),
		TaskUpdated:         item.TaskUpdated,
		LatestTaskVersionID: item.LatestTaskVersionID,
		LatestVersionNumber: item.LatestVersionNumber,
		CreatedAt:           item.CreatedAt,
	}
}

// submissionListModel преобразует историю решений с pagination metadata.
func submissionListModel(list tasksit.SubmissionList) *model.ITSubmissionList {
	items := make([]*model.ITSubmission, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, submissionModel(item))
	}

	return &model.ITSubmissionList{
		Items:  items,
		Limit:  list.Limit,
		Offset: list.Offset,
	}
}

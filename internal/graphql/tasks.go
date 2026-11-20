package graphql

import (
	"context"
	"strconv"
	"strings"

	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// tasksActor возвращает проверенного пользователя для actor headers.
func tasksActor(ctx context.Context, admin bool) (tasks.Actor, error) {
	var auth middleware.AuthInfo
	var err error
	if admin {
		auth, err = middleware.RequireAdmin(ctx)
	} else {
		auth, err = middleware.RequireAuth(ctx)
	}
	if err != nil {
		return tasks.Actor{}, err
	}

	return tasks.Actor{
		UserID: auth.UserID,
		Roles:  auth.Roles,
	}, nil
}

// publicTaskFilter преобразует публичные фильтры GraphQL в HTTP-параметры.
func publicTaskFilter(filter *model.ITTaskFilter, pagination *model.PaginationInput) tasks.TaskFilter {
	result := tasks.TaskFilter{
		Limit:  50,
		Offset: 0,
	}
	if filter != nil {
		result.TopicID = filter.TopicID
		if filter.Search != nil {
			result.Search = strings.TrimSpace(*filter.Search)
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

// adminTaskFilter преобразует административные фильтры GraphQL в HTTP-параметры.
func adminTaskFilter(filter *model.ITAdminTaskFilter, pagination *model.PaginationInput) tasks.TaskFilter {
	result := tasks.TaskFilter{
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

// applyTaskPagination применяет общие limit и offset к фильтру tasks.
func applyTaskPagination(filter *tasks.TaskFilter, pagination *model.PaginationInput) {
	if pagination == nil {
		return
	}
	filter.Limit = intValue(pagination.Limit, 50)
	filter.Offset = intValue(pagination.Offset, 0)
}

// tasksInput преобразует GraphQL-ввод теста в DTO tasks.
func tasksInput(input model.ITTaskInput) tasks.TaskInput {
	options := make([]tasks.TaskOptionInput, 0, len(input.Options))
	for _, option := range input.Options {
		if option == nil {
			continue
		}
		options = append(options, tasks.TaskOptionInput{
			Text:      option.Text,
			IsCorrect: option.IsCorrect,
		})
	}
	examples := make([]tasks.TaskExample, 0, len(input.Examples))
	for _, example := range input.Examples {
		if example == nil {
			continue
		}
		examples = append(examples, tasks.TaskExample{
			Input:       example.Input,
			Output:      example.Output,
			Explanation: stringValue(example.Explanation),
		})
	}
	difficulty := model.ITTaskDifficultyEasy.String()
	if input.Difficulty != nil {
		difficulty = input.Difficulty.String()
	}

	return tasks.TaskInput{
		TopicID:     input.TopicID,
		Title:       input.Title,
		Statement:   input.Statement,
		TaskType:    input.TaskType.String(),
		Difficulty:  difficulty,
		Options:     options,
		Tags:        input.Tags,
		Examples:    examples,
		Constraints: input.Constraints,
	}
}

// submissionInput преобразует GraphQL-ответ пользователя в DTO tasks.
func submissionInput(input model.ITSubmissionInput) tasks.SubmissionInput {
	return tasks.SubmissionInput{
		TaskVersionID:     input.TaskVersionID,
		IdempotencyKey:    input.IdempotencyKey,
		SelectedOptionIDs: input.SelectedOptionIds,
	}
}

// codeSubmissionInput преобразует GraphQL upload либо код из консоли в multipart-клиент tasks.
func codeSubmissionInput(input model.ITCodeSubmissionInput) tasks.CodeSubmissionInput {
	mode := input.RunMode.String()
	if mode == "" {
		mode = runModeSamples
	}
	dto := tasks.CodeSubmissionInput{
		RunMode:        mode,
		TaskVersionID:  input.TaskVersionID,
		IdempotencyKey: input.IdempotencyKey,
		Language:       input.Language.String(),
		SourceCode:     input.SourceCode,
	}
	if input.File != nil {
		dto.FileName = input.File.Filename
		dto.File = input.File.File
	}

	return dto
}

// taskModel преобразует полную задачу tasks в GraphQL-модель.
func taskModel(item tasks.Task) *model.ITTask {
	options := make([]*model.ITTaskOption, 0, len(item.Options))
	for _, option := range item.Options {
		options = append(options, &model.ITTaskOption{
			ID:        option.ID,
			Text:      option.Text,
			Position:  option.Position,
			IsCorrect: option.IsCorrect,
		})
	}
	examples := make([]*model.ITTaskExample, 0, len(item.Examples))
	for _, example := range item.Examples {
		examples = append(examples, &model.ITTaskExample{
			Input:       example.Input,
			Output:      example.Output,
			Explanation: example.Explanation,
		})
	}
	var source *model.ITTaskSource
	if item.Source != nil {
		source = &model.ITTaskSource{
			SourceID:    item.Source.SourceID,
			SourceName:  item.Source.SourceName,
			SourceURL:   item.Source.SourceURL,
			PublishedAt: item.Source.PublishedAt,
		}
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
		Tags:          item.Tags,
		Examples:      examples,
		Constraints:   item.Constraints,
		Source:        source,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

// publicTaskModel принудительно скрывает признаки правильных вариантов.
func publicTaskModel(item tasks.Task) *model.ITTask {
	result := taskModel(item)
	for _, option := range result.Options {
		option.IsCorrect = nil
	}

	return result
}

// taskListModel преобразует список задач с pagination metadata.
func taskListModel(list tasks.TaskList) *model.ITTaskList {
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
func submissionModel(item tasks.Submission) *model.ITSubmission {
	return &model.ITSubmission{
		ID:                  item.ID,
		UserID:              item.UserID,
		TaskID:              item.TaskID,
		TaskVersionID:       item.TaskVersionID,
		TaskVersionNumber:   item.TaskVersionNumber,
		TaskTitle:           item.TaskTitle,
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
func submissionListModel(list tasks.SubmissionList) *model.ITSubmissionList {
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

// codeSubmissionModel преобразует асинхронный результат sandbox в GraphQL-модель.
func codeSubmissionModel(item tasks.CodeSubmission) *model.ITCodeSubmission {
	mode := model.ITCodeRunMode(item.RunMode)
	if mode == "" {
		mode = model.ITCodeRunModeSamples
	}
	// Неизвестный режим закрываем консервативно, без case payload.
	allowSamples := mode == model.ITCodeRunModeSamples
	if !mode.IsValid() {
		mode = model.ITCodeRunModeGraded
	}

	tests := make([]*model.ITExecutionTestResult, 0, len(item.Tests))
	for _, test := range item.Tests {
		if !allowSamples || !publicSampleID(test.TestID) {
			continue
		}
		tests = append(tests, &model.ITExecutionTestResult{
			TestID:      test.TestID,
			Verdict:     model.ITExecutionVerdict(test.Verdict),
			Stdout:      test.Stdout,
			Stderr:      test.Stderr,
			DurationMs:  int(test.DurationMS),
			MemoryBytes: int(test.MemoryBytes),
		})
	}

	var verdict *model.ITExecutionVerdict
	if item.Verdict != nil {
		value := model.ITExecutionVerdict(*item.Verdict)
		verdict = &value
	}

	return &model.ITCodeSubmission{
		RunMode:           mode,
		ID:                item.ID,
		UserID:            item.UserID,
		TaskID:            item.TaskID,
		TaskVersionID:     item.TaskVersionID,
		TaskVersionNumber: item.TaskVersionNumber,
		ExecutionID:       item.ExecutionID,
		CorrelationID:     item.CorrelationID,
		Language:          model.ITProgrammingLanguage(item.Language),
		SourceFileName:    item.SourceFileName,
		Status:            model.ITCodeSubmissionStatus(item.Status),
		Verdict:           verdict,
		Compilation:       executionPhaseModel(item.Compilation),
		Execution:         executionPhaseModel(item.Execution),
		Tests:             tests,
		Failure:           executionFailureModel(item.Failure),
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
		CompletedAt:       item.CompletedAt,
	}
}

// executionPhaseModel преобразует необязательный результат фазы выполнения.
func executionPhaseModel(item *tasks.ExecutionPhaseResult) *model.ITExecutionPhaseResult {
	if item == nil {
		return nil
	}

	return &model.ITExecutionPhaseResult{
		ExitCode:    item.ExitCode,
		Stdout:      "",
		Stderr:      "",
		DurationMs:  int(item.DurationMS),
		MemoryBytes: int(item.MemoryBytes),
	}
}

// executionFailureModel преобразует безопасную ошибку sandbox.
func executionFailureModel(item *tasks.ExecutionFailure) *model.ITExecutionFailure {
	if item == nil {
		return nil
	}

	code := item.Code
	switch code {
	case executionInfrastructureError, "CHECKER_ERROR", "COMPILATION_ERROR", "RUNTIME_ERROR", "TIME_LIMIT_EXCEEDED", "MEMORY_LIMIT_EXCEEDED", "OUTPUT_LIMIT_EXCEEDED", "WRONG_ANSWER", "CANCELED":
	default:
		code = executionInfrastructureError
	}

	return &model.ITExecutionFailure{Code: code, Message: "execution could not be completed successfully"}
}

// codeSubmissionListModel преобразует историю программных решений.
func codeSubmissionListModel(list tasks.CodeSubmissionList) *model.ITCodeSubmissionList {
	items := make([]*model.ITCodeSubmission, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, codeSubmissionModel(item))
	}

	return &model.ITCodeSubmissionList{
		Items:  items,
		Limit:  list.Limit,
		Offset: list.Offset,
	}
}

// Samples владеет только каноническими открытыми примерами tasks.
func publicSampleID(id string) bool {
	if !strings.HasPrefix(id, "open-") {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, "open-"))

	return err == nil && n >= 1 && n <= 20 && id == "open-"+strconv.Itoa(n)
}

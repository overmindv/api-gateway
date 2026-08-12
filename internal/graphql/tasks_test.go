package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/overmindv/api-gateway/internal/apperror"
	"github.com/overmindv/api-gateway/internal/client/tasksit"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type tasksServiceStub struct {
	filter          tasksit.TaskFilter
	actor           tasksit.Actor
	input           tasksit.TaskInput
	submissionInput tasksit.SubmissionInput
	codeInput       tasksit.CodeSubmissionInput
	taskID          string
	submissionID    string
	status          string
	deleted         bool
}

// ListPublished сохраняет публичный фильтр и возвращает список.
func (s *tasksServiceStub) ListPublished(_ context.Context, filter tasksit.TaskFilter) (tasksit.TaskList, error) {
	s.filter = filter

	return tasksit.TaskList{Items: []tasksit.TaskSummary{sampleTaskSummary()}, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// GetPublished возвращает публичную задачу без правильных ответов.
func (s *tasksServiceStub) GetPublished(_ context.Context, id string) (tasksit.Task, error) {
	s.taskID = id

	return sampleUpstreamTask(), nil
}

// ListAdmin сохраняет административный фильтр и actor.
func (s *tasksServiceStub) ListAdmin(_ context.Context, filter tasksit.TaskFilter, actor tasksit.Actor) (tasksit.TaskList, error) {
	s.filter = filter
	s.actor = actor

	return tasksit.TaskList{Items: []tasksit.TaskSummary{sampleTaskSummary()}, Limit: filter.Limit, Offset: filter.Offset}, nil
}

// GetAdmin возвращает полную административную задачу.
func (s *tasksServiceStub) GetAdmin(_ context.Context, id string, actor tasksit.Actor) (tasksit.Task, error) {
	s.taskID = id
	s.actor = actor

	return sampleUpstreamTask(), nil
}

// Create сохраняет вход создания теста.
func (s *tasksServiceStub) Create(_ context.Context, input tasksit.TaskInput, actor tasksit.Actor) (tasksit.Task, error) {
	s.input = input
	s.actor = actor

	return sampleUpstreamTask(), nil
}

// Update сохраняет вход новой версии теста.
func (s *tasksServiceStub) Update(_ context.Context, id string, input tasksit.TaskInput, actor tasksit.Actor) (tasksit.Task, error) {
	s.taskID = id
	s.input = input
	s.actor = actor
	result := sampleUpstreamTask()
	result.VersionNumber = 2

	return result, nil
}

// ChangeStatus сохраняет lifecycle-переход.
func (s *tasksServiceStub) ChangeStatus(_ context.Context, id, status string, actor tasksit.Actor) (tasksit.Task, error) {
	s.taskID = id
	s.status = status
	s.actor = actor
	result := sampleUpstreamTask()
	result.Status = status

	return result, nil
}

// Delete сохраняет факт удаления теста.
func (s *tasksServiceStub) Delete(_ context.Context, id string, actor tasksit.Actor) error {
	s.taskID = id
	s.actor = actor
	s.deleted = true

	return nil
}

// Submit сохраняет ответ пользователя.
func (s *tasksServiceStub) Submit(_ context.Context, taskID string, input tasksit.SubmissionInput, actor tasksit.Actor) (tasksit.Submission, error) {
	s.taskID = taskID
	s.submissionInput = input
	s.actor = actor

	return sampleUpstreamSubmission(), nil
}

// GetSubmission возвращает сохранённый результат.
func (s *tasksServiceStub) GetSubmission(_ context.Context, id string, actor tasksit.Actor) (tasksit.Submission, error) {
	s.submissionID = id
	s.actor = actor

	return sampleUpstreamSubmission(), nil
}

// ListMySubmissions возвращает историю текущего пользователя.
func (s *tasksServiceStub) ListMySubmissions(_ context.Context, taskID *string, limit, offset int, actor tasksit.Actor) (tasksit.SubmissionList, error) {
	if taskID != nil {
		s.taskID = *taskID
	}
	s.filter.Limit = limit
	s.filter.Offset = offset
	s.actor = actor

	return tasksit.SubmissionList{Items: []tasksit.Submission{sampleUpstreamSubmission()}, Limit: limit, Offset: offset}, nil
}

// SubmitCode сохраняет файл программного решения.
func (s *tasksServiceStub) SubmitCode(_ context.Context, taskID string, input tasksit.CodeSubmissionInput, actor tasksit.Actor) (tasksit.CodeSubmission, error) {
	s.taskID = taskID
	s.codeInput = input
	s.actor = actor

	return sampleUpstreamCodeSubmission(), nil
}

// GetCodeSubmission возвращает результат программного решения.
func (s *tasksServiceStub) GetCodeSubmission(_ context.Context, id string, actor tasksit.Actor) (tasksit.CodeSubmission, error) {
	s.submissionID = id
	s.actor = actor

	return sampleUpstreamCodeSubmission(), nil
}

// ListMyCodeSubmissions возвращает историю программных решений.
func (s *tasksServiceStub) ListMyCodeSubmissions(_ context.Context, taskID *string, limit, offset int, actor tasksit.Actor) (tasksit.CodeSubmissionList, error) {
	if taskID != nil {
		s.taskID = *taskID
	}

	s.filter.Limit = limit
	s.filter.Offset = offset
	s.actor = actor

	return tasksit.CodeSubmissionList{
		Items:  []tasksit.CodeSubmission{sampleUpstreamCodeSubmission()},
		Limit:  limit,
		Offset: offset,
	}, nil
}

// Health возвращает успешную готовность stub-сервиса.
func (s *tasksServiceStub) Health(context.Context) error {
	return nil
}

// TestITTaskPublicQueries проверяет публичные запросы и сокрытие ответа.
func TestITTaskPublicQueries(t *testing.T) {
	t.Parallel()

	stub := &tasksServiceStub{}
	resolver := &queryResolver{Resolver: &Resolver{Tasks: stub}}
	difficulty := model.ITTaskDifficultyMedium
	limit, offset := 12, 3
	list, err := resolver.ItTasks(context.Background(), &model.ITTaskFilter{
		Difficulty: &difficulty,
	}, &model.PaginationInput{
		Limit:  &limit,
		Offset: &offset,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || stub.filter.Difficulty != "medium" || stub.filter.Limit != 12 || stub.filter.Offset != 3 {
		t.Fatalf("unexpected list mapping: %+v %+v", list, stub.filter)
	}
	task, err := resolver.ItTask(context.Background(), "task-id")
	if err != nil {
		t.Fatal(err)
	}
	if len(task.Options) != 1 || task.Options[0].IsCorrect != nil {
		t.Fatalf("public answer leaked: %+v", task.Options)
	}
}

// TestITTaskAdminMutations проверяет роли и основные admin mutations.
func TestITTaskAdminMutations(t *testing.T) {
	t.Parallel()

	stub := &tasksServiceStub{}
	root := &Resolver{Tasks: stub}
	resolver := &mutationResolver{Resolver: root}
	query := &queryResolver{Resolver: root}
	if _, err := resolver.CreateITTask(context.Background(), sampleGraphQLTaskInput()); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated, got %v", err)
	}
	if _, err := query.AdminITTask(context.Background(), "task-id"); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated admin query, got %v", err)
	}
	ctx := tasksContext("admin-id", []string{"admin"})
	status := model.ITTaskStatusDraft
	adminList, err := query.AdminITTasks(ctx, &model.ITAdminTaskFilter{Status: &status}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(adminList.Items) != 1 || stub.filter.Status != "draft" {
		t.Fatalf("unexpected admin list: %+v %+v", adminList, stub.filter)
	}
	adminTask, err := query.AdminITTask(ctx, "task-id")
	if err != nil {
		t.Fatal(err)
	}
	if adminTask.Options[0].IsCorrect == nil || !*adminTask.Options[0].IsCorrect {
		t.Fatalf("admin correct marker is missing: %+v", adminTask.Options)
	}
	created, err := resolver.CreateITTask(ctx, sampleGraphQLTaskInput())
	if err != nil {
		t.Fatal(err)
	}
	if created.Options[0].IsCorrect == nil || !*created.Options[0].IsCorrect || stub.actor.UserID != "admin-id" {
		t.Fatalf("unexpected create mapping: %+v %+v", created, stub.actor)
	}
	if stub.input.TaskType != "single_choice" || stub.input.Difficulty != "easy" || len(stub.input.Options) != 2 {
		t.Fatalf("unexpected upstream input: %+v", stub.input)
	}
	if _, err := resolver.UpdateITTask(ctx, "task-id", sampleGraphQLTaskInput()); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ChangeITTaskStatus(ctx, "task-id", model.ITTaskStatusPublished); err != nil {
		t.Fatal(err)
	}
	deleted, err := resolver.DeleteITTask(ctx, "task-id")
	if err != nil || !deleted || !stub.deleted || stub.status != "published" {
		t.Fatalf("unexpected mutation state: deleted=%v status=%s err=%v", deleted, stub.status, err)
	}
}

// TestITTaskSubmissionQueries проверяет отправку, результат и историю.
func TestITTaskSubmissionQueries(t *testing.T) {
	t.Parallel()

	stub := &tasksServiceStub{}
	root := &Resolver{Tasks: stub}
	query := &queryResolver{Resolver: root}
	mutation := &mutationResolver{Resolver: root}
	ctx := tasksContext("user-id", []string{"student"})
	result, err := mutation.SubmitITTaskAnswer(ctx, "task-id", model.ITSubmissionInput{
		TaskVersionID:     "version-id",
		IdempotencyKey:    "key-id",
		SelectedOptionIds: []string{"option-id"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Correct || !result.TaskUpdated || stub.submissionInput.IdempotencyKey != "key-id" {
		t.Fatalf("unexpected submission mapping: %+v %+v", result, stub.submissionInput)
	}
	if _, err := query.ItSubmission(ctx, "submission-id"); err != nil {
		t.Fatal(err)
	}
	limit := 7
	history, err := query.MyITSubmissions(ctx, &stub.taskID, &model.PaginationInput{Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Items) != 1 || history.Limit != 7 || stub.actor.UserID != "user-id" {
		t.Fatalf("unexpected history mapping: %+v", history)
	}
}

// TestITTaskCodeSubmissionQueries проверяет upload, polling-результат и историю кода.
func TestITTaskCodeSubmissionQueries(t *testing.T) {
	t.Parallel()

	stub := &tasksServiceStub{}
	root := &Resolver{Tasks: stub}
	query := &queryResolver{Resolver: root}
	mutation := &mutationResolver{Resolver: root}
	ctx := tasksContext("user-id", []string{"student"})
	result, err := mutation.SubmitITTaskCode(ctx, "task-id", model.ITCodeSubmissionInput{
		TaskVersionID:  "version-id",
		IdempotencyKey: "key-id",
		Language:       model.ITProgrammingLanguagePython,
		File: gqlgen.Upload{
			File:        bytes.NewReader([]byte("print(42)")),
			Filename:    "solution.py",
			Size:        9,
			ContentType: "text/x-python",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	source, err := io.ReadAll(stub.codeInput.File)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != model.ITCodeSubmissionStatusCompleted || result.Verdict == nil || string(source) != "print(42)" {
		t.Fatalf("unexpected code submission mapping: result=%+v source=%q", result, source)
	}
	if stub.codeInput.Language != "python" || stub.codeInput.FileName != "solution.py" || stub.codeInput.IdempotencyKey != "key-id" {
		t.Fatalf("unexpected code input: %+v", stub.codeInput)
	}

	loaded, err := query.ItCodeSubmission(ctx, "code-submission-id")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Execution == nil || len(loaded.Tests) != 1 || loaded.Tests[0].Verdict != model.ITExecutionVerdictAccepted {
		t.Fatalf("unexpected execution result: %+v", loaded)
	}

	limit := 8
	history, err := query.MyITCodeSubmissions(ctx, &stub.taskID, &model.PaginationInput{Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Items) != 1 || history.Limit != 8 || stub.actor.UserID != "user-id" {
		t.Fatalf("unexpected code history mapping: %+v", history)
	}
}

// TestITTaskCodeSubmissionMultipart проверяет GraphQL multipart transport целиком.
func TestITTaskCodeSubmissionMultipart(t *testing.T) {
	t.Parallel()

	stub := &tasksServiceStub{}
	handler := Handler(
		nil,
		nil,
		stub,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	operations := map[string]any{
		"query": `mutation Submit($taskId: ID!, $input: ITCodeSubmissionInput!) {
  submitITTaskCode(taskId: $taskId, input: $input) { id status sourceFileName }
}`,
		"variables": map[string]any{
			"taskId": "task-id",
			"input": map[string]any{
				"taskVersionId":  "version-id",
				"idempotencyKey": "key-id",
				"language":       "python",
				"file":           nil,
			},
		},
	}
	writeMultipartJSON(t, writer, "operations", operations)
	writeMultipartJSON(t, writer, "map", map[string][]string{
		"0": {"variables.input.file"},
	})

	file, err := writer.CreateFormFile("0", "solution.py")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("print(42)")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/graphql", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request = request.WithContext(tasksContext("user-id", []string{"student"}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected GraphQL status: %d body=%s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			SubmitITTaskCode struct {
				ID string `json:"id"`
			} `json:"submitITTaskCode"`
		} `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Errors) != 0 || response.Data.SubmitITTaskCode.ID != "code-submission-id" {
		t.Fatalf("unexpected GraphQL response: %s", recorder.Body.String())
	}
	if stub.codeInput.FileName != "solution.py" || stub.codeInput.Language != "python" {
		t.Fatalf("unexpected multipart input: %+v", stub.codeInput)
	}
}

// writeMultipartJSON записывает служебную JSON-часть GraphQL multipart-запроса.
func writeMultipartJSON(t *testing.T, writer *multipart.Writer, name string, value any) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField(name, string(data)); err != nil {
		t.Fatal(err)
	}
}

// tasksContext создаёт контекст с проверенным JWT actor.
func tasksContext(userID string, roles []string) context.Context {
	return middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{
		UserID: userID,
		Token:  "jwt",
		Roles:  roles,
	})
}

// sampleGraphQLTaskInput возвращает валидный GraphQL-ввод теста.
func sampleGraphQLTaskInput() model.ITTaskInput {
	return model.ITTaskInput{
		Title:      "Go interfaces",
		Statement:  "Choose one",
		TaskType:   model.ITTaskTypeSingleChoice,
		Difficulty: nil,
		Options: []*model.ITTaskOptionInput{
			{Text: "A", IsCorrect: true},
			{Text: "B", IsCorrect: false},
		},
	}
}

// sampleUpstreamTask возвращает задачу tasks-it для mapper-тестов.
func sampleUpstreamTask() tasksit.Task {
	correct := true

	return tasksit.Task{
		ID:            "task-id",
		Status:        "draft",
		TaskVersionID: "version-id",
		VersionNumber: 1,
		Title:         "Go interfaces",
		Statement:     "Choose one",
		TaskType:      "single_choice",
		Difficulty:    "easy",
		Options: []tasksit.TaskOption{{
			ID:        "option-id",
			Text:      "A",
			Position:  0,
			IsCorrect: &correct,
		}},
	}
}

// sampleTaskSummary возвращает краткую задачу tasks-it.
func sampleTaskSummary() tasksit.TaskSummary {
	return tasksit.TaskSummary{
		ID:            "task-id",
		Status:        "published",
		TaskVersionID: "version-id",
		VersionNumber: 1,
		Title:         "Go interfaces",
		TaskType:      "single_choice",
		Difficulty:    "easy",
	}
}

// sampleUpstreamSubmission возвращает исторический результат tasks-it.
func sampleUpstreamSubmission() tasksit.Submission {
	return tasksit.Submission{
		ID:                  "submission-id",
		UserID:              "user-id",
		TaskID:              "task-id",
		TaskVersionID:       "version-id",
		TaskVersionNumber:   1,
		SelectedOptionIDs:   []string{"option-id"},
		CorrectOptionIDs:    []string{"option-id"},
		Correct:             true,
		Verdict:             "accepted",
		TaskUpdated:         true,
		LatestTaskVersionID: "version-id-2",
		LatestVersionNumber: 2,
	}
}

// sampleUpstreamCodeSubmission возвращает завершённый результат sandbox.
func sampleUpstreamCodeSubmission() tasksit.CodeSubmission {
	verdict := "accepted"

	return tasksit.CodeSubmission{
		ID:                "code-submission-id",
		UserID:            "user-id",
		TaskID:            "task-id",
		TaskVersionID:     "version-id",
		TaskVersionNumber: 1,
		ExecutionID:       "execution-id",
		CorrelationID:     "correlation-id",
		Language:          "python",
		SourceFileName:    "solution.py",
		Status:            "completed",
		Verdict:           &verdict,
		Execution: &tasksit.ExecutionPhaseResult{
			Stdout:      "42\n",
			DurationMS:  12,
			MemoryBytes: 1024,
		},
		Tests: []tasksit.ExecutionTestResult{
			{
				TestID:      "open-1",
				Verdict:     "accepted",
				Stdout:      "42\n",
				DurationMS:  12,
				MemoryBytes: 1024,
			},
		},
		CreatedAt: "2026-08-13T12:00:00Z",
		UpdatedAt: "2026-08-13T12:00:01Z",
	}
}

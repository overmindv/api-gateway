package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

const maxCodeSourceSize = 256 << 10

type Service interface {
	ListPublished(context.Context, TaskFilter) (TaskList, error)
	GetPublished(context.Context, string) (Task, error)
	ListAdmin(context.Context, TaskFilter, Actor) (TaskList, error)
	GetAdmin(context.Context, string, Actor) (Task, error)
	Create(context.Context, TaskInput, Actor) (Task, error)
	Update(context.Context, string, TaskInput, Actor) (Task, error)
	ChangeStatus(context.Context, string, string, Actor) (Task, error)
	Delete(context.Context, string, Actor) error
	Submit(context.Context, string, SubmissionInput, Actor) (Submission, error)
	GetSubmission(context.Context, string, Actor) (Submission, error)
	ListMySubmissions(context.Context, *string, int, int, Actor) (SubmissionList, error)
	SubmitCode(context.Context, string, CodeSubmissionInput, Actor) (CodeSubmission, error)
	GetCodeSubmission(context.Context, string, Actor) (CodeSubmission, error)
	ListMyCodeSubmissions(context.Context, *string, int, int, Actor) (CodeSubmissionList, error)
	Health(context.Context) error
}

type CandidateService interface {
	ListCandidates(context.Context, CandidateFilter, Actor) (CandidateList, error)
	GetCandidate(context.Context, string, Actor) (Candidate, error)
	UpdateCandidate(context.Context, string, CandidateReview, Actor) (Candidate, error)
	ApproveCandidate(context.Context, string, CandidateReview, Actor) (Task, error)
	RejectCandidate(context.Context, string, int, string, Actor) (Candidate, error)
}

// ListCandidates получает очередь модерации с явными фильтрами.
func (c *Client) ListCandidates(ctx context.Context, filter CandidateFilter, actor Actor) (CandidateList, error) {
	values := url.Values{}
	values.Set("limit", strconv.Itoa(filter.Limit))
	values.Set("offset", strconv.Itoa(filter.Offset))
	if filter.Status != "" {
		values.Set("status", filter.Status)
	}
	if filter.SourceID != "" {
		values.Set("source_id", filter.SourceID)
	}
	if filter.Difficulty != "" {
		values.Set("difficulty", filter.Difficulty)
	}

	return request[CandidateList](c, ctx, http.MethodGet, pathWithQuery("/v1/admin/task-candidates", values), nil, actor)
}

// GetCandidate получает кандидата с immutable provenance.
func (c *Client) GetCandidate(ctx context.Context, id string, actor Actor) (Candidate, error) {
	return request[Candidate](c, ctx, http.MethodGet, "/v1/admin/task-candidates/"+url.PathEscape(id), nil, actor)
}

// UpdateCandidate сохраняет правки с optimistic revision.
func (c *Client) UpdateCandidate(ctx context.Context, id string, input CandidateReview, actor Actor) (Candidate, error) {
	return request[Candidate](c, ctx, http.MethodPut, "/v1/admin/task-candidates/"+url.PathEscape(id), input, actor)
}

// ApproveCandidate атомарно публикует programming-задачу.
func (c *Client) ApproveCandidate(ctx context.Context, id string, input CandidateReview, actor Actor) (Task, error) {
	return request[Task](c, ctx, http.MethodPost, "/v1/admin/task-candidates/"+url.PathEscape(id)+"/approve", input, actor)
}

// RejectCandidate завершает pending-кандидата.
func (c *Client) RejectCandidate(ctx context.Context, id string, revision int, reason string, actor Actor) (Candidate, error) {
	input := map[string]any{"expected_revision": revision, "reason": reason}

	return request[Candidate](c, ctx, http.MethodPost, "/v1/admin/task-candidates/"+url.PathEscape(id)+"/reject", input, actor)
}

type Client struct {
	baseURL string
	http    *http.Client
	log     *slog.Logger
}

// New создаёт HTTP-клиент внутреннего API tasks.
func New(baseURL string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
		log: log,
	}
}

// ListPublished получает опубликованные тесты без правильных ответов.
func (c *Client) ListPublished(ctx context.Context, filter TaskFilter) (TaskList, error) {
	return request[TaskList](c, ctx, http.MethodGet, pathWithQuery("/v1/tasks", filterQuery(filter, false)), nil, Actor{})
}

// GetPublished получает актуальную опубликованную версию теста.
func (c *Client) GetPublished(ctx context.Context, id string) (Task, error) {
	return request[Task](c, ctx, http.MethodGet, "/v1/tasks/"+url.PathEscape(id), nil, Actor{})
}

// ListAdmin получает административный список тестов.
func (c *Client) ListAdmin(ctx context.Context, filter TaskFilter, actor Actor) (TaskList, error) {
	return request[TaskList](c, ctx, http.MethodGet, pathWithQuery("/v1/admin/tasks", filterQuery(filter, true)), nil, actor)
}

// GetAdmin получает тест вместе с признаками правильных вариантов.
func (c *Client) GetAdmin(ctx context.Context, id string, actor Actor) (Task, error) {
	return request[Task](c, ctx, http.MethodGet, "/v1/admin/tasks/"+url.PathEscape(id), nil, actor)
}

// Create создаёт draft теста с первой версией.
func (c *Client) Create(ctx context.Context, input TaskInput, actor Actor) (Task, error) {
	return request[Task](c, ctx, http.MethodPost, "/v1/admin/tasks", input, actor)
}

// Update создаёт следующую версию теста.
func (c *Client) Update(ctx context.Context, id string, input TaskInput, actor Actor) (Task, error) {
	return request[Task](c, ctx, http.MethodPut, "/v1/admin/tasks/"+url.PathEscape(id), input, actor)
}

// ChangeStatus изменяет lifecycle-статус теста.
func (c *Client) ChangeStatus(ctx context.Context, id, status string, actor Actor) (Task, error) {
	return request[Task](c, ctx, http.MethodPatch, "/v1/admin/tasks/"+url.PathEscape(id)+"/status", map[string]string{"status": status}, actor)
}

// Delete выполняет soft delete теста.
func (c *Client) Delete(ctx context.Context, id string, actor Actor) error {
	_, err := request[struct{}](c, ctx, http.MethodDelete, "/v1/admin/tasks/"+url.PathEscape(id), nil, actor)
	if err != nil {
		return fmt.Errorf("delete tasks task: %w", err)
	}

	return nil
}

// Submit отправляет и сохраняет ответ пользователя.
func (c *Client) Submit(ctx context.Context, taskID string, input SubmissionInput, actor Actor) (Submission, error) {
	return request[Submission](c, ctx, http.MethodPost, "/v1/tasks/"+url.PathEscape(taskID)+"/submissions", input, actor)
}

// GetSubmission получает результат владельца или администратора.
func (c *Client) GetSubmission(ctx context.Context, id string, actor Actor) (Submission, error) {
	return request[Submission](c, ctx, http.MethodGet, "/v1/submissions/"+url.PathEscape(id), nil, actor)
}

// ListMySubmissions получает историю решений текущего пользователя.
func (c *Client) ListMySubmissions(ctx context.Context, taskID *string, limit, offset int, actor Actor) (SubmissionList, error) {
	values := url.Values{}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	if taskID != nil {
		values.Set("task_id", *taskID)
	}

	return request[SubmissionList](c, ctx, http.MethodGet, pathWithQuery("/v1/me/submissions", values), nil, actor)
}

// SubmitCode передаёт исходный файл решения в tasks как multipart/form-data.
func (c *Client) SubmitCode(ctx context.Context, taskID string, input CodeSubmissionInput, actor Actor) (CodeSubmission, error) {
	body, contentType, err := codeSubmissionBody(input)
	if err != nil {
		return CodeSubmission{}, fmt.Errorf("build tasks code submission: %w", err)
	}

	return requestWithBody[CodeSubmission](
		c,
		ctx,
		http.MethodPost,
		"/v1/tasks/"+url.PathEscape(taskID)+"/code-submissions",
		body,
		contentType,
		actor,
	)
}

// GetCodeSubmission получает актуальный результат проверки программного решения.
func (c *Client) GetCodeSubmission(ctx context.Context, id string, actor Actor) (CodeSubmission, error) {
	return request[CodeSubmission](c, ctx, http.MethodGet, "/v1/code-submissions/"+url.PathEscape(id), nil, actor)
}

// ListMyCodeSubmissions получает историю программных решений текущего пользователя.
func (c *Client) ListMyCodeSubmissions(ctx context.Context, taskID *string, limit, offset int, actor Actor) (CodeSubmissionList, error) {
	values := url.Values{}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	if taskID != nil {
		values.Set("task_id", *taskID)
	}

	return request[CodeSubmissionList](c, ctx, http.MethodGet, pathWithQuery("/v1/me/code-submissions", values), nil, actor)
}

// Health проверяет готовность tasks по HTTP-статусу /ready.
// Тело не декодируется: parker отдаёт на /ready простой текст, а не JSON.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ready", nil)
	if err != nil {
		return fmt.Errorf("create tasks health request: %w", err)
	}
	setHeaders(req, "", Actor{}, middleware.RequestID(ctx))
	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		c.log.WarnContext(ctx, "tasks health call failed", "request_id", middleware.RequestID(ctx), "error", err, "duration", time.Since(started))

		return fmt.Errorf("call tasks health: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)

		return fmt.Errorf("tasks readiness status %d", response.StatusCode)
	}
	c.log.InfoContext(ctx, "tasks health ok", "request_id", middleware.RequestID(ctx), "status", response.StatusCode, "duration", time.Since(started))

	return nil
}

// request выполняет типизированный запрос к tasks.
func request[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	body, err := requestBody(input)
	if err != nil {
		var zero T

		return zero, err
	}
	contentType := ""
	if input != nil {
		contentType = "application/json"
	}

	return requestWithBody[T](c, ctx, method, path, body, contentType, actor)
}

// requestWithBody выполняет типизированный запрос с уже подготовленным телом.
func requestWithBody[T any](c *Client, ctx context.Context, method, path string, body io.Reader, contentType string, actor Actor) (T, error) {
	var zero T

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return zero, fmt.Errorf("create tasks request: %w", err)
	}
	setHeaders(req, contentType, actor, middleware.RequestID(ctx))

	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		c.log.WarnContext(ctx, "tasks http call failed", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "error", err, "duration", time.Since(started))

		return zero, fmt.Errorf("call tasks: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return zero, c.decodeError(ctx, response, method, path, started)
	}
	if response.StatusCode == http.StatusNoContent {
		return zero, nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode tasks response: %w", err)
	}
	c.log.InfoContext(ctx, "tasks http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "duration", time.Since(started))

	return zero, nil
}

// requestBody кодирует необязательное JSON-тело запроса.
func requestBody(input any) (io.Reader, error) {
	if input == nil {
		return nil, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal tasks request: %w", err)
	}

	return bytes.NewReader(data), nil
}

// codeSubmissionBody формирует ограниченное multipart-тело для tasks.
// Решение передаётся ровно одним способом: кодом из консоли (source_code) либо файлом.
func codeSubmissionBody(input CodeSubmissionInput) (io.Reader, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fields := map[string]string{
		"task_version_id": input.TaskVersionID,
		"idempotency_key": input.IdempotencyKey,
		"language":        input.Language,
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, "", fmt.Errorf("write multipart field %s: %w", name, err)
		}
	}

	switch {
	case input.SourceCode != nil:
		if err := writer.WriteField("source_code", *input.SourceCode); err != nil {
			return nil, "", fmt.Errorf("write multipart field source_code: %w", err)
		}
	case input.File != nil:
		fileName := filepath.Base(input.FileName)
		if fileName == "." || fileName == "" {
			return nil, "", fmt.Errorf("source file name is required")
		}
		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			return nil, "", fmt.Errorf("create multipart source file: %w", err)
		}
		written, err := io.Copy(part, io.LimitReader(input.File, maxCodeSourceSize+1))
		if err != nil {
			return nil, "", fmt.Errorf("copy multipart source file: %w", err)
		}
		if written > maxCodeSourceSize {
			return nil, "", &Error{
				Code:       "INVALID_SOURCE_FILE",
				Message:    "файл решения превышает 262144 байта",
				StatusCode: http.StatusBadRequest,
			}
		}
	default:
		return nil, "", fmt.Errorf("source file or source code is required")
	}

	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}

	return body, writer.FormDataContentType(), nil
}

// setHeaders добавляет служебные и actor headers.
func setHeaders(req *http.Request, contentType string, actor Actor, requestID string) {
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	if actor.UserID != "" {
		req.Header.Set("X-User-ID", actor.UserID)
		req.Header.Set("X-User-Roles", strings.Join(actor.Roles, ","))
	}
}

// decodeError преобразует безопасную upstream-ошибку в typed error.
func (c *Client) decodeError(ctx context.Context, response *http.Response, method, path string, started time.Time) error {
	upstream := Error{StatusCode: response.StatusCode}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&upstream); err != nil {
		c.log.WarnContext(ctx, "tasks http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "error", err, "duration", time.Since(started))

		return fmt.Errorf("tasks returned HTTP %d: %w", response.StatusCode, err)
	}
	c.log.WarnContext(ctx, "tasks http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "code", upstream.Code, "duration", time.Since(started))

	return &upstream
}

// filterQuery кодирует явные фильтры и pagination.
func filterQuery(filter TaskFilter, includeStatus bool) url.Values {
	values := url.Values{}
	values.Set("limit", strconv.Itoa(filter.Limit))
	values.Set("offset", strconv.Itoa(filter.Offset))
	if includeStatus && filter.Status != "" {
		values.Set("status", filter.Status)
	}
	if filter.TaskType != "" {
		values.Set("task_type", filter.TaskType)
	}
	if filter.Difficulty != "" {
		values.Set("difficulty", filter.Difficulty)
	}
	if filter.TopicID != nil {
		values.Set("topic_id", *filter.TopicID)
	}

	return values
}

// pathWithQuery добавляет query string только при наличии параметров.
func pathWithQuery(path string, values url.Values) string {
	if len(values) == 0 {
		return path
	}

	return path + "?" + values.Encode()
}

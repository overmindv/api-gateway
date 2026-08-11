package tasksit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

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

// New создаёт HTTP-клиент внутреннего API tasks-it.
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
		return fmt.Errorf("delete tasks-it task: %w", err)
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

// Health проверяет готовность tasks-it и его PostgreSQL.
func (c *Client) Health(ctx context.Context) error {
	_, err := request[map[string]string](c, ctx, http.MethodGet, "/ready", nil, Actor{})
	if err != nil {
		return fmt.Errorf("check tasks-it readiness: %w", err)
	}

	return nil
}

// request выполняет типизированный запрос к tasks-it.
func request[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	var zero T
	body, err := requestBody(input)
	if err != nil {
		return zero, err
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return zero, fmt.Errorf("create tasks-it request: %w", err)
	}
	setHeaders(req, input != nil, actor, middleware.RequestID(ctx))
	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		c.log.WarnContext(ctx, "tasks-it http call failed", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "error", err, "duration", time.Since(started))

		return zero, fmt.Errorf("call tasks-it: %w", err)
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
		return zero, fmt.Errorf("decode tasks-it response: %w", err)
	}
	c.log.InfoContext(ctx, "tasks-it http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "duration", time.Since(started))

	return zero, nil
}

// requestBody кодирует необязательное JSON-тело запроса.
func requestBody(input any) (io.Reader, error) {
	if input == nil {
		return nil, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal tasks-it request: %w", err)
	}

	return bytes.NewReader(data), nil
}

// setHeaders добавляет служебные и actor headers.
func setHeaders(req *http.Request, hasBody bool, actor Actor, requestID string) {
	req.Header.Set("Accept", "application/json")
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
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
		c.log.WarnContext(ctx, "tasks-it http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "error", err, "duration", time.Since(started))

		return fmt.Errorf("tasks-it returned HTTP %d: %w", response.StatusCode, err)
	}
	c.log.WarnContext(ctx, "tasks-it http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "code", upstream.Code, "duration", time.Since(started))

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

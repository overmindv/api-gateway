package taskhunter

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
	Sources(context.Context, Actor) (Sources, error)
	ListJobs(context.Context, bool, int, int, Actor) (JobList, error)
	GetJob(context.Context, string, Actor) (Job, error)
	StartJob(context.Context, CreateJobInput, Actor) (Job, error)
	Acknowledge(context.Context, string, Actor) error
	Health(context.Context) error
}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
	log     *slog.Logger
}

// New создаёт защищённый internal client task-hunter.
func New(baseURL, token string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: timeout},
		log:     log,
	}
}

// Sources получает серверный allowlist доступных источников.
func (c *Client) Sources(ctx context.Context, actor Actor) (Sources, error) {
	return request[Sources](c, ctx, http.MethodGet, "/v1/admin/collection-sources", nil, actor)
}

// ListJobs возвращает журнал заданий или непрочитанные уведомления инициатора.
func (c *Client) ListJobs(ctx context.Context, unread bool, limit, offset int, actor Actor) (JobList, error) {
	values := url.Values{}
	values.Set("unread", strconv.FormatBool(unread))
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))

	return request[JobList](c, ctx, http.MethodGet, "/v1/admin/collection-jobs?"+values.Encode(), nil, actor)
}

// GetJob получает одно задание со статистикой его источников.
func (c *Client) GetJob(ctx context.Context, id string, actor Actor) (Job, error) {
	return request[Job](c, ctx, http.MethodGet, "/v1/admin/collection-jobs/"+url.PathEscape(id), nil, actor)
}

// StartJob ставит валидированный ручной сбор в очередь.
func (c *Client) StartJob(ctx context.Context, input CreateJobInput, actor Actor) (Job, error) {
	return request[Job](c, ctx, http.MethodPost, "/v1/admin/collection-jobs", input, actor)
}

// Acknowledge подтверждает показ terminal-уведомления инициатору.
func (c *Client) Acknowledge(ctx context.Context, id string, actor Actor) error {
	_, err := request[struct{}](c, ctx, http.MethodPost, "/v1/admin/collection-jobs/"+url.PathEscape(id)+"/acknowledge", struct{}{}, actor)
	if err != nil {
		return fmt.Errorf("acknowledge task collection job: %w", err)
	}

	return nil
}

// Health проверяет readiness внутреннего task-hunter.
func (c *Client) Health(ctx context.Context) error {
	_, err := request[map[string]string](c, ctx, http.MethodGet, "/ready", nil, Actor{})
	if err != nil {
		return fmt.Errorf("check task-hunter readiness: %w", err)
	}

	return nil
}

// request выполняет защищённый запрос и преобразует безопасную upstream-ошибку.
func request[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	var zero T
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return zero, fmt.Errorf("marshal task-hunter request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return zero, fmt.Errorf("create task-hunter request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor.UserID != "" {
		req.Header.Set("X-User-ID", actor.UserID)
		req.Header.Set("X-User-Roles", strings.Join(actor.Roles, ","))
	}
	if requestID := middleware.RequestID(ctx); requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("call task-hunter: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		upstream := Error{StatusCode: response.StatusCode}
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&upstream); err != nil {
			return zero, fmt.Errorf("task-hunter returned HTTP %d: %w", response.StatusCode, err)
		}

		return zero, &upstream
	}
	if response.StatusCode == http.StatusNoContent {
		return zero, nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode task-hunter response: %w", err)
	}
	c.log.InfoContext(ctx, "task-hunter http call", "method", method, "path", path, "status", response.StatusCode, "duration", time.Since(started))

	return zero, nil
}

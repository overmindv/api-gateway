package taskhunter

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
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
	http *http.Client
	hx   *httpx.Client
}

// New создаёт защищённый internal client task-hunter.
func New(baseURL, token string, timeout time.Duration, log *slog.Logger) *Client {
	httpClient := &http.Client{Timeout: timeout}

	return &Client{
		http: httpClient,
		hx: httpx.New("task-hunter", baseURL, httpClient, log, func(statusCode int) error {
			return &Error{StatusCode: statusCode}
		}, httpx.Header{Name: "Authorization", Value: "Bearer " + token}),
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
	values.Set("limit", strconv.Itoa(httpx.ClampLimit(limit)))
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

// Health проверяет готовность task-hunter по HTTP-статусу /ready.
// Тело не декодируется: parker отдаёт на /ready простой текст, а не JSON.
func (c *Client) Health(ctx context.Context) error {
	return c.hx.Health(ctx)
}

// request выполняет защищённый запрос и преобразует безопасную upstream-ошибку.
func request[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	return httpx.DoJSON[T](c.hx, ctx, method, path, input, hxActor(actor))
}

// hxActor приводит Actor клиента к httpx.Actor для проброса заголовков.
func hxActor(actor Actor) httpx.Actor {
	return httpx.Actor{UserID: actor.UserID, Roles: actor.Roles}
}

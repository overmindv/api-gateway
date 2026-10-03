package media

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
)

type Service interface {
	CreateUpload(context.Context, CreateUploadInput, Actor) (UploadTarget, error)
	CreateParts(context.Context, string, []int, Actor) ([]UploadPart, error)
	CompleteUpload(context.Context, string, []CompletedPart, Actor) (File, error)
	GetFile(context.Context, string, Actor) (File, error)
	ListFiles(context.Context, string, int, int, Actor) (FileList, error)
	DownloadURL(context.Context, string, string, Actor) (Download, error)
	DeleteFile(context.Context, string, Actor) error
	ResolvePublicFiles(context.Context, []string, []string) ([]PublicFile, error)
	Health(context.Context) error
}

type Client struct {
	http *http.Client
	hx   *httpx.Client
}

// New создаёт internal HTTP client Media.
func New(baseURL, token string, timeout time.Duration, log *slog.Logger) *Client {
	httpClient := &http.Client{Timeout: timeout}

	return &Client{
		http: httpClient,
		hx: httpx.New("media", baseURL, httpClient, log, func(_ int) error {
			return &Error{}
		}, httpx.Header{Name: "X-Media-Service-Token", Value: token}),
	}
}

func (c *Client) CreateUpload(ctx context.Context, input CreateUploadInput, actor Actor) (UploadTarget, error) {
	return request[UploadTarget](c, ctx, http.MethodPost, "/v1/uploads", input, actor)
}
func (c *Client) CreateParts(ctx context.Context, id string, numbers []int, actor Actor) ([]UploadPart, error) {
	return request[[]UploadPart](c, ctx, http.MethodPost, "/v1/uploads/"+url.PathEscape(id)+"/parts", map[string]any{"part_numbers": numbers}, actor)
}
func (c *Client) CompleteUpload(ctx context.Context, id string, parts []CompletedPart, actor Actor) (File, error) {
	return request[File](c, ctx, http.MethodPost, "/v1/uploads/"+url.PathEscape(id)+"/complete", map[string]any{"parts": parts}, actor)
}
func (c *Client) GetFile(ctx context.Context, id string, actor Actor) (File, error) {
	return request[File](c, ctx, http.MethodGet, "/v1/files/"+url.PathEscape(id), nil, actor)
}
func (c *Client) ListFiles(ctx context.Context, status string, limit, offset int, actor Actor) (FileList, error) {
	query := url.Values{}
	query.Set("status", status)
	query.Set("limit", strconv.Itoa(httpx.ClampLimit(limit)))
	query.Set("offset", strconv.Itoa(offset))
	return request[FileList](c, ctx, http.MethodGet, "/v1/files?"+query.Encode(), nil, actor)
}
func (c *Client) DownloadURL(ctx context.Context, id, variant string, actor Actor) (Download, error) {
	return request[Download](c, ctx, http.MethodPost, "/v1/files/"+url.PathEscape(id)+"/download-url", map[string]string{"variant": variant}, actor)
}
func (c *Client) DeleteFile(ctx context.Context, id string, actor Actor) error {
	_, err := request[map[string]bool](c, ctx, http.MethodDelete, "/v1/files/"+url.PathEscape(id), nil, actor)
	return err
}

// ResolvePublicFiles одним запросом получает CDN URLs аватаров.
func (c *Client) ResolvePublicFiles(ctx context.Context, ids, variants []string) ([]PublicFile, error) {
	return request[[]PublicFile](c, ctx, http.MethodPost, "/v1/internal/public-files/resolve", map[string]any{"file_ids": ids, "variants": variants}, Actor{})
}

// Health проверяет готовность Media по HTTP status (тело /ready может быть не JSON).
func (c *Client) Health(ctx context.Context) error {
	return c.hx.Health(ctx)
}

func request[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	return httpx.DoJSON[T](c.hx, ctx, method, path, input, hxActor(actor))
}

// hxActor приводит Actor клиента к httpx.Actor для проброса заголовков.
func hxActor(actor Actor) httpx.Actor {
	return httpx.Actor{UserID: actor.UserID, Roles: actor.Roles}
}

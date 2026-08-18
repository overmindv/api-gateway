package media

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
	baseURL string
	token   string
	http    *http.Client
	log     *slog.Logger
}

// New создаёт internal HTTP client Media.
func New(baseURL, token string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{Timeout: timeout}, log: log}
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
	query.Set("limit", strconv.Itoa(limit))
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
func (c *Client) Health(ctx context.Context) error {
	_, err := request[map[string]string](c, ctx, http.MethodGet, "/ready", nil, Actor{})
	return err
}

func request[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	var zero T
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return zero, fmt.Errorf("marshal media request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return zero, fmt.Errorf("create media request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Media-Service-Token", c.token)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requestID := middleware.RequestID(ctx); requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	if actor.UserID != "" {
		req.Header.Set("X-User-ID", actor.UserID)
		req.Header.Set("X-User-Roles", strings.Join(actor.Roles, ","))
	}
	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("call media: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var upstream Error
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&upstream); err != nil {
			return zero, fmt.Errorf("media returned HTTP %d", response.StatusCode)
		}
		return zero, &upstream
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode media response: %w", err)
	}
	c.log.InfoContext(ctx, "media http call", "method", method, "path", path, "status", response.StatusCode, "duration", time.Since(started))

	return zero, nil
}

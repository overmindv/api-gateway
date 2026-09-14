package feed

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

// FeedService — контракт чтения ленты активностей.
type FeedService interface {
	// ListFeed возвращает записи ленты по убыванию времени события.
	ListFeed(context.Context, int, int) (FeedList, error)
	Health(context.Context) error
}

// Item — запись ленты.
type Item struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Text        string `json:"text"`
	Href        string `json:"href"`
	ActorUserID string `json:"actorUserId"`
	OccurredAt  string `json:"occurredAt"`
}

// FeedList — страница ленты.
type FeedList struct {
	Items  []Item `json:"items"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Client — internal HTTP клиент сервиса Feed.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	log     *slog.Logger
}

// New создаёт internal HTTP client Feed.
func New(baseURL, token string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{Timeout: timeout}, log: log}
}

// ListFeed запрашивает страницу ленты активностей.
func (c *Client) ListFeed(ctx context.Context, limit, offset int) (FeedList, error) {
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))

	return request[FeedList](c, ctx, http.MethodGet, "/v1/feed?"+query.Encode(), nil)
}

// Health проверяет готовность сервиса Feed.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ready", nil)
	if err != nil {
		return fmt.Errorf("create feed health request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Feed-Service-Token", c.token)
	if requestID := middleware.RequestID(ctx); requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("feed health: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("feed health returned HTTP %d", response.StatusCode)
	}

	return nil
}

func request[T any](c *Client, ctx context.Context, method, path string, input any) (T, error) {
	var zero T
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return zero, fmt.Errorf("marshal feed request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return zero, fmt.Errorf("create feed request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Feed-Service-Token", c.token)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requestID := middleware.RequestID(ctx); requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("call feed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var upstream Error
		if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&upstream); err != nil {
			return zero, fmt.Errorf("feed returned HTTP %d", response.StatusCode)
		}
		return zero, &upstream
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode feed response: %w", err)
	}
	c.log.InfoContext(ctx, "feed http call", "method", method, "path", path, "status", response.StatusCode, "duration", time.Since(started))

	return zero, nil
}

// Error — ошибка upstream сервиса Feed.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error возвращает строковое представление upstream ошибки Feed.
func (e *Error) Error() string { return e.Code + ": " + e.Message }

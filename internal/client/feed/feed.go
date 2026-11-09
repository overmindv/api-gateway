package feed

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
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
	http *http.Client
	hx   *httpx.Client
}

// New создаёт internal HTTP client Feed.
func New(baseURL, token string, timeout time.Duration, log *slog.Logger) *Client {
	httpClient := &http.Client{Timeout: timeout}

	return &Client{
		http: httpClient,
		hx: httpx.New("feed", baseURL, httpClient, log, func(_ int) error {
			return &Error{}
		}, httpx.Header{Name: "X-Feed-Service-Token", Value: token}),
	}
}

// ListFeed запрашивает страницу ленты активностей.
func (c *Client) ListFeed(ctx context.Context, limit, offset int) (FeedList, error) {
	query := url.Values{}
	query.Set("limit", strconv.Itoa(httpx.ClampLimit(limit)))
	query.Set("offset", strconv.Itoa(offset))

	return request[FeedList](c, ctx, http.MethodGet, "/v1/feed?"+query.Encode(), nil)
}

// Health проверяет готовность сервиса Feed.
func (c *Client) Health(ctx context.Context) error {
	return c.hx.Health(ctx)
}

func request[T any](c *Client, ctx context.Context, method, path string, input any) (T, error) {
	return httpx.DoJSON[T](c.hx, ctx, method, path, input, httpx.Actor{})
}

// Error — ошибка upstream сервиса Feed.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error возвращает строковое представление upstream ошибки Feed.
func (e *Error) Error() string { return e.Code + ": " + e.Message }

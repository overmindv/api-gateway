// Package httpx содержит общие примитивы HTTP-клиентов внутренних сервисов:
// типизированные запросы, проброс actor-контекста в заголовки, декодирование
// безопасных upstream-ошибок, пагинационный clamp и проверку готовности.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

const (
	// DefaultLimit — значение limit по умолчанию для пагинирующих методов.
	DefaultLimit = 50
	// MinLimit — минимально допустимое значение limit.
	MinLimit = 1
	// MaxLimit — максимально допустимое значение limit.
	MaxLimit = 100

	maxResponseSize = 8 << 20
	maxErrorSize    = 1 << 20
	maxHealthSize   = 64 << 10
)

// ClampLimit приводит limit пагинации к диапазону [MinLimit, MaxLimit].
// Неположительные значения заменяются значением по умолчанию DefaultLimit.
func ClampLimit(limit int) int {
	if limit < MinLimit {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}

	return limit
}

// Actor — доверенный контекст пользователя, пробрасываемый нисходящим сервисам
// в заголовках X-User-ID и X-User-Roles. Заголовки добавляются только тогда,
// когда UserID непустой; роли склеиваются через запятую.
type Actor struct {
	UserID string
	Roles  []string
}

// Header — постоянный заголовок исходящих запросов (например, сервисный токен).
type Header struct {
	Name  string
	Value string
}

// NewError — фабрика ошибки upstream-сервиса. Вызывается при не-2xx ответе до
// декодирования тела и получает HTTP-статус, чтобы клиент сохранил его в своём
// типе ошибки (например tasks.Error{StatusCode}).
type NewError func(statusCode int) error

// Client — типизированный HTTP-клиент одного внутреннего сервиса.
type Client struct {
	name    string
	baseURL string
	headers []Header
	http    *http.Client
	log     *slog.Logger
	newErr  NewError
}

// New создаёт HTTP-клиент внутреннего сервиса. Принимает имя сервиса (для логов
// и текстов ошибок), базовый URL, HTTP-клиент, logger, фабрику ошибок и
// постоянные заголовки (сервисные токены и т.п.).
func New(name, baseURL string, httpClient *http.Client, log *slog.Logger, newErr NewError, headers ...Header) *Client {
	return &Client{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		headers: headers,
		http:    httpClient,
		log:     log,
		newErr:  newErr,
	}
}

// DoJSON выполняет типизированный REST-запрос, кодируя input как JSON-тело.
// Если input == nil, тело и Content-Type не добавляются.
func DoJSON[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	var body io.Reader
	contentType := ""
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			var zero T

			return zero, fmt.Errorf("marshal %s request: %w", c.name, err)
		}
		body = bytes.NewReader(data)
		contentType = "application/json"
	}

	return DoWithBody[T](c, ctx, method, path, body, contentType, actor)
}

// DoWithBody выполняет типизированный REST-запрос с уже подготовленным телом
// (например, multipart) и явным Content-Type.
func DoWithBody[T any](c *Client, ctx context.Context, method, path string, body io.Reader, contentType string, actor Actor) (T, error) {
	var zero T

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return zero, fmt.Errorf("create %s request: %w", c.name, err)
	}
	c.setHeaders(req, middleware.RequestID(ctx), contentType, actor)

	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		c.log.WarnContext(ctx, c.name+" http call failed", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "error", err, "duration", time.Since(started))

		return zero, fmt.Errorf("call %s: %w", c.name, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return zero, c.decodeUpstreamError(ctx, response, method, path, started)
	}
	if response.StatusCode == http.StatusNoContent {
		return zero, nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize)).Decode(&zero); err != nil {
		return zero, fmt.Errorf("decode %s response: %w", c.name, err)
	}
	c.log.InfoContext(ctx, c.name+" http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "duration", time.Since(started))

	return zero, nil
}

// Health проверяет готовность сервиса по HTTP-статусу /ready.
// Тело не декодируется: parker отдаёт на /ready простой текст, а не JSON.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ready", nil)
	if err != nil {
		return fmt.Errorf("create %s health request: %w", c.name, err)
	}
	c.setHeaders(req, middleware.RequestID(ctx), "", Actor{})

	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		c.log.WarnContext(ctx, c.name+" health call failed", "request_id", middleware.RequestID(ctx), "error", err, "duration", time.Since(started))

		return fmt.Errorf("call %s health: %w", c.name, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxHealthSize))

		return fmt.Errorf("%s readiness status %d", c.name, response.StatusCode)
	}
	c.log.InfoContext(ctx, c.name+" health ok", "request_id", middleware.RequestID(ctx), "status", response.StatusCode, "duration", time.Since(started))

	return nil
}

// setHeaders добавляет служебные, постоянные и actor заголовки.
func (c *Client) setHeaders(req *http.Request, requestID, contentType string, actor Actor) {
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, header := range c.headers {
		req.Header.Set(header.Name, header.Value)
	}
	if requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	if actor.UserID != "" {
		req.Header.Set("X-User-ID", actor.UserID)
		req.Header.Set("X-User-Roles", strings.Join(actor.Roles, ","))
	}
}

// decodeUpstreamError декодирует безопасное тело upstream-ошибки в тип из
// фабрики newErr и возвращает его как error, сохраняя технический код для
// ErrorPresenter. Тело не читается и не логируется целиком.
func (c *Client) decodeUpstreamError(ctx context.Context, response *http.Response, method, path string, started time.Time) error {
	upstream := c.newErr(response.StatusCode)
	if err := json.NewDecoder(io.LimitReader(response.Body, maxErrorSize)).Decode(upstream); err != nil {
		c.log.WarnContext(ctx, c.name+" http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "error", err, "duration", time.Since(started))

		return fmt.Errorf("%s returned HTTP %d: %w", c.name, response.StatusCode, err)
	}
	c.log.WarnContext(ctx, c.name+" http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "error", upstream, "duration", time.Since(started))

	return upstream
}

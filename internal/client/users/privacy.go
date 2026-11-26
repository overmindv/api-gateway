package users

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/overmindv/api-gateway/internal/middleware"
)

const (
	privacyExportTimeout = 30 * time.Second
	privacyBodyLimit     = 1 << 20
	privacyExportLimit   = 12 << 20
)

const (
	codePrivacyRequestInvalid = "PRIVACY_REQUEST_INVALID"
)

// PrivacyService расширяет Users API жизненным циклом экспорта и удаления.
type PrivacyService interface {
	PreparePrivacyRequest(context.Context, string, string) (PrivacyStart, error)
	ConfirmPrivacyRequest(context.Context, string, string) (PrivacyRequest, error)
	PrivacyRequestStatus(context.Context, string, string) (PrivacyRequest, error)
	AccountExport(context.Context, string) (AccountExport, error)
}

// PrivacyRequest — состояние запроса экспорта или удаления.
type PrivacyRequest struct {
	ID          string               `json:"id"`
	Kind        string               `json:"kind"`
	State       string               `json:"state"`
	Publication string               `json:"publication"`
	CreatedAt   string               `json:"created_at"`
	ExpiresAt   string               `json:"expires_at"`
	Owners      []PrivacyOwnerStatus `json:"owners"`
}

// PrivacyOwnerStatus — обработка удаления одним сервисом-владельцем.
type PrivacyOwnerStatus struct {
	Owner     string `json:"owner"`
	State     string `json:"state"`
	ErrorCode string `json:"error_code"`
}

// PrivacyStart — созданный запрос и одноразовый receipt.
// Receipt нельзя логировать или помещать в URL: он подтверждает владение запросом.
type PrivacyStart struct {
	Request PrivacyRequest `json:"request"`
	Receipt string         `json:"receipt"`
}

// AccountExportProfile — явный allowlist полей профиля.
type AccountExportProfile struct {
	ID                 string  `json:"id"`
	Email              string  `json:"email"`
	Username           string  `json:"username"`
	FirstName          string  `json:"first_name"`
	LastName           string  `json:"last_name"`
	BirthDate          *string `json:"birth_date"`
	Phone              *string `json:"phone"`
	AvatarFileID       *string `json:"avatar_file_id"`
	Timezone           string  `json:"timezone"`
	Locale             string  `json:"locale"`
	PublicName         bool    `json:"public_name"`
	LeaderboardVisible bool    `json:"leaderboard_visible"`
	CreatedAt          string  `json:"created_at"`
}

// AccountExportRecord — одна запись экспорта без credential-полей.
type AccountExportRecord struct {
	UserID     string `json:"user_id"`
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	OccurredAt string `json:"occurred_at"`
	ResourceID string `json:"resource_id"`
	Title      string `json:"title"`
	Text       string `json:"text"`
	Result     string `json:"result"`
}

// AccountExportPage — страница экспорта одного владельца.
type AccountExportPage struct {
	UserID     string                `json:"user_id"`
	Watermark  string                `json:"watermark"`
	NextCursor string                `json:"next_cursor"`
	Records    []AccountExportRecord `json:"records"`
}

// AccountExport — готовый артефакт: профиль и страницы по сервисам.
type AccountExport struct {
	Profile  AccountExportProfile           `json:"profile"`
	Services map[string][]AccountExportPage `json:"services"`
}

// PreparePrivacyRequest создаёт запрос экспорта или удаления после проверки пароля.
func (c *Client) PreparePrivacyRequest(ctx context.Context, kind, password string) (PrivacyStart, error) {
	var result PrivacyStart
	err := c.privacyCall(ctx, http.MethodPost, "/requests", map[string]string{"kind": kind, "password": password}, "", true, privacyBodyLimit, &result)

	return result, err
}

// ConfirmPrivacyRequest подтверждает запрос одноразовым receipt.
func (c *Client) ConfirmPrivacyRequest(ctx context.Context, id, receipt string) (PrivacyRequest, error) {
	var result PrivacyRequest
	path, err := privacyRequestPath(id, "/confirm")
	if err != nil {
		return result, err
	}
	err = c.privacyCall(ctx, http.MethodPost, path, map[string]bool{"confirm": true}, receipt, false, privacyBodyLimit, &result)

	return result, err
}

// PrivacyRequestStatus возвращает состояние запроса по receipt.
func (c *Client) PrivacyRequestStatus(ctx context.Context, id, receipt string) (PrivacyRequest, error) {
	var result PrivacyRequest
	path, err := privacyRequestPath(id, "")
	if err != nil {
		return result, err
	}
	err = c.privacyCall(ctx, http.MethodGet, path, nil, receipt, false, privacyBodyLimit, &result)

	return result, err
}

// AccountExport скачивает готовый артефакт владельца по активному access JWT.
func (c *Client) AccountExport(ctx context.Context, id string) (AccountExport, error) {
	var result AccountExport
	path, err := privacyRequestPath(id, "/export")
	if err != nil {
		return result, err
	}
	err = c.privacyCall(ctx, http.MethodGet, path, nil, "", true, privacyExportLimit, &result)

	return result, err
}

func privacyRequestPath(id, suffix string) (string, error) {
	if uuid.Validate(id) != nil {
		return "", &Error{Code: codePrivacyRequestInvalid, Message: "privacy request not found"}
	}

	return "/requests/" + url.PathEscape(id) + suffix, nil
}

func (c *Client) privacyCall(ctx context.Context, method, suffix string, body any, receipt string, authenticated bool, limit int64, target any) error {
	targetURL, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("parse users url: %w", err)
	}
	targetURL.Path = "/internal/privacy" + suffix
	targetURL.RawPath = ""
	targetURL.RawQuery = ""
	targetURL.Fragment = ""

	timeout := c.timeout
	if limit == privacyExportLimit {
		timeout = privacyExportTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var payload io.Reader
	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return fmt.Errorf("encode privacy request: %w", marshalErr)
		}
		payload = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(callCtx, method, targetURL.String(), payload)
	if err != nil {
		return fmt.Errorf("create privacy request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Cache-Control", "no-store")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if receipt != "" {
		request.Header.Set("X-Privacy-Receipt", receipt)
	}
	if authenticated {
		info, authErr := middleware.RequireAuth(ctx)
		if authErr != nil {
			return authErr
		}
		request.Header.Set("Authorization", "Bearer "+info.Token)
	}
	c.signClientIP(request)
	if requestID := middleware.RequestID(ctx); requestID != "" {
		request.Header.Set(middleware.RequestIDHeader, requestID)
	}

	client := *c.http
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return &Error{Code: codeDeadlineExceeded, Message: "privacy request timed out"}
		}

		return &Error{Code: codeServiceUnavailable, Message: "users is unavailable"}
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, privacyBodyLimit))

		return privacyStatusError(response.StatusCode)
	}
	if target == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, limit)).Decode(target); err != nil {
		return &Error{Code: codeBadGateway, Message: "users returned invalid privacy data"}
	}

	return nil
}

// privacyStatusError переводит HTTP-статус users в безопасный код без тела ответа.
func privacyStatusError(status int) error {
	code := codeBadGateway
	switch status {
	case http.StatusUnauthorized:
		code = codeUnauthenticated
	case http.StatusForbidden:
		code = "PERMISSION_DENIED"
	case http.StatusTooManyRequests:
		code = codeRateLimited
	case http.StatusConflict:
		code = codePrivacyRequestInvalid
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		code = codeServiceUnavailable
	}

	return &Error{Code: code, Message: "privacy request rejected"}
}

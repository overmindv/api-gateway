package users

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/overmindv/api-gateway/internal/middleware"
)

// CheckSession получает свежий session context из внутреннего Users API.
func (c *Client) CheckSession(ctx context.Context, token string) (middleware.SessionState, error) {
	var state middleware.SessionState
	err := c.sessionCall(ctx, "check", token, &state)

	return state, err
}

// RevokeSession отзывает текущую сессию на стороне Users.
func (c *Client) RevokeSession(ctx context.Context, token string) error {
	return c.sessionCall(ctx, "revoke", token, nil)
}

func (c *Client) sessionCall(ctx context.Context, action, token string, state *middleware.SessionState) error {
	target, err := url.Parse(c.url)
	if err != nil {
		return err
	}
	target.Path = "/internal/session/" + action
	target.RawPath = ""
	target.RawQuery = ""
	target.Fragment = ""
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := *c.http
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return &Error{Code: codeBadGateway, Message: "session check unavailable"}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return &Error{Code: "UNAUTHENTICATED", Message: "session rejected"}
	}
	expected := http.StatusOK
	if state == nil {
		expected = http.StatusNoContent
	}
	if resp.StatusCode != expected {
		return &Error{Code: codeBadGateway, Message: "session check unavailable"}
	}
	if state == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (16<<10)+1))
	if err != nil || len(data) > 16<<10 {
		return fmt.Errorf("invalid session response")
	}
	if err = json.Unmarshal(data, state); err != nil {
		return fmt.Errorf("invalid session response")
	}

	return nil
}

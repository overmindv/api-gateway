package arcee

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/overmindv/api-gateway/internal/config"
	"github.com/overmindv/api-gateway/internal/middleware"
)

const userFields = `id email username firstName lastName birthDate phone roles isAdmin isSuperuser createdAt updatedAt`

type Client struct {
	url       string
	healthURL string
	timeout   time.Duration
	http      *http.Client
	log       *slog.Logger
}

func New(cfg config.Arcee, log *slog.Logger) *Client {
	return &Client{
		url:       cfg.GraphQLURL,
		healthURL: cfg.HealthURL,
		timeout:   cfg.Timeout,
		http:      &http.Client{},
		log:       log,
	}
}

func (c *Client) Register(ctx context.Context, input RegisterInput) (*AuthPayload, error) {
	var data struct {
		Register *AuthPayload `json:"register"`
	}

	err := c.call(ctx, "Register", `mutation Register($input: RegisterInput!) { register(input: $input) { token expiresAt user { `+userFields+` } } }`, map[string]any{"input": input}, false, &data)
	if err != nil {
		return nil, err
	}

	if data.Register == nil {
		return nil, &Error{
			Code:    "BAD_GATEWAY",
			Message: "arcee returned an empty registration response",
		}
	}

	return data.Register, nil
}

func (c *Client) Login(ctx context.Context, input LoginInput) (*AuthPayload, error) {
	var data struct {
		Login *AuthPayload `json:"login"`
	}

	err := c.call(ctx, "Login", `mutation Login($input: LoginInput!) { login(input: $input) { token expiresAt user { `+userFields+` } } }`, map[string]any{"input": input}, false, &data)
	if err != nil {
		return nil, err
	}

	if data.Login == nil {
		return nil, &Error{
			Code:    "BAD_GATEWAY",
			Message: "arcee returned an empty login response",
		}
	}

	return data.Login, nil
}

func (c *Client) GetUser(ctx context.Context, id string) (*User, error) {
	var data struct {
		User *User `json:"user"`
	}

	err := c.call(ctx, "GetUser", `query GetUser($id: ID!) { user(id: $id) { `+userFields+` } }`, map[string]any{"id": id}, true, &data)
	if err != nil {
		return nil, err
	}

	if data.User == nil {
		return nil, &Error{
			Code:    "BAD_GATEWAY",
			Message: "arcee returned an empty user response",
		}
	}

	return data.User, nil
}

func (c *Client) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var data struct {
		UserByUsername *User `json:"userByUsername"`
	}

	err := c.call(ctx, "UserByUsername", `query UserByUsername($username: String!) { userByUsername(username: $username) { `+userFields+` } }`, map[string]any{"username": username}, true, &data)
	if err != nil {
		return nil, err
	}

	if data.UserByUsername == nil {
		return nil, &Error{
			Code:    "BAD_GATEWAY",
			Message: "arcee returned an empty user response",
		}
	}

	return data.UserByUsername, nil
}

func (c *Client) ListUsers(ctx context.Context, search string, limit, offset int) ([]*User, error) {
	var data struct {
		Users []*User `json:"users"`
	}

	err := c.call(ctx, "ListUsers", `query ListUsers($search: String, $limit: Int, $offset: Int) { users(search: $search, limit: $limit, offset: $offset) { `+userFields+` } }`, map[string]any{"search": search, "limit": limit, "offset": offset}, true, &data)

	return data.Users, err
}

func (c *Client) UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error) {
	var data struct {
		UpdateUser *User `json:"updateUser"`
	}

	err := c.call(ctx, "UpdateUser", `mutation UpdateUser($id: ID!, $input: UpdateUserInput!) { updateUser(id: $id, input: $input) { `+userFields+` } }`, map[string]any{"id": id, "input": input}, true, &data)
	if err != nil {
		return nil, err
	}

	if data.UpdateUser == nil {
		return nil, &Error{
			Code:    "BAD_GATEWAY",
			Message: "arcee returned an empty update response",
		}
	}

	return data.UpdateUser, nil
}

func (c *Client) DeleteUser(ctx context.Context, id string) (bool, error) {
	var data struct {
		DeleteUser bool `json:"deleteUser"`
	}

	err := c.call(ctx, "DeleteUser", `mutation DeleteUser($id: ID!) { deleteUser(id: $id) }`, map[string]any{"id": id}, true, &data)

	return data.DeleteUser, err
}

func (c *Client) SetUserAdmin(ctx context.Context, id string, admin bool) (*User, error) {
	var data struct {
		SetUserAdmin *User `json:"setUserAdmin"`
	}

	err := c.call(ctx, "SetUserAdmin", `mutation SetUserAdmin($id: ID!, $admin: Boolean!) { setUserAdmin(id: $id, admin: $admin) { `+userFields+` } }`, map[string]any{"id": id, "admin": admin}, true, &data)
	if err != nil {
		return nil, err
	}

	if data.SetUserAdmin == nil {
		return nil, &Error{
			Code:    "BAD_GATEWAY",
			Message: "arcee returned an empty admin response",
		}
	}

	return data.SetUserAdmin, nil
}

func (c *Client) SetUserAdminByUsername(ctx context.Context, username string, admin bool) (*User, error) {
	var data struct {
		SetUserAdminByUsername *User `json:"setUserAdminByUsername"`
	}

	err := c.call(ctx, "SetUserAdminByUsername", `mutation SetUserAdminByUsername($username: String!, $admin: Boolean!) { setUserAdminByUsername(username: $username, admin: $admin) { `+userFields+` } }`, map[string]any{"username": username, "admin": admin}, true, &data)
	if err != nil {
		return nil, err
	}

	if data.SetUserAdminByUsername == nil {
		return nil, &Error{
			Code:    "BAD_GATEWAY",
			Message: "arcee returned an empty admin response",
		}
	}

	return data.SetUserAdminByUsername, nil
}

func (c *Client) Health(ctx context.Context) error {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(callCtx, http.MethodGet, c.healthURL, nil)
	if err != nil {
		return err
	}

	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("arcee health returned HTTP %d", response.StatusCode)
	}

	return nil
}

type requestEnvelope struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}
type responseEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}
type graphQLError struct {
	Message    string         `json:"message"`
	Extensions map[string]any `json:"extensions"`
}

func (c *Client) call(ctx context.Context, operation, query string, variables map[string]any, authenticated bool, target any) error {
	started := time.Now()
	code := "OK"

	defer func() {
		c.log.InfoContext(ctx, "arcee graphql call", "request_id", middleware.RequestID(ctx), "operation", operation, "status", code, "duration", time.Since(started))
	}()

	body, err := json.Marshal(requestEnvelope{Query: query, Variables: variables})
	if err != nil {
		code = "INTERNAL_SERVER_ERROR"
		return fmt.Errorf("encode arcee request: %w", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		code = "INTERNAL_SERVER_ERROR"
		return fmt.Errorf("create arcee request: %w", err)
	}

	request.Header.Set("Content-Type", "application/json")
	if requestID := middleware.RequestID(ctx); requestID != "" {
		request.Header.Set(middleware.RequestIDHeader, requestID)
	}

	if authenticated {
		info, authErr := middleware.RequireAuth(ctx)
		if authErr != nil {
			code = "UNAUTHENTICATED"
			return authErr
		}
		request.Header.Set("Authorization", "Bearer "+info.Token)
	}

	response, err := c.http.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			code = "DEADLINE_EXCEEDED"
			return &Error{Code: code, Message: "arcee request timed out"}
		}

		code = "SERVICE_UNAVAILABLE"
		return &Error{Code: code, Message: "arcee is unavailable"}
	}

	defer func() { _ = response.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		code = "BAD_GATEWAY"
		return &Error{Code: code, Message: "cannot read arcee response"}
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		code = "BAD_GATEWAY"
		return &Error{Code: code, Message: fmt.Sprintf("arcee returned HTTP %d", response.StatusCode)}
	}

	var envelope responseEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		code = "BAD_GATEWAY"
		return &Error{Code: code, Message: "arcee returned invalid JSON"}
	}

	if len(envelope.Errors) > 0 {
		message := envelope.Errors[0].Message
		code = extensionCode(envelope.Errors[0].Extensions)
		return &Error{Code: code, Message: message}
	}

	if err := json.Unmarshal(envelope.Data, target); err != nil {
		code = "BAD_GATEWAY"
		return &Error{Code: code, Message: "arcee returned invalid data"}
	}

	return nil
}

func extensionCode(extensions map[string]any) string {
	if value, ok := extensions["code"].(string); ok && value != "" {
		return value
	}

	return "BAD_GATEWAY"
}

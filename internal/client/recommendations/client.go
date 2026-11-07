// Package recommendations описывает только HTTP-контракт сервиса рекомендаций.
package recommendations

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
)

type Request struct {
	TopicIDs       []string   `json:"topic_ids"`
	Minutes        int        `json:"minutes"`
	Topics         []Topic    `json:"topics"`
	Tasks          []Task     `json:"tasks"`
	Evidence       []Evidence `json:"evidence"`
	InputTruncated bool       `json:"input_truncated"`
}

type Topic struct {
	ID              string   `json:"id"`
	PrerequisiteIDs []string `json:"prerequisite_ids"`
}

type Task struct {
	ID            string `json:"id"`
	TaskVersionID string `json:"task_version_id"`
	TopicID       string `json:"topic_id"`
	Title         string `json:"title"`
	Difficulty    string `json:"difficulty"`
	TaskType      string `json:"task_type"`
}

type Evidence struct {
	TaskID        string `json:"task_id"`
	TaskVersionID string `json:"task_version_id"`
	Verdict       string `json:"verdict"`
	OccurredAt    string `json:"occurred_at"`
}

type Plan struct {
	PolicyVersion string   `json:"policy_version"`
	TotalMinutes  int      `json:"total_minutes"`
	Warnings      []string `json:"warnings"`
	Items         []Item   `json:"items"`
}

type Item struct {
	TaskID           string `json:"task_id"`
	TaskVersionID    string `json:"task_version_id"`
	TopicID          string `json:"topic_id"`
	Title            string `json:"title"`
	Difficulty       string `json:"difficulty"`
	TaskType         string `json:"task_type"`
	EstimatedMinutes int    `json:"estimated_minutes"`
	ReasonCode       string `json:"reason_code"`
}

// Error не сохраняет текст upstream-ошибки: он не должен попасть в логи и GraphQL.
type Error struct {
	Code string `json:"-"`
}

func (e *Error) Error() string { return e.Code }

type Service interface {
	PracticePlan(context.Context, Request, string) (Plan, error)
}
type Client struct{ hx *httpx.Client }

func New(baseURL, token string, timeout time.Duration, log *slog.Logger) *Client {
	client := &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{hx: httpx.New("recommendations", baseURL, client, log, func(status int) error {
		code := "SERVICE_UNAVAILABLE"
		if status == http.StatusBadRequest {
			code = "INVALID_ARGUMENT"
		}
		return &Error{Code: code}
	}, httpx.Header{Name: "Authorization", Value: "Bearer " + token})}
}

func (c *Client) PracticePlan(ctx context.Context, input Request, userID string) (Plan, error) {
	plan, err := httpx.DoJSON[Plan](c.hx, ctx, http.MethodPost, "/v1/practice-plan", input, httpx.Actor{UserID: userID})
	if err != nil {
		var upstream *Error
		if errors.As(err, &upstream) {
			return Plan{}, upstream
		}
		return Plan{}, &Error{Code: "SERVICE_UNAVAILABLE"}
	}
	return plan, nil
}

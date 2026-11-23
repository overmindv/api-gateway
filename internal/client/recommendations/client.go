// Package recommendations описывает только HTTP-контракт сервиса рекомендаций.
package recommendations

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
)

const (
	codeInvalidArgument    = "INVALID_ARGUMENT"
	codeConflict           = "CONFLICT"
	codeNotFound           = "NOT_FOUND"
	codeServiceUnavailable = "SERVICE_UNAVAILABLE"
	codeVersionConflict    = "VERSION_CONFLICT"
)

type Request struct {
	Goal           string     `json:"goal,omitempty"`
	TopicIDs       []string   `json:"topic_ids"`
	Minutes        int        `json:"minutes"`
	SessionCount   int        `json:"session_count,omitempty"`
	Topics         []Topic    `json:"topics"`
	Tasks          []Task     `json:"tasks"`
	Evidence       []Evidence `json:"evidence"`
	InputTruncated bool       `json:"input_truncated"`
}

type Topic struct {
	Title           string   `json:"title,omitempty"`
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
	AIStatus            string         `json:"ai_status"`
	PolicyVersion       string         `json:"policy_version"`
	TotalMinutes        int            `json:"total_minutes"`
	Warnings            []string       `json:"warnings"`
	Items               []Item         `json:"items"`
	RequestedSessions   int            `json:"requested_sessions"`
	TotalPlannedMinutes int            `json:"total_planned_minutes"`
	Sessions            []Session      `json:"sessions"`
	TopicInsights       []TopicInsight `json:"topic_insights"`
}

type Session struct {
	Number       int    `json:"number"`
	TotalMinutes int    `json:"total_minutes"`
	Items        []Item `json:"items"`
}

type TopicInsight struct {
	TopicID               string   `json:"topic_id"`
	IsTarget              bool     `json:"is_target"`
	AcceptedTasks         int      `json:"accepted_tasks"`
	FailedTasks           int      `json:"failed_tasks"`
	EligibleTasks         int      `json:"eligible_tasks"`
	PlannedTasks          int      `json:"planned_tasks"`
	RemainingToReadiness  int      `json:"remaining_to_readiness"`
	RecommendedDifficulty string   `json:"recommended_difficulty"`
	MinimumMinutes        int      `json:"minimum_minutes"`
	BlockedBy             []string `json:"blocked_by"`
	NextAction            string   `json:"next_action"`
}

type Goal struct {
	TopicIDs     []string `json:"topic_ids"`
	Minutes      int      `json:"minutes"`
	SessionCount int      `json:"session_count,omitempty"`
	Revision     int64    `json:"revision"`
}

type GoalInput struct {
	TopicIDs     []string `json:"topic_ids"`
	Minutes      int      `json:"minutes"`
	SessionCount int      `json:"session_count,omitempty"`
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
	Code           string `json:"-"`
	acceptUpstream bool
}

func (e *Error) Error() string { return e.Code }

func (e *Error) UnmarshalJSON(data []byte) error {
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if e.acceptUpstream && safeRecommendationCode(payload.Code) {
		e.Code = payload.Code
	}

	return nil
}

func safeRecommendationCode(code string) bool {
	switch code {
	case "BUDGET_EXCEEDED", "DIAGNOSTIC_BUDGET", "DIAGNOSTIC_NOT_STARTED", "DIAGNOSTIC_SESSION_CHANGED",
		"DIAGNOSTIC_TASK_UNAVAILABLE", "GOAL_LIMIT", "GRADE_PENDING", "IDEMPOTENCY_CONFLICT", "INFEASIBLE_GOAL",
		codeInvalidArgument, codeNotFound, "PERMISSION_DENIED", "PLAN_LIMIT", "PROFILE_REQUIRED", "RESOURCE_UNAVAILABLE",
		"REVIEW_NOT_DUE", codeServiceUnavailable, "STEP_BLOCKED", "UNKNOWN_TEMPLATE", "UNKNOWN_TOPIC", codeVersionConflict, "WRITE_LIMIT":
		return true
	default:
		return false
	}
}

type Service interface {
	PracticePlan(context.Context, Request, string) (Plan, error)
}

type GoalService interface {
	GetGoal(context.Context, string) (*Goal, error)
	SaveGoal(context.Context, GoalInput, string) (Goal, error)
	DeleteGoal(context.Context, string) error
}
type Client struct {
	hx        *httpx.Client
	learnerHx *httpx.Client
}

func New(baseURL, token string, timeout time.Duration, log *slog.Logger) *Client {
	client := &http.Client{Timeout: timeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}

	serviceHeader := httpx.Header{Name: "Authorization", Value: "Bearer " + token}
	legacy := httpx.New("recommendations", baseURL, client, log, func(status int) error {
		code := codeServiceUnavailable
		switch status {
		case http.StatusBadRequest:
			code = codeInvalidArgument
		case http.StatusConflict:
			code = codeConflict
		case http.StatusTooManyRequests:
			code = "LIMIT_REACHED"
		case http.StatusNotFound:
			code = codeNotFound
		}

		return &Error{Code: code}
	}, serviceHeader)
	learner := httpx.New("recommendations", baseURL, client, log, func(int) error {
		return &Error{Code: codeServiceUnavailable, acceptUpstream: true}
	}, serviceHeader)

	return &Client{hx: legacy, learnerHx: learner}
}

func (c *Client) GetGoal(ctx context.Context, userID string) (*Goal, error) {
	goal, err := httpx.DoJSON[Goal](c.hx, ctx, http.MethodGet, "/v1/practice-goal", nil, httpx.Actor{UserID: userID})
	if err != nil {
		var upstream *Error
		if errors.As(err, &upstream) && upstream.Code == codeNotFound {
			return nil, nil
		}

		return nil, goalError(err)
	}

	return &goal, nil
}

func (c *Client) SaveGoal(ctx context.Context, input GoalInput, userID string) (Goal, error) {
	goal, err := httpx.DoJSON[Goal](c.hx, ctx, http.MethodPut, "/v1/practice-goal", input, httpx.Actor{UserID: userID})
	if err != nil {
		return Goal{}, goalError(err)
	}

	return goal, nil
}

func (c *Client) DeleteGoal(ctx context.Context, userID string) error {
	_, err := httpx.DoJSON[struct{}](c.hx, ctx, http.MethodDelete, "/v1/practice-goal", nil, httpx.Actor{UserID: userID})

	return goalError(err)
}

func goalError(err error) error {
	if err == nil {
		return nil
	}
	var upstream *Error
	if errors.As(err, &upstream) {
		return upstream
	}

	return &Error{Code: codeServiceUnavailable}
}

func (c *Client) PracticePlan(ctx context.Context, input Request, userID string) (Plan, error) {
	plan, err := httpx.DoJSON[Plan](c.hx, ctx, http.MethodPost, "/v1/practice-plan", input, httpx.Actor{UserID: userID})
	if err != nil {
		var upstream *Error
		if errors.As(err, &upstream) {
			return Plan{}, upstream
		}

		return Plan{}, &Error{Code: codeServiceUnavailable}
	}

	return plan, nil
}

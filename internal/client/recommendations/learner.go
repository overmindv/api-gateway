package recommendations

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type GoalTemplate struct {
	Version          string `json:"version"`
	Title            string `json:"title"`
	SuggestsDeadline bool   `json:"suggests_deadline"`
}

type AvailabilityInput struct {
	WeeklyMinutes int    `json:"weekly_minutes"`
	Timezone      string `json:"timezone"`
}

type Availability struct {
	WeeklyMinutes int       `json:"weekly_minutes"`
	Timezone      string    `json:"timezone"`
	Version       int64     `json:"version"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type LearnerProfile struct {
	Availability *Availability `json:"availability"`
}

type SelfAssessment struct {
	TopicID string `json:"topic_id"`
	Level   string `json:"level"`
}

type LearnerGoalInput struct {
	Title           string           `json:"title"`
	TemplateVersion string           `json:"template_version,omitempty"`
	TargetTopics    []string         `json:"target_topics"`
	Deadline        string           `json:"deadline,omitempty"`
	WeeklyMinutes   int              `json:"weekly_minutes"`
	Active          bool             `json:"active"`
	Languages       []string         `json:"languages"`
	Formats         []string         `json:"formats"`
	SelfAssessments []SelfAssessment `json:"self_assessments"`
}

type TopicSkill struct {
	TopicID string `json:"topic_id"`
	State   string `json:"state"`
}

type LearnerGoal struct {
	ID string `json:"id"`
	LearnerGoalInput
	Timezone          string       `json:"timezone"`
	Version           int64        `json:"version"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	Skills            []TopicSkill `json:"skills"`
	Feasibility       string       `json:"feasibility"`
	FeasibilityReason string       `json:"feasibility_reason,omitempty"`
}

type LearnerService interface {
	GoalTemplates(context.Context) ([]GoalTemplate, error)
	GetLearnerProfile(context.Context) (LearnerProfile, error)
	SaveLearnerProfile(context.Context, AvailabilityInput, string, int64) (LearnerProfile, error)
	ListLearnerGoals(context.Context) ([]LearnerGoal, error)
	GetLearnerGoal(context.Context, string) (LearnerGoal, error)
	CreateLearnerGoal(context.Context, LearnerGoalInput, string) (LearnerGoal, error)
	UpdateLearnerGoal(context.Context, string, LearnerGoalInput, string, int64) (LearnerGoal, error)
	DeleteLearnerGoal(context.Context, string, string, int64) error
}

func (c *Client) GoalTemplates(ctx context.Context) ([]GoalTemplate, error) {
	return learnerCall[[]GoalTemplate](c, ctx, http.MethodGet, "/v1/learner/goal-templates", nil)
}

func (c *Client) GetLearnerProfile(ctx context.Context) (LearnerProfile, error) {
	return learnerCall[LearnerProfile](c, ctx, http.MethodGet, "/v1/learner/profile", nil)
}

func (c *Client) SaveLearnerProfile(ctx context.Context, input AvailabilityInput, requestID string, expectedVersion int64) (LearnerProfile, error) {
	return learnerWrite[LearnerProfile](c, ctx, http.MethodPut, "/v1/learner/profile", input, requestID, &expectedVersion)
}

func (c *Client) ListLearnerGoals(ctx context.Context) ([]LearnerGoal, error) {
	return learnerCall[[]LearnerGoal](c, ctx, http.MethodGet, "/v1/learner/goals", nil)
}

func (c *Client) GetLearnerGoal(ctx context.Context, id string) (LearnerGoal, error) {
	return learnerCall[LearnerGoal](c, ctx, http.MethodGet, learnerGoalPath(id), nil)
}

func (c *Client) CreateLearnerGoal(ctx context.Context, input LearnerGoalInput, requestID string) (LearnerGoal, error) {
	return learnerWrite[LearnerGoal](c, ctx, http.MethodPost, "/v1/learner/goals", input, requestID, nil)
}

func (c *Client) UpdateLearnerGoal(ctx context.Context, id string, input LearnerGoalInput, requestID string, expectedVersion int64) (LearnerGoal, error) {
	return learnerWrite[LearnerGoal](c, ctx, http.MethodPut, learnerGoalPath(id), input, requestID, &expectedVersion)
}

func (c *Client) DeleteLearnerGoal(ctx context.Context, id, requestID string, expectedVersion int64) error {
	_, err := learnerWrite[struct{}](c, ctx, http.MethodDelete, learnerGoalPath(id), nil, requestID, &expectedVersion)

	return err
}

func learnerGoalPath(id string) string { return "/v1/learner/goals/" + url.PathEscape(id) }

func learnerCall[T any](c *Client, ctx context.Context, method, path string, input any) (T, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		var zero T

		return zero, err
	}
	value, err := httpx.DoJSON[T](c.learnerHx, ctx, method, path, input, httpx.Actor{UserID: auth.UserID})
	if err != nil {
		var zero T

		return zero, goalError(err)
	}

	return value, nil
}

func learnerWrite[T any](c *Client, ctx context.Context, method, path string, input any, requestID string, expectedVersion *int64) (T, error) {
	headers := []httpx.Header{{Name: "Idempotency-Key", Value: requestID}}
	if expectedVersion != nil {
		headers = append(headers, httpx.Header{Name: "X-Expected-Version", Value: strconv.FormatInt(*expectedVersion, 10)})
	}
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		var zero T

		return zero, err
	}
	value, err := httpx.DoJSONWithHeaders[T](c.learnerHx, ctx, method, path, input, httpx.Actor{UserID: auth.UserID}, headers...)
	if err != nil {
		var zero T

		return zero, goalError(err)
	}

	return value, nil
}

package recommendations

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
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
	GoalTemplates(context.Context, string) ([]GoalTemplate, error)
	GetLearnerProfile(context.Context, string) (LearnerProfile, error)
	SaveLearnerProfile(context.Context, AvailabilityInput, string, int64, string) (LearnerProfile, error)
	ListLearnerGoals(context.Context, string) ([]LearnerGoal, error)
	GetLearnerGoal(context.Context, string, string) (LearnerGoal, error)
	CreateLearnerGoal(context.Context, LearnerGoalInput, string, string) (LearnerGoal, error)
	UpdateLearnerGoal(context.Context, string, LearnerGoalInput, string, int64, string) (LearnerGoal, error)
	DeleteLearnerGoal(context.Context, string, string, int64, string) error
}

func (c *Client) GoalTemplates(ctx context.Context, actor string) ([]GoalTemplate, error) {
	return learnerCall[[]GoalTemplate](c, ctx, http.MethodGet, "/v1/learner/goal-templates", nil, actor)
}

func (c *Client) GetLearnerProfile(ctx context.Context, actor string) (LearnerProfile, error) {
	return learnerCall[LearnerProfile](c, ctx, http.MethodGet, "/v1/learner/profile", nil, actor)
}

func (c *Client) SaveLearnerProfile(ctx context.Context, input AvailabilityInput, requestID string, expectedVersion int64, actor string) (LearnerProfile, error) {
	return learnerWrite[LearnerProfile](c, ctx, http.MethodPut, "/v1/learner/profile", input, requestID, &expectedVersion, actor)
}

func (c *Client) ListLearnerGoals(ctx context.Context, actor string) ([]LearnerGoal, error) {
	return learnerCall[[]LearnerGoal](c, ctx, http.MethodGet, "/v1/learner/goals", nil, actor)
}

func (c *Client) GetLearnerGoal(ctx context.Context, id, actor string) (LearnerGoal, error) {
	return learnerCall[LearnerGoal](c, ctx, http.MethodGet, learnerGoalPath(id), nil, actor)
}

func (c *Client) CreateLearnerGoal(ctx context.Context, input LearnerGoalInput, requestID, actor string) (LearnerGoal, error) {
	return learnerWrite[LearnerGoal](c, ctx, http.MethodPost, "/v1/learner/goals", input, requestID, nil, actor)
}

func (c *Client) UpdateLearnerGoal(ctx context.Context, id string, input LearnerGoalInput, requestID string, expectedVersion int64, actor string) (LearnerGoal, error) {
	return learnerWrite[LearnerGoal](c, ctx, http.MethodPut, learnerGoalPath(id), input, requestID, &expectedVersion, actor)
}

func (c *Client) DeleteLearnerGoal(ctx context.Context, id, requestID string, expectedVersion int64, actor string) error {
	_, err := learnerWrite[struct{}](c, ctx, http.MethodDelete, learnerGoalPath(id), nil, requestID, &expectedVersion, actor)

	return err
}

func learnerGoalPath(id string) string { return "/v1/learner/goals/" + url.PathEscape(id) }

func learnerCall[T any](c *Client, ctx context.Context, method, path string, input any, actor string) (T, error) {
	value, err := httpx.DoJSON[T](c.hx, ctx, method, path, input, httpx.Actor{UserID: actor})
	if err != nil {
		var zero T

		return zero, goalError(err)
	}

	return value, nil
}

func learnerWrite[T any](c *Client, ctx context.Context, method, path string, input any, requestID string, expectedVersion *int64, actor string) (T, error) {
	headers := []httpx.Header{{Name: "Idempotency-Key", Value: requestID}}
	if expectedVersion != nil {
		headers = append(headers, httpx.Header{Name: "X-Expected-Version", Value: strconv.FormatInt(*expectedVersion, 10)})
	}
	value, err := httpx.DoJSONWithHeaders[T](c.hx, ctx, method, path, input, httpx.Actor{UserID: actor}, headers...)
	if err != nil {
		var zero T

		return zero, goalError(err)
	}

	return value, nil
}

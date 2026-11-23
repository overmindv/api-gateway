package recommendations

import (
	"context"
	"net/http"
	"time"
)

type DiagnosticTask struct {
	TaskID           string `json:"task_id"`
	TaskVersionID    string `json:"task_version_id"`
	TopicID          string `json:"topic_id"`
	EstimatedMinutes int    `json:"estimated_minutes"`
}

type DiagnosticStopRule struct {
	PolicyVersion string `json:"policy_version"`
	MaxTasks      int    `json:"max_tasks"`
	BudgetMinutes int    `json:"budget_minutes"`
}

type DiagnosticTopicCoverage struct {
	TopicID     string `json:"topic_id"`
	Assigned    int    `json:"assigned"`
	Assessed    int    `json:"assessed"`
	Observation string `json:"observation"`
	Uncertainty string `json:"uncertainty"`
}

type DiagnosticOutcome struct {
	TaskID            string    `json:"task_id"`
	TaskVersionID     string    `json:"task_version_id"`
	TopicID           string    `json:"topic_id"`
	SubmissionID      string    `json:"submission_id"`
	EventID           string    `json:"event_id"`
	CorrectionVersion int64     `json:"correction_version"`
	Verdict           string    `json:"verdict"`
	GradedAt          time.Time `json:"graded_at"`
}

type DiagnosticReport struct {
	Assigned int                       `json:"assigned"`
	Assessed int                       `json:"assessed"`
	Topics   []DiagnosticTopicCoverage `json:"topics"`
	Outcomes []DiagnosticOutcome       `json:"outcomes"`
}

type DiagnosticSession struct {
	ID               string             `json:"id"`
	GoalID           string             `json:"goal_id"`
	GoalVersion      int64              `json:"goal_version"`
	Status           string             `json:"status"`
	Baseline         string             `json:"baseline"`
	Tasks            []DiagnosticTask   `json:"tasks"`
	EstimatedMinutes int                `json:"estimated_minutes"`
	StopRule         DiagnosticStopRule `json:"stop_rule"`
	StartedAt        time.Time          `json:"started_at"`
	ExpiresAt        time.Time          `json:"expires_at"`
	SkippedAt        *time.Time         `json:"skipped_at,omitempty"`
	FinalizedAt      *time.Time         `json:"finalized_at,omitempty"`
	Report           *DiagnosticReport  `json:"report,omitempty"`
}

type DiagnosticStartInput struct {
	GoalVersion   int64 `json:"goal_version"`
	BudgetMinutes int   `json:"budget_minutes"`
}

type DiagnosticSkipInput struct {
	GoalVersion int64 `json:"goal_version"`
}

type DiagnosticFinalizeInput struct {
	GoalVersion  int64  `json:"goal_version"`
	SessionID    string `json:"session_id"`
	AllowPartial bool   `json:"allow_partial"`
}

type DiagnosticService interface {
	GetDiagnostic(context.Context, string) (DiagnosticSession, error)
	StartDiagnostic(context.Context, string, DiagnosticStartInput) (DiagnosticSession, error)
	SkipDiagnostic(context.Context, string, DiagnosticSkipInput) (DiagnosticSession, error)
	FinalizeDiagnostic(context.Context, string, DiagnosticFinalizeInput) (DiagnosticSession, error)
}

func (c *Client) GetDiagnostic(ctx context.Context, goalID string) (DiagnosticSession, error) {
	return learnerCall[DiagnosticSession](c, ctx, http.MethodGet, learnerGoalPath(goalID)+"/diagnostic", nil)
}

func (c *Client) StartDiagnostic(ctx context.Context, goalID string, input DiagnosticStartInput) (DiagnosticSession, error) {
	return learnerCall[DiagnosticSession](c, ctx, http.MethodPut, learnerGoalPath(goalID)+"/diagnostic", input)
}

func (c *Client) SkipDiagnostic(ctx context.Context, goalID string, input DiagnosticSkipInput) (DiagnosticSession, error) {
	return learnerCall[DiagnosticSession](c, ctx, http.MethodPut, learnerGoalPath(goalID)+"/diagnostic/skip", input)
}

func (c *Client) FinalizeDiagnostic(ctx context.Context, goalID string, input DiagnosticFinalizeInput) (DiagnosticSession, error) {
	return learnerCall[DiagnosticSession](c, ctx, http.MethodPut, learnerGoalPath(goalID)+"/diagnostic/finalize", input)
}

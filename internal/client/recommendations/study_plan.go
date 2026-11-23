package recommendations

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
)

type StudyPlanFactors struct {
	Gap        int `json:"gap"`
	Difficulty int `json:"difficulty"`
	Duration   int `json:"duration"`
	Variety    int `json:"variety"`
	DueReview  int `json:"due_review"`
}

type StudyPlanRankingPolicy struct {
	Version             string           `json:"policy_version"`
	TargetBPS           int              `json:"target_bps"`
	UnknownGapBPS       int              `json:"unknown_gap_bps"`
	PrerequisiteMinBPS  int              `json:"prerequisite_min_bps"`
	MediumDifficultyBPS int              `json:"medium_difficulty_bps"`
	HardDifficultyBPS   int              `json:"hard_difficulty_bps"`
	Weights             StudyPlanFactors `json:"weights"`
}

type StudyPlanEvidencePolicy struct {
	Revision         string `json:"revision"`
	Algorithm        string `json:"algorithm"`
	GapMaxBPS        int    `json:"gap_max_bps"`
	DemonstratedBPS  int    `json:"demonstrated_min_bps"`
	MinWeightedTasks int    `json:"min_weighted_tasks"`
}

type StudyPlanAssessment struct {
	Verdict  string    `json:"verdict"`
	GradedAt time.Time `json:"graded_at"`
}

type StudyPlanItem struct {
	Kind              string               `json:"kind"`
	ResourceID        string               `json:"resource_id"`
	RevisionID        string               `json:"revision_id"`
	TopicID           string               `json:"topic_id"`
	EstimatedMinutes  int                  `json:"estimated_minutes"`
	Score             int                  `json:"score"`
	Components        StudyPlanFactors     `json:"components"`
	Stage             string               `json:"stage"`
	RequiresChecks    []string             `json:"requires_checks,omitempty"`
	Superseded        bool                 `json:"superseded,omitempty"`
	Ordinal           int                  `json:"ordinal"`
	ActivityStartedAt *time.Time           `json:"activity_started_at,omitempty"`
	ActivityDoneAt    *time.Time           `json:"activity_completed_at,omitempty"`
	Assessment        *StudyPlanAssessment `json:"assessment,omitempty"`
	Skipped           bool                 `json:"skipped,omitempty"`
}

type StudyPlanWeek struct {
	From            string          `json:"from"`
	Through         string          `json:"through"`
	CapacityMinutes int             `json:"capacity_minutes"`
	PlannedMinutes  int             `json:"planned_minutes"`
	Items           []StudyPlanItem `json:"items"`
}

type StudyPlanUnmetOutcome struct {
	TopicID   string   `json:"topic_id"`
	Stage     string   `json:"stage,omitempty"`
	Reason    string   `json:"reason"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

type StudyPlanSuggestion struct {
	Kind     string   `json:"kind"`
	TopicIDs []string `json:"topic_ids,omitempty"`
	Deadline string   `json:"deadline,omitempty"`
}

type StudyPlanGuidance struct {
	Basis               string `json:"basis"`
	TemplateVersion     string `json:"template_version"`
	VerifiedTopics      int    `json:"verified_topics"`
	RecommendDiagnostic bool   `json:"recommend_diagnostic"`
}

type StudyPlanFreshness struct {
	Status     string     `json:"status"`
	CheckedAt  *time.Time `json:"checked_at,omitempty"`
	EntitiesAt *time.Time `json:"entities_synced_at,omitempty"`
	TasksAt    *time.Time `json:"tasks_synced_at,omitempty"`
	ContentAt  *time.Time `json:"content_synced_at,omitempty"`
}

type StudyPlan struct {
	GoalID              string                  `json:"goal_id"`
	Version             int64                   `json:"version"`
	GoalVersion         int64                   `json:"goal_version"`
	AvailabilityVersion int64                   `json:"availability_version"`
	TargetTopics        []string                `json:"target_topics"`
	PlannerVersion      string                  `json:"planner_version"`
	RankingAlgorithm    string                  `json:"ranking_algorithm"`
	RankingPolicy       StudyPlanRankingPolicy  `json:"ranking_policy"`
	EvidenceWatermark   string                  `json:"evidence_watermark"`
	EvidencePolicy      StudyPlanEvidencePolicy `json:"evidence_policy"`
	CreatedAt           time.Time               `json:"created_at"`
	Timezone            string                  `json:"timezone"`
	WeeklyMinutes       int                     `json:"weekly_minutes"`
	HorizonLimited      bool                    `json:"horizon_limited"`
	Weeks               []StudyPlanWeek         `json:"weeks"`
	UnmetOutcomes       []StudyPlanUnmetOutcome `json:"unmet_outcomes"`
	CoverageGaps        []StudyPlanUnmetOutcome `json:"coverage_gaps,omitempty"`
	Guidance            *StudyPlanGuidance      `json:"guidance,omitempty"`
	Suggestions         []StudyPlanSuggestion   `json:"suggestions"`
	Current             bool                    `json:"current"`
	Stale               bool                    `json:"stale"`
	Freshness           *StudyPlanFreshness     `json:"freshness,omitempty"`
	PausedAt            *time.Time              `json:"paused_at,omitempty"`
	NextStep            *int                    `json:"next_step"`
	BlockedReason       string                  `json:"blocked_reason,omitempty"`
	GoalDemonstrated    bool                    `json:"goal_demonstrated"`
}

type StudyPlanGenerationInput struct {
	GoalVersion         int64 `json:"goal_version"`
	AvailabilityVersion int64 `json:"availability_version"`
	PlanVersion         int64 `json:"plan_version"`
}

type StudyPlanFeedbackInput struct {
	PlanVersion   int64  `json:"plan_version"`
	Action        string `json:"action"`
	Ordinal       *int   `json:"ordinal"`
	WeeklyMinutes *int   `json:"weekly_minutes"`
}

type StudyPlanFeedbackReceipt struct {
	RequestID     string    `json:"request_id"`
	PlanVersion   int64     `json:"plan_version"`
	Action        string    `json:"action"`
	Ordinal       *int      `json:"ordinal,omitempty"`
	WeeklyMinutes *int      `json:"weekly_minutes,omitempty"`
	AcceptedAt    time.Time `json:"accepted_at"`
}

type StudyPlanReason struct {
	Code    string `json:"code"`
	TopicID string `json:"topic_id,omitempty"`
	Minutes int    `json:"minutes,omitempty"`
	Message string `json:"message"`
}

type StudyPlanEvidenceRef struct {
	Watermark      string    `json:"watermark"`
	PolicyRevision string    `json:"policy_revision"`
	CapturedAt     time.Time `json:"captured_at"`
}

type StudyPlanEvidence struct {
	TopicID        string                `json:"topic_id"`
	ScoreBPS       *int                  `json:"score_bps,omitempty"`
	SampleCount    int                   `json:"sample_count"`
	LastEvidenceAt *time.Time            `json:"last_evidence_at,omitempty"`
	Uncertainty    string                `json:"uncertainty"`
	Ref            *StudyPlanEvidenceRef `json:"ref,omitempty"`
}

type StudyPlanAlternative struct {
	Kind             string `json:"kind"`
	ResourceID       string `json:"resource_id"`
	RevisionID       string `json:"revision_id"`
	EstimatedMinutes int    `json:"estimated_minutes"`
}

type StudyPlanStepExplanation struct {
	Ordinal     int                   `json:"ordinal"`
	Reasons     []StudyPlanReason     `json:"reasons"`
	Evidence    StudyPlanEvidence     `json:"evidence"`
	Alternative *StudyPlanAlternative `json:"alternative,omitempty"`
}

type StudyPlanLimit struct {
	TopicID string `json:"topic_id"`
	Stage   string `json:"stage,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StudyPlanExplanation struct {
	PlanVersion int64                      `json:"plan_version"`
	Stale       bool                       `json:"stale"`
	Steps       []StudyPlanStepExplanation `json:"steps"`
	Limits      []StudyPlanLimit           `json:"limits"`
	Freshness   *StudyPlanFreshness        `json:"freshness,omitempty"`
}

type StudyPlanService interface {
	CurrentStudyPlan(context.Context, string, string) (StudyPlan, error)
	GenerateStudyPlan(context.Context, string, StudyPlanGenerationInput, string, string) (StudyPlan, error)
	ExplainStudyPlan(context.Context, string, int64, string) (StudyPlanExplanation, error)
	SubmitStudyPlanFeedback(context.Context, string, StudyPlanFeedbackInput, string, string) (StudyPlanFeedbackReceipt, error)
	StartStudyPlanStep(context.Context, string, int64, int, string) (StudyPlan, error)
	CompleteStudyPlanStepActivity(context.Context, string, int64, int, string) (StudyPlan, error)
	RecordStudyPlanStepAssessment(context.Context, string, int64, int, string) (StudyPlan, error)
}

func (c *Client) CurrentStudyPlan(ctx context.Context, goalID, actor string) (StudyPlan, error) {
	return learnerCall[StudyPlan](c, ctx, http.MethodGet, studyPlanPath(goalID), nil, actor)
}

func (c *Client) GenerateStudyPlan(ctx context.Context, goalID string, input StudyPlanGenerationInput, requestID, actor string) (StudyPlan, error) {
	value, err := httpx.DoJSONWithHeaders[StudyPlan](c.hx, ctx, http.MethodPut, studyPlanPath(goalID), input, httpx.Actor{UserID: actor}, httpx.Header{Name: "Idempotency-Key", Value: requestID})
	if err != nil {
		return StudyPlan{}, goalError(err)
	}

	return value, nil
}

func (c *Client) ExplainStudyPlan(ctx context.Context, goalID string, planVersion int64, actor string) (StudyPlanExplanation, error) {
	query := url.Values{"plan_version": []string{strconv.FormatInt(planVersion, 10)}}

	return learnerCall[StudyPlanExplanation](c, ctx, http.MethodGet, studyPlanPath(goalID)+"/explanation?"+query.Encode(), nil, actor)
}

func (c *Client) SubmitStudyPlanFeedback(ctx context.Context, goalID string, input StudyPlanFeedbackInput, requestID, actor string) (StudyPlanFeedbackReceipt, error) {
	value, err := httpx.DoJSONWithHeaders[StudyPlanFeedbackReceipt](c.hx, ctx, http.MethodPost, studyPlanPath(goalID)+"/feedback", input, httpx.Actor{UserID: actor}, httpx.Header{Name: "Idempotency-Key", Value: requestID})
	if err != nil {
		return StudyPlanFeedbackReceipt{}, goalError(err)
	}

	return value, nil
}

func (c *Client) StartStudyPlanStep(ctx context.Context, goalID string, planVersion int64, ordinal int, actor string) (StudyPlan, error) {
	return c.studyPlanProgress(ctx, goalID, planVersion, ordinal, "start", actor)
}

func (c *Client) CompleteStudyPlanStepActivity(ctx context.Context, goalID string, planVersion int64, ordinal int, actor string) (StudyPlan, error) {
	return c.studyPlanProgress(ctx, goalID, planVersion, ordinal, "activity", actor)
}

func (c *Client) RecordStudyPlanStepAssessment(ctx context.Context, goalID string, planVersion int64, ordinal int, actor string) (StudyPlan, error) {
	return c.studyPlanProgress(ctx, goalID, planVersion, ordinal, "assessment", actor)
}

func (c *Client) studyPlanProgress(ctx context.Context, goalID string, planVersion int64, ordinal int, action, actor string) (StudyPlan, error) {
	path := studyPlanPath(goalID) + "/steps/" + strconv.Itoa(ordinal) + "/" + action

	return learnerCall[StudyPlan](c, ctx, http.MethodPut, path, struct {
		PlanVersion int64 `json:"plan_version"`
	}{PlanVersion: planVersion}, actor)
}

func studyPlanPath(goalID string) string { return learnerGoalPath(goalID) + "/study-plan" }

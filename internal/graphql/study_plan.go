package graphql

import (
	"context"
	"strings"
	"time"

	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func (r *queryResolver) CurrentLearningPlan(ctx context.Context, goalID string) (*model.LearnerStudyPlan, error) {
	auth, service, err := studyPlanService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.CurrentStudyPlan(ctx, goalID, auth.UserID)
	if err != nil {
		return nil, err
	}

	return studyPlanModel(value), nil
}

func (r *queryResolver) LearningPlanExplanation(ctx context.Context, goalID string, planVersion int) (*model.LearnerStudyPlanExplanation, error) {
	auth, service, err := studyPlanService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.ExplainStudyPlan(ctx, goalID, int64(planVersion), auth.UserID)
	if err != nil {
		return nil, err
	}

	return studyPlanExplanationModel(value), nil
}

func (r *mutationResolver) GenerateLearningPlan(ctx context.Context, goalID string, goalVersion int, availabilityVersion int, planVersion int, requestID string) (*model.LearnerStudyPlan, error) {
	auth, service, err := studyPlanService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.GenerateStudyPlan(ctx, goalID, recommendations.StudyPlanGenerationInput{GoalVersion: int64(goalVersion), AvailabilityVersion: int64(availabilityVersion), PlanVersion: int64(planVersion)}, requestID, auth.UserID)
	if err != nil {
		return nil, err
	}

	return studyPlanModel(value), nil
}

func (r *mutationResolver) SubmitLearningPlanFeedback(ctx context.Context, goalID string, input model.LearnerStudyPlanFeedbackInput, requestID string) (*model.LearnerStudyPlanFeedbackReceipt, error) {
	auth, service, err := studyPlanService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.SubmitStudyPlanFeedback(ctx, goalID, recommendations.StudyPlanFeedbackInput{PlanVersion: int64(input.PlanVersion), Action: strings.ToLower(input.Action.String()), Ordinal: input.Ordinal, WeeklyMinutes: input.WeeklyMinutes}, requestID, auth.UserID)
	if err != nil {
		return nil, err
	}

	return &model.LearnerStudyPlanFeedbackReceipt{RequestID: value.RequestID, PlanVersion: int(value.PlanVersion), Action: model.LearnerStudyPlanFeedbackAction(strings.ToUpper(value.Action)), Ordinal: value.Ordinal, WeeklyMinutes: value.WeeklyMinutes, AcceptedAt: value.AcceptedAt.Format(time.RFC3339Nano)}, nil
}

func (r *mutationResolver) StartLearningPlanStep(ctx context.Context, goalID string, planVersion int, ordinal int) (*model.LearnerStudyPlan, error) {
	return r.updateStudyPlanProgress(ctx, goalID, planVersion, ordinal, func(service recommendations.StudyPlanService, actor string) (recommendations.StudyPlan, error) {
		return service.StartStudyPlanStep(ctx, goalID, int64(planVersion), ordinal, actor)
	})
}

func (r *mutationResolver) CompleteLearningPlanStepActivity(ctx context.Context, goalID string, planVersion int, ordinal int) (*model.LearnerStudyPlan, error) {
	return r.updateStudyPlanProgress(ctx, goalID, planVersion, ordinal, func(service recommendations.StudyPlanService, actor string) (recommendations.StudyPlan, error) {
		return service.CompleteStudyPlanStepActivity(ctx, goalID, int64(planVersion), ordinal, actor)
	})
}

func (r *mutationResolver) RecordLearningPlanStepAssessment(ctx context.Context, goalID string, planVersion int, ordinal int) (*model.LearnerStudyPlan, error) {
	return r.updateStudyPlanProgress(ctx, goalID, planVersion, ordinal, func(service recommendations.StudyPlanService, actor string) (recommendations.StudyPlan, error) {
		return service.RecordStudyPlanStepAssessment(ctx, goalID, int64(planVersion), ordinal, actor)
	})
}

func (r *mutationResolver) updateStudyPlanProgress(ctx context.Context, _ string, _ int, _ int, write func(recommendations.StudyPlanService, string) (recommendations.StudyPlan, error)) (*model.LearnerStudyPlan, error) {
	auth, service, err := studyPlanService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := write(service, auth.UserID)
	if err != nil {
		return nil, err
	}

	return studyPlanModel(value), nil
}

func studyPlanService(ctx context.Context, resolver *Resolver) (middleware.AuthInfo, recommendations.StudyPlanService, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return middleware.AuthInfo{}, nil, err
	}
	service, ok := resolver.Recommendations.(recommendations.StudyPlanService)
	if !ok {
		return middleware.AuthInfo{}, nil, &recommendations.Error{Code: codeServiceUnavailable}
	}

	return auth, service, nil
}

func studyPlanModel(value recommendations.StudyPlan) *model.LearnerStudyPlan {
	weeks := make([]*model.LearnerStudyPlanWeek, 0, len(value.Weeks))
	for _, week := range value.Weeks {
		items := make([]*model.LearnerStudyPlanItem, 0, len(week.Items))
		for _, item := range week.Items {
			var assessment *model.LearnerStudyPlanAssessment
			if item.Assessment != nil {
				assessment = &model.LearnerStudyPlanAssessment{Verdict: item.Assessment.Verdict, GradedAt: item.Assessment.GradedAt.Format(time.RFC3339Nano)}
			}
			items = append(items, &model.LearnerStudyPlanItem{Kind: item.Kind, ResourceID: item.ResourceID, RevisionID: item.RevisionID,
				TopicID: item.TopicID, EstimatedMinutes: item.EstimatedMinutes, Score: item.Score, Components: studyPlanFactorsModel(item.Components),
				Stage: item.Stage, RequiresChecks: cloneStrings(item.RequiresChecks), Superseded: item.Superseded, Ordinal: item.Ordinal,
				ActivityStartedAt: timeString(item.ActivityStartedAt), ActivityCompletedAt: timeString(item.ActivityDoneAt), Assessment: assessment, Skipped: item.Skipped})
		}
		weeks = append(weeks, &model.LearnerStudyPlanWeek{From: week.From, Through: week.Through, CapacityMinutes: week.CapacityMinutes, PlannedMinutes: week.PlannedMinutes, Items: items})
	}
	result := &model.LearnerStudyPlan{GoalID: value.GoalID, Version: int(value.Version), GoalVersion: int(value.GoalVersion), AvailabilityVersion: int(value.AvailabilityVersion),
		TargetTopics: cloneStrings(value.TargetTopics), PlannerVersion: value.PlannerVersion, RankingAlgorithm: value.RankingAlgorithm,
		RankingPolicy: &model.LearnerStudyPlanRankingPolicy{Version: value.RankingPolicy.Version, TargetBps: value.RankingPolicy.TargetBPS,
			UnknownGapBps: value.RankingPolicy.UnknownGapBPS, PrerequisiteMinBps: value.RankingPolicy.PrerequisiteMinBPS,
			MediumDifficultyBps: value.RankingPolicy.MediumDifficultyBPS, HardDifficultyBps: value.RankingPolicy.HardDifficultyBPS,
			Weights: studyPlanFactorsModel(value.RankingPolicy.Weights)},
		EvidenceWatermark: value.EvidenceWatermark,
		EvidencePolicy: &model.LearnerStudyPlanEvidencePolicy{Revision: value.EvidencePolicy.Revision, Algorithm: value.EvidencePolicy.Algorithm,
			GapMaxBps: value.EvidencePolicy.GapMaxBPS, DemonstratedBps: value.EvidencePolicy.DemonstratedBPS, MinWeightedTasks: value.EvidencePolicy.MinWeightedTasks},
		CreatedAt: value.CreatedAt.Format(time.RFC3339Nano), Timezone: value.Timezone, WeeklyMinutes: value.WeeklyMinutes, HorizonLimited: value.HorizonLimited,
		Weeks: weeks, UnmetOutcomes: studyPlanOutcomesModel(value.UnmetOutcomes), CoverageGaps: studyPlanOutcomesModel(value.CoverageGaps),
		Suggestions: studyPlanSuggestionsModel(value.Suggestions), Current: value.Current, Stale: value.Stale, Freshness: studyPlanFreshnessModel(value.Freshness),
		PausedAt: timeString(value.PausedAt), NextStep: value.NextStep, BlockedReason: value.BlockedReason, GoalDemonstrated: value.GoalDemonstrated}
	if value.Guidance != nil {
		result.Guidance = &model.LearnerStudyPlanGuidance{Basis: value.Guidance.Basis, TemplateVersion: value.Guidance.TemplateVersion,
			VerifiedTopics: value.Guidance.VerifiedTopics, RecommendDiagnostic: value.Guidance.RecommendDiagnostic}
	}

	return result
}

func studyPlanExplanationModel(value recommendations.StudyPlanExplanation) *model.LearnerStudyPlanExplanation {
	steps := make([]*model.LearnerStudyPlanStepExplanation, 0, len(value.Steps))
	for _, step := range value.Steps {
		reasons := make([]*model.LearnerStudyPlanReason, 0, len(step.Reasons))
		for _, reason := range step.Reasons {
			reasons = append(reasons, &model.LearnerStudyPlanReason{Code: reason.Code, TopicID: reason.TopicID, Minutes: reason.Minutes, Message: reason.Message})
		}
		evidence := &model.LearnerStudyPlanEvidence{TopicID: step.Evidence.TopicID, ScoreBps: step.Evidence.ScoreBPS, SampleCount: step.Evidence.SampleCount,
			LastEvidenceAt: timeString(step.Evidence.LastEvidenceAt), Uncertainty: step.Evidence.Uncertainty}
		if step.Evidence.Ref != nil {
			evidence.Ref = &model.LearnerStudyPlanEvidenceRef{Watermark: step.Evidence.Ref.Watermark, PolicyRevision: step.Evidence.Ref.PolicyRevision, CapturedAt: step.Evidence.Ref.CapturedAt.Format(time.RFC3339Nano)}
		}
		mapped := &model.LearnerStudyPlanStepExplanation{Ordinal: step.Ordinal, Reasons: reasons, Evidence: evidence}
		if step.Alternative != nil {
			mapped.Alternative = &model.LearnerStudyPlanAlternative{Kind: step.Alternative.Kind, ResourceID: step.Alternative.ResourceID, RevisionID: step.Alternative.RevisionID, EstimatedMinutes: step.Alternative.EstimatedMinutes}
		}
		steps = append(steps, mapped)
	}
	limits := make([]*model.LearnerStudyPlanLimit, 0, len(value.Limits))
	for _, limit := range value.Limits {
		limits = append(limits, &model.LearnerStudyPlanLimit{TopicID: limit.TopicID, Stage: limit.Stage, Code: limit.Code, Message: limit.Message})
	}

	return &model.LearnerStudyPlanExplanation{PlanVersion: int(value.PlanVersion), Stale: value.Stale, Steps: steps, Limits: limits, Freshness: studyPlanFreshnessModel(value.Freshness)}
}

func studyPlanFactorsModel(value recommendations.StudyPlanFactors) *model.LearnerStudyPlanFactors {
	return &model.LearnerStudyPlanFactors{Gap: value.Gap, Difficulty: value.Difficulty, Duration: value.Duration, Variety: value.Variety, DueReview: value.DueReview}
}

func studyPlanOutcomesModel(values []recommendations.StudyPlanUnmetOutcome) []*model.LearnerStudyPlanUnmetOutcome {
	result := make([]*model.LearnerStudyPlanUnmetOutcome, 0, len(values))
	for _, value := range values {
		result = append(result, &model.LearnerStudyPlanUnmetOutcome{TopicID: value.TopicID, Stage: value.Stage, Reason: value.Reason, BlockedBy: cloneStrings(value.BlockedBy)})
	}

	return result
}

func studyPlanSuggestionsModel(values []recommendations.StudyPlanSuggestion) []*model.LearnerStudyPlanSuggestion {
	result := make([]*model.LearnerStudyPlanSuggestion, 0, len(values))
	for _, value := range values {
		result = append(result, &model.LearnerStudyPlanSuggestion{Kind: value.Kind, TopicIds: cloneStrings(value.TopicIDs), Deadline: value.Deadline})
	}

	return result
}

func studyPlanFreshnessModel(value *recommendations.StudyPlanFreshness) *model.LearnerStudyPlanFreshness {
	if value == nil {
		return nil
	}

	return &model.LearnerStudyPlanFreshness{Status: value.Status, CheckedAt: timeString(value.CheckedAt), EntitiesSyncedAt: timeString(value.EntitiesAt), TasksSyncedAt: timeString(value.TasksAt), ContentSyncedAt: timeString(value.ContentAt)}
}

func cloneStrings(values []string) []string {
	return append([]string{}, values...)
}

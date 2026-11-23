package graphql

import (
	"context"
	"time"

	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func (r *queryResolver) LearnerGoalTemplates(ctx context.Context) ([]*model.LearnerGoalTemplate, error) {
	_, service, err := learnerService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	values, err := service.GoalTemplates(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*model.LearnerGoalTemplate, 0, len(values))
	for _, value := range values {
		result = append(result, &model.LearnerGoalTemplate{Version: value.Version, Title: value.Title, SuggestsDeadline: value.SuggestsDeadline})
	}

	return result, nil
}

func (r *queryResolver) LearnerProfile(ctx context.Context) (*model.LearnerProfile, error) {
	_, service, err := learnerService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.GetLearnerProfile(ctx)
	if err != nil {
		return nil, err
	}

	return learnerProfileModel(value), nil
}

func (r *queryResolver) LearnerGoals(ctx context.Context) ([]*model.LearnerGoal, error) {
	_, service, err := learnerService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	values, err := service.ListLearnerGoals(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*model.LearnerGoal, 0, len(values))
	for _, value := range values {
		result = append(result, learnerGoalModel(value))
	}

	return result, nil
}

func (r *queryResolver) LearnerGoal(ctx context.Context, id string) (*model.LearnerGoal, error) {
	_, service, err := learnerService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.GetLearnerGoal(ctx, id)
	if err != nil {
		return nil, err
	}

	return learnerGoalModel(value), nil
}

func (r *queryResolver) LearnerDiagnostic(ctx context.Context, goalID string) (*model.LearnerDiagnosticSession, error) {
	_, service, err := diagnosticService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.GetDiagnostic(ctx, goalID)
	if err != nil {
		return nil, err
	}

	return diagnosticModel(value), nil
}

func (r *mutationResolver) SaveLearnerProfile(ctx context.Context, input model.LearnerAvailabilityInput, expectedVersion int, requestID string) (*model.LearnerProfile, error) {
	_, service, err := learnerService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.SaveLearnerProfile(ctx, recommendations.AvailabilityInput{WeeklyMinutes: input.WeeklyMinutes, Timezone: input.Timezone}, requestID, int64(expectedVersion))
	if err != nil {
		return nil, err
	}

	return learnerProfileModel(value), nil
}

func (r *mutationResolver) CreateLearnerGoal(ctx context.Context, input model.LearnerGoalInput, requestID string) (*model.LearnerGoal, error) {
	_, service, err := learnerService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.CreateLearnerGoal(ctx, learnerGoalInput(input), requestID)
	if err != nil {
		return nil, err
	}

	return learnerGoalModel(value), nil
}

func (r *mutationResolver) UpdateLearnerGoal(ctx context.Context, id string, input model.LearnerGoalInput, expectedVersion int, requestID string) (*model.LearnerGoal, error) {
	_, service, err := learnerService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.UpdateLearnerGoal(ctx, id, learnerGoalInput(input), requestID, int64(expectedVersion))
	if err != nil {
		return nil, err
	}

	return learnerGoalModel(value), nil
}

func (r *mutationResolver) DeleteLearnerGoal(ctx context.Context, id string, expectedVersion int, requestID string) (bool, error) {
	_, service, err := learnerService(ctx, r.Resolver)
	if err != nil {
		return false, err
	}
	if err := service.DeleteLearnerGoal(ctx, id, requestID, int64(expectedVersion)); err != nil {
		return false, err
	}

	return true, nil
}

func (r *mutationResolver) StartLearnerDiagnostic(ctx context.Context, goalID string, goalVersion int, budgetMinutes int) (*model.LearnerDiagnosticSession, error) {
	_, service, err := diagnosticService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.StartDiagnostic(ctx, goalID, recommendations.DiagnosticStartInput{GoalVersion: int64(goalVersion), BudgetMinutes: budgetMinutes})
	if err != nil {
		return nil, err
	}

	return diagnosticModel(value), nil
}

func (r *mutationResolver) SkipLearnerDiagnostic(ctx context.Context, goalID string, goalVersion int) (*model.LearnerDiagnosticSession, error) {
	_, service, err := diagnosticService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.SkipDiagnostic(ctx, goalID, recommendations.DiagnosticSkipInput{GoalVersion: int64(goalVersion)})
	if err != nil {
		return nil, err
	}

	return diagnosticModel(value), nil
}

func (r *mutationResolver) FinalizeLearnerDiagnostic(ctx context.Context, goalID string, goalVersion int, sessionID string, allowPartial bool) (*model.LearnerDiagnosticSession, error) {
	_, service, err := diagnosticService(ctx, r.Resolver)
	if err != nil {
		return nil, err
	}
	value, err := service.FinalizeDiagnostic(ctx, goalID, recommendations.DiagnosticFinalizeInput{GoalVersion: int64(goalVersion), SessionID: sessionID, AllowPartial: allowPartial})
	if err != nil {
		return nil, err
	}

	return diagnosticModel(value), nil
}

func learnerService(ctx context.Context, resolver *Resolver) (middleware.AuthInfo, recommendations.LearnerService, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return middleware.AuthInfo{}, nil, err
	}
	service, ok := resolver.Recommendations.(recommendations.LearnerService)
	if !ok {
		return middleware.AuthInfo{}, nil, &recommendations.Error{Code: codeServiceUnavailable}
	}

	return auth, service, nil
}

func diagnosticService(ctx context.Context, resolver *Resolver) (middleware.AuthInfo, recommendations.DiagnosticService, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return middleware.AuthInfo{}, nil, err
	}
	service, ok := resolver.Recommendations.(recommendations.DiagnosticService)
	if !ok {
		return middleware.AuthInfo{}, nil, &recommendations.Error{Code: codeServiceUnavailable}
	}

	return auth, service, nil
}

func learnerProfileModel(value recommendations.LearnerProfile) *model.LearnerProfile {
	result := &model.LearnerProfile{}
	if value.Availability != nil {
		result.Availability = &model.LearnerAvailability{WeeklyMinutes: value.Availability.WeeklyMinutes, Timezone: value.Availability.Timezone,
			Version: int(value.Availability.Version), UpdatedAt: value.Availability.UpdatedAt.Format(time.RFC3339Nano)}
	}

	return result
}

func learnerGoalInput(input model.LearnerGoalInput) recommendations.LearnerGoalInput {
	assessments := make([]recommendations.SelfAssessment, 0, len(input.SelfAssessments))
	for _, item := range input.SelfAssessments {
		assessments = append(assessments, recommendations.SelfAssessment{TopicID: item.TopicID, Level: item.Level})
	}

	return recommendations.LearnerGoalInput{Title: input.Title, TemplateVersion: stringValue(input.TemplateVersion), TargetTopics: input.TargetTopics,
		Deadline: stringValue(input.Deadline), WeeklyMinutes: input.WeeklyMinutes, Active: input.Active, Languages: input.Languages,
		Formats: input.Formats, SelfAssessments: assessments}
}

func learnerGoalModel(value recommendations.LearnerGoal) *model.LearnerGoal {
	assessments := make([]*model.LearnerSelfAssessment, 0, len(value.SelfAssessments))
	for _, item := range value.SelfAssessments {
		assessments = append(assessments, &model.LearnerSelfAssessment{TopicID: item.TopicID, Level: item.Level})
	}
	skills := make([]*model.LearnerTopicSkill, 0, len(value.Skills))
	for _, item := range value.Skills {
		skills = append(skills, &model.LearnerTopicSkill{TopicID: item.TopicID, State: item.State})
	}

	return &model.LearnerGoal{ID: value.ID, Title: value.Title, TemplateVersion: value.TemplateVersion, TargetTopics: value.TargetTopics,
		Deadline: value.Deadline, WeeklyMinutes: value.WeeklyMinutes, Active: value.Active, Languages: value.Languages, Formats: value.Formats,
		SelfAssessments: assessments, Timezone: value.Timezone, Version: int(value.Version), CreatedAt: value.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: value.UpdatedAt.Format(time.RFC3339Nano), Skills: skills, Feasibility: value.Feasibility, FeasibilityReason: value.FeasibilityReason}
}

func diagnosticModel(value recommendations.DiagnosticSession) *model.LearnerDiagnosticSession {
	tasks := make([]*model.LearnerDiagnosticTask, 0, len(value.Tasks))
	for _, item := range value.Tasks {
		tasks = append(tasks, &model.LearnerDiagnosticTask{TaskID: item.TaskID, TaskVersionID: item.TaskVersionID, TopicID: item.TopicID, EstimatedMinutes: item.EstimatedMinutes})
	}
	result := &model.LearnerDiagnosticSession{ID: value.ID, GoalID: value.GoalID, GoalVersion: int(value.GoalVersion), Status: value.Status,
		Baseline: value.Baseline, Tasks: tasks, EstimatedMinutes: value.EstimatedMinutes,
		StopRule:  &model.LearnerDiagnosticStopRule{PolicyVersion: value.StopRule.PolicyVersion, MaxTasks: value.StopRule.MaxTasks, BudgetMinutes: value.StopRule.BudgetMinutes},
		StartedAt: value.StartedAt.Format(time.RFC3339Nano), ExpiresAt: value.ExpiresAt.Format(time.RFC3339Nano), SkippedAt: timeString(value.SkippedAt), FinalizedAt: timeString(value.FinalizedAt)}
	if value.Report != nil {
		topics := make([]*model.LearnerDiagnosticTopicCoverage, 0, len(value.Report.Topics))
		for _, item := range value.Report.Topics {
			topics = append(topics, &model.LearnerDiagnosticTopicCoverage{TopicID: item.TopicID, Assigned: item.Assigned, Assessed: item.Assessed, Observation: item.Observation, Uncertainty: item.Uncertainty})
		}
		outcomes := make([]*model.LearnerDiagnosticOutcome, 0, len(value.Report.Outcomes))
		for _, item := range value.Report.Outcomes {
			outcomes = append(outcomes, &model.LearnerDiagnosticOutcome{TaskID: item.TaskID, TaskVersionID: item.TaskVersionID, TopicID: item.TopicID,
				SubmissionID: item.SubmissionID, EventID: item.EventID, CorrectionVersion: int(item.CorrectionVersion), Verdict: item.Verdict, GradedAt: item.GradedAt.Format(time.RFC3339Nano)})
		}
		result.Report = &model.LearnerDiagnosticReport{Assigned: value.Report.Assigned, Assessed: value.Report.Assessed, Topics: topics, Outcomes: outcomes}
	}

	return result
}

func timeString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339Nano)

	return &formatted
}

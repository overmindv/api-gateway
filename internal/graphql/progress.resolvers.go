package graphql

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

const (
	progressDeadline    = 10 * time.Second
	ratingPageSize      = 100
	ratingMaxCandidates = 1000
	ratingMaxEntries    = 50
	ratingMaxTopics     = 1000
)

func (r *queryResolver) MyPracticeProgress(ctx context.Context) (*model.PracticeProgress, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	service, ok := r.Tasks.(tasks.ProgressService)
	if !ok {
		return nil, progressUnavailable()
	}
	ctx, cancel := context.WithTimeout(ctx, progressDeadline)
	defer cancel()
	result, err := service.MyProgress(ctx, tasks.Actor{UserID: auth.UserID, Roles: auth.Roles})
	if err != nil {
		return nil, err
	}
	return progressModel(result), nil
}

func (r *queryResolver) PracticeProgress(ctx context.Context, userID string) (*model.PracticeProgress, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(userID)
	if err != nil || id == uuid.Nil {
		return nil, progressInvalid()
	}
	service, ok := r.Tasks.(tasks.ProgressService)
	if !ok {
		return nil, progressUnavailable()
	}
	ctx, cancel := context.WithTimeout(ctx, progressDeadline)
	defer cancel()
	actor := tasks.Actor{UserID: auth.UserID, Roles: auth.Roles}
	if id.String() == auth.UserID {
		mine, err := service.MyProgress(ctx, actor)
		if err != nil {
			return nil, err
		}
		return progressModel(mine), nil
	}
	if r.Resolver.Users == nil {
		return nil, progressUnavailable()
	}
	profile, err := r.Resolver.Users.GetPublicUser(ctx, id.String())
	if err != nil {
		var upstream *users.Error
		if errors.As(err, &upstream) && upstream.Code == "NOT_FOUND" {
			return nil, nil
		}
		return nil, progressUnavailable()
	}
	if profile == nil {
		return nil, progressUnavailable()
	}
	result, err := service.UserProgress(ctx, id.String(), actor)
	if err != nil || result == nil {
		return nil, err
	}
	return progressModel(*result), nil
}

func (r *mutationResolver) SetMyPracticeStatsPublic(ctx context.Context, public bool) (bool, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return false, err
	}
	service, ok := r.Tasks.(tasks.ProgressService)
	if !ok {
		return false, progressUnavailable()
	}
	ctx, cancel := context.WithTimeout(ctx, progressDeadline)
	defer cancel()
	return service.SetMyProgressPublic(ctx, public, tasks.Actor{UserID: auth.UserID, Roles: auth.Roles})
}

func (r *queryResolver) PracticeLeaderboard(ctx context.Context, courseID *string) ([]*model.PracticeLeaderboardEntry, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	service, ok := r.Tasks.(tasks.ProgressService)
	if !ok || r.Resolver.Users == nil {
		return nil, progressUnavailable()
	}
	ctx, cancel := context.WithTimeout(ctx, progressDeadline)
	defer cancel()
	var topicIDs *[]string
	if courseID != nil {
		if r.Catalog == nil {
			return nil, progressUnavailable()
		}
		id, parseErr := uuid.Parse(*courseID)
		if parseErr != nil || id == uuid.Nil {
			return nil, progressInvalid()
		}
		ids, err := r.progressCourseTopics(ctx, id.String())
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return []*model.PracticeLeaderboardEntry{}, nil
		}
		topicIDs = &ids
	}
	actor := tasks.Actor{UserID: auth.UserID, Roles: auth.Roles}
	result := make([]*model.PracticeLeaderboardEntry, 0, ratingMaxEntries)
	previousScore, rank := -1, 0
	for offset := 0; offset <= ratingMaxCandidates; offset += ratingPageSize {
		pageLimit := ratingPageSize
		if offset == ratingMaxCandidates {
			pageLimit = 1
		}
		candidates, err := service.RatedUsers(ctx, topicIDs, pageLimit, offset, actor)
		if err != nil {
			return nil, err
		}
		if offset == ratingMaxCandidates {
			if len(candidates) == 0 {
				return result, nil
			}
			return nil, progressUnavailable()
		}
		for _, candidate := range candidates {
			profile, err := r.Resolver.Users.GetPublicUser(ctx, candidate.UserID)
			if err != nil {
				var upstream *users.Error
				if errors.As(err, &upstream) && upstream.Code == "NOT_FOUND" {
					continue
				}
				return nil, progressUnavailable()
			}
			if profile == nil {
				return nil, progressUnavailable()
			}
			if candidate.Score != previousScore {
				rank = len(result) + 1
				previousScore = candidate.Score
			}
			name := profile.FirstName + " " + profile.LastName
			if profile.FirstName == "" && profile.LastName == "" {
				name = profile.Username
			}
			result = append(result, &model.PracticeLeaderboardEntry{
				Rank: rank, UserID: candidate.UserID, Username: profile.Username,
				DisplayName: name, Score: candidate.Score, SolvedTasks: candidate.SolvedTasks,
			})
			if len(result) == ratingMaxEntries {
				return result, nil
			}
		}
		if len(candidates) < ratingPageSize {
			return result, nil
		}
	}
	return nil, progressUnavailable()
}

func (r *Resolver) progressCourseTopics(ctx context.Context, courseID string) ([]string, error) {
	course, err := r.Catalog.GetCourse(ctx, courseID)
	if err != nil {
		return nil, err
	}
	if course.ID != courseID {
		return nil, progressUnavailable()
	}
	ids := make([]string, 0)
	for offset := 0; offset <= ratingMaxTopics; offset += ratingPageSize {
		page, err := r.Catalog.ListTopics(ctx, courseID, entities.ListOptions{Limit: ratingPageSize, Offset: offset})
		if err != nil {
			return nil, err
		}
		if offset == ratingMaxTopics {
			if len(page) == 0 {
				return ids, nil
			}
			return nil, progressUnavailable()
		}
		for _, topic := range page {
			if topic.CourseID == nil || *topic.CourseID != courseID {
				return nil, progressUnavailable()
			}
			id, err := uuid.Parse(topic.ID)
			if err != nil || id == uuid.Nil {
				return nil, progressUnavailable()
			}
			ids = append(ids, id.String())
		}
		if len(page) < ratingPageSize {
			return ids, nil
		}
	}
	return nil, progressUnavailable()
}

func progressModel(input tasks.Progress) *model.PracticeProgress {
	days := make([]*model.PracticeProgressDay, 0, len(input.Daily))
	for _, day := range input.Daily {
		days = append(days, &model.PracticeProgressDay{Date: day.Date, Attempts: day.Attempts, Accepted: day.Accepted})
	}
	difficulties := make([]*model.PracticeProgressDifficulty, 0, len(input.ByDifficulty))
	for _, item := range input.ByDifficulty {
		difficulties = append(difficulties, &model.PracticeProgressDifficulty{Difficulty: item.Difficulty, Solved: item.Solved})
	}
	return &model.PracticeProgress{
		UserID: input.UserID, Public: input.Public, Attempts: input.Attempts,
		AcceptedAttempts: input.AcceptedAttempts, SolvedTasks: input.SolvedTasks,
		Score: input.Score, AccuracyPercent: input.AccuracyPercent, Daily: days,
		ByDifficulty: difficulties,
	}
}

func progressInvalid() error     { return &tasks.Error{Code: "INVALID_ARGUMENT"} }
func progressUnavailable() error { return &tasks.Error{Code: "SERVICE_UNAVAILABLE"} }

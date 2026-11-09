package tasks

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

type ProgressDay struct {
	Date     string `json:"date"`
	Attempts int    `json:"attempts"`
	Accepted int    `json:"accepted"`
}

type ProgressDifficulty struct {
	Difficulty string `json:"difficulty"`
	Solved     int    `json:"solved"`
}

type Progress struct {
	UserID           string               `json:"user_id"`
	Public           bool                 `json:"public"`
	Attempts         int                  `json:"attempts"`
	AcceptedAttempts int                  `json:"accepted_attempts"`
	SolvedTasks      int                  `json:"solved_tasks"`
	Score            int                  `json:"score"`
	AccuracyPercent  int                  `json:"accuracy_percent"`
	Daily            []ProgressDay        `json:"daily"`
	ByDifficulty     []ProgressDifficulty `json:"by_difficulty"`
}

type RatedUser struct {
	UserID      string `json:"user_id"`
	SolvedTasks int    `json:"solved_tasks"`
	Score       int    `json:"score"`
}

// ProgressService отдельно от основного Service, чтобы старые клиенты и моки
// не становились владельцами новой возможности автоматически.
type ProgressService interface {
	MyProgress(context.Context, Actor) (Progress, error)
	UserProgress(context.Context, string, Actor) (*Progress, error)
	SetMyProgressPublic(context.Context, bool, Actor) (bool, error)
	RatedUsers(context.Context, *[]string, int, int, Actor) ([]RatedUser, error)
}

func (c *Client) MyProgress(ctx context.Context, actor Actor) (Progress, error) {
	return request[Progress](c, ctx, http.MethodGet, "/v1/me/practice-progress", nil, actor)
}

func (c *Client) UserProgress(ctx context.Context, userID string, actor Actor) (*Progress, error) {
	result, err := request[Progress](c, ctx, http.MethodGet, "/v1/users/"+url.PathEscape(userID)+"/practice-progress", nil, actor)
	if err != nil {
		var upstream *Error
		if errors.As(err, &upstream) && upstream.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

func (c *Client) SetMyProgressPublic(ctx context.Context, public bool, actor Actor) (bool, error) {
	result, err := request[struct {
		Public bool `json:"public"`
	}](c, ctx, http.MethodPut, "/v1/me/practice-progress/visibility", struct {
		Public bool `json:"public"`
	}{Public: public}, actor)
	return result.Public, err
}

func (c *Client) RatedUsers(ctx context.Context, topicIDs *[]string, limit, offset int, actor Actor) ([]RatedUser, error) {
	values := url.Values{}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	if topicIDs == nil {
		values.Set("scope", "global")
	} else {
		values.Set("scope", "course")
		for _, id := range *topicIDs {
			values.Add("topic_id", id)
		}
	}
	return request[[]RatedUser](c, ctx, http.MethodGet, pathWithQuery("/v1/practice-leaderboard", values), nil, actor)
}

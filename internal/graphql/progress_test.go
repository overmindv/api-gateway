package graphql

import (
	"context"
	"errors"
	"testing"

	"github.com/overmindv/api-gateway/internal/apperror"
	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
)

const progressCourse = "00000000-0000-4000-8000-000000000021"
const progressTopic = "00000000-0000-4000-8000-000000000022"
const progressPeerA = "00000000-0000-4000-8000-000000000023"
const progressPeerB = "00000000-0000-4000-8000-000000000024"
const progressDeleted = "00000000-0000-4000-8000-000000000025"
const progressPeerC = "00000000-0000-4000-8000-000000000026"

type progressTasksStub struct {
	tasks.Service
	candidates []tasks.RatedUser
	topics     *[]string
	actor      tasks.Actor
	calls      int
	public     bool
}

func (s *progressTasksStub) MyProgress(_ context.Context, actor tasks.Actor) (tasks.Progress, error) {
	s.actor = actor
	return tasks.Progress{UserID: actor.UserID, Public: s.public, Score: 30, Daily: []tasks.ProgressDay{}, ByDifficulty: []tasks.ProgressDifficulty{}}, nil
}
func (s *progressTasksStub) UserProgress(_ context.Context, _ string, actor tasks.Actor) (*tasks.Progress, error) {
	s.actor = actor
	return nil, nil
}
func (s *progressTasksStub) SetMyProgressPublic(_ context.Context, public bool, actor tasks.Actor) (bool, error) {
	s.actor = actor
	s.public = public
	return public, nil
}
func (s *progressTasksStub) RatedUsers(_ context.Context, topics *[]string, limit, offset int, actor tasks.Actor) ([]tasks.RatedUser, error) {
	s.actor = actor
	s.topics = topics
	s.calls++
	if offset >= len(s.candidates) {
		return []tasks.RatedUser{}, nil
	}
	end := offset + limit
	if end > len(s.candidates) {
		end = len(s.candidates)
	}
	return s.candidates[offset:end], nil
}

type progressUsersStub struct{ users.UserService }

func (s *progressUsersStub) GetPublicUser(_ context.Context, id string) (*users.PublicUser, error) {
	if id == progressDeleted {
		return nil, &users.Error{Code: "NOT_FOUND"}
	}
	return &users.PublicUser{ID: id, Username: "learner", FirstName: "Иван", LastName: "Петров"}, nil
}

type progressCatalogStub struct {
	entities.CatalogService
	topics []entities.Topic
}

func (s *progressCatalogStub) GetCourse(_ context.Context, id string) (entities.Course, error) {
	return entities.Course{ID: id}, nil
}
func (s *progressCatalogStub) ListTopics(_ context.Context, _ string, _ entities.ListOptions) ([]entities.Topic, error) {
	return s.topics, nil
}

func TestProgressResolversRequireAuthenticatedActor(t *testing.T) {
	service := &progressTasksStub{}
	r := &Resolver{Tasks: service, Users: &progressUsersStub{}}
	if _, err := (&queryResolver{r}).MyPracticeProgress(context.Background()); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("my progress auth: %v", err)
	}
	if _, err := (&queryResolver{r}).PracticeLeaderboard(context.Background(), nil); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("leaderboard auth: %v", err)
	}
	if _, err := (&mutationResolver{r}).SetMyPracticeStatsPublic(context.Background(), true); !errors.Is(err, apperror.ErrUnauthenticated) {
		t.Fatalf("visibility auth: %v", err)
	}
	if service.calls != 0 {
		t.Fatal("upstream called without JWT")
	}
}

func TestProgressOwnPrivatePeerAndVisibility(t *testing.T) {
	service := &progressTasksStub{}
	r := &Resolver{Tasks: service, Users: &progressUsersStub{}}
	ctx := practiceContext()
	mine, err := (&queryResolver{r}).MyPracticeProgress(ctx)
	if err != nil || mine.Score != 30 || mine.Public || service.actor.UserID != practiceUser {
		t.Fatalf("my progress=%+v actor=%+v err=%v", mine, service.actor, err)
	}
	peer, err := (&queryResolver{r}).PracticeProgress(ctx, progressPeerA)
	if err != nil || peer != nil {
		t.Fatalf("private peer=%+v err=%v", peer, err)
	}
	public, err := (&mutationResolver{r}).SetMyPracticeStatsPublic(ctx, true)
	if err != nil || !public || service.actor.UserID != practiceUser {
		t.Fatalf("visibility=%v actor=%+v err=%v", public, service.actor, err)
	}
}

func TestPracticeLeaderboardSkipsDeletedAndRanksVisibleTies(t *testing.T) {
	service := &progressTasksStub{candidates: []tasks.RatedUser{
		{UserID: progressDeleted, Score: 50, SolvedTasks: 2},
		{UserID: progressPeerA, Score: 40, SolvedTasks: 2},
		{UserID: progressPeerB, Score: 40, SolvedTasks: 2},
		{UserID: progressPeerC, Score: 10, SolvedTasks: 1},
	}}
	r := &Resolver{Tasks: service, Users: &progressUsersStub{}}
	rows, err := (&queryResolver{r}).PracticeLeaderboard(practiceContext(), nil)
	if err != nil || len(rows) != 3 {
		t.Fatalf("rating=%+v err=%v", rows, err)
	}
	if rows[0].Rank != 1 || rows[1].Rank != 1 || rows[2].Rank != 3 || rows[0].UserID != progressPeerA {
		t.Fatalf("deleted account or ties shifted rank: %+v", rows)
	}
	if service.actor.UserID != practiceUser || service.topics != nil {
		t.Fatalf("actor/scope=%+v %v", service.actor, service.topics)
	}
}

func TestPracticeLeaderboardCourseIncludesArchivedAndEmpty(t *testing.T) {
	course := progressCourse
	topic := progressTopic
	service := &progressTasksStub{candidates: []tasks.RatedUser{{UserID: progressPeerA, Score: 20, SolvedTasks: 1}}}
	catalog := &progressCatalogStub{topics: []entities.Topic{{ID: topic, CourseID: &course, Status: "archived"}}}
	r := &Resolver{Tasks: service, Users: &progressUsersStub{}, Catalog: catalog}
	rows, err := (&queryResolver{r}).PracticeLeaderboard(practiceContext(), &course)
	if err != nil || len(rows) != 1 || service.topics == nil || len(*service.topics) != 1 || (*service.topics)[0] != topic {
		t.Fatalf("archived course topic lost: rows=%+v topics=%v err=%v", rows, service.topics, err)
	}
	catalog.topics = nil
	service.calls = 0
	rows, err = (&queryResolver{r}).PracticeLeaderboard(practiceContext(), &course)
	if err != nil || len(rows) != 0 || service.calls != 0 {
		t.Fatalf("empty course should not become global: rows=%+v calls=%d err=%v", rows, service.calls, err)
	}
}

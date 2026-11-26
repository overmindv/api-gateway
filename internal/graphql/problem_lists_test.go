package graphql

import (
	"context"
	"testing"

	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// problemListCandidates фиксирует actor и отдаёт подготовленную подборку.
type problemListCandidates struct {
	detail tasks.ProblemListDetail
	list   tasks.ProblemListList
	actors []tasks.Actor
}

func (f *problemListCandidates) ListCandidates(context.Context, tasks.CandidateFilter, tasks.Actor) (tasks.CandidateList, error) {
	return tasks.CandidateList{}, nil
}

func (f *problemListCandidates) GetCandidate(context.Context, string, tasks.Actor) (tasks.Candidate, error) {
	return tasks.Candidate{}, nil
}

func (f *problemListCandidates) UpdateCandidate(context.Context, string, tasks.CandidateReview, tasks.Actor) (tasks.Candidate, error) {
	return tasks.Candidate{}, nil
}

func (f *problemListCandidates) ApproveCandidate(context.Context, string, tasks.CandidateReview, tasks.Actor) (tasks.Task, error) {
	return tasks.Task{}, nil
}

func (f *problemListCandidates) RejectCandidate(context.Context, string, int, string, tasks.Actor) (tasks.Candidate, error) {
	return tasks.Candidate{}, nil
}

func (f *problemListCandidates) ListProblemLists(_ context.Context, _ int, _ int, actor tasks.Actor) (tasks.ProblemListList, error) {
	f.actors = append(f.actors, actor)

	return f.list, nil
}

// GetProblemList имитирует публичный маршрут tasks: не-администратор получает 0.
func (f *problemListCandidates) GetProblemList(_ context.Context, _ string, actor tasks.Actor) (tasks.ProblemListDetail, error) {
	f.actors = append(f.actors, actor)
	detail := f.detail
	if !actor.IsAdmin() {
		detail.PendingCandidates = 0
	}

	return detail, nil
}

// TestProblemListIsPublicForAuthenticatedUsers проверяет, что студент получает
// подборку с нулевой очередью модерации, а администратор — полную, и роль
// передаётся в tasks.
func TestProblemListIsPublicForAuthenticatedUsers(t *testing.T) {
	t.Parallel()

	fake := &problemListCandidates{detail: tasks.ProblemListDetail{
		ProblemListSummary: tasks.ProblemListSummary{
			ID: "list-1", Name: "Top 100", SourceID: "leetcode",
			SourceURL: "https://leetcode.com/problem-list/abc", Truncated: true, MemberCount: 1,
		},
		PendingCandidates: 5,
		Members: []tasks.ProblemListMember{{
			TaskID: "task-1", Title: "Two Sum", Difficulty: "easy", Status: "published",
		}},
	}}
	r := &queryResolver{&Resolver{Candidates: fake}}
	studentCtx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{
		UserID: "student-1", Token: "jwt", Roles: []string{"student"},
	})

	public, err := r.ProblemList(studentCtx, "list-1")
	if err != nil {
		t.Fatal(err)
	}
	if !public.Truncated || public.MemberCount != 1 || len(public.Members) != 1 {
		t.Fatalf("публичная подборка: %+v", public)
	}
	if public.PendingCandidates != 0 {
		t.Fatalf("студент увидел очередь модерации: %d", public.PendingCandidates)
	}

	adminCtx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{
		UserID: "admin-1", Token: "jwt", Roles: []string{"admin"},
	})
	moderated, err := r.ProblemList(adminCtx, "list-1")
	if err != nil {
		t.Fatal(err)
	}
	if moderated.PendingCandidates != 5 {
		t.Fatalf("администратор не увидел очередь модерации: %d", moderated.PendingCandidates)
	}

	if len(fake.actors) != 2 || fake.actors[0].IsAdmin() || !fake.actors[1].IsAdmin() {
		t.Fatalf("роль не передана в tasks: %+v", fake.actors)
	}
}

// TestProblemListsRequiresAuthentication закрывает публичный список от анонимов.
func TestProblemListsRequiresAuthentication(t *testing.T) {
	t.Parallel()

	r := &queryResolver{&Resolver{Candidates: &problemListCandidates{}}}
	if _, err := r.ProblemLists(context.Background(), nil); err == nil {
		t.Fatal("анонимный запрос подборок не отклонён")
	}
}

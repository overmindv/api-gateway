package graphql

import (
	"context"
	"testing"

	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/graphql/model"
)

type savedFake struct {
	recommendations.Service
	plan   recommendations.SavedPlan
	actor  string
	writes int
	op     string
}

func (s *savedFake) GetSaved(_ context.Context, _ string, actor string) (recommendations.SavedPlan, error) {
	s.actor = actor
	return s.plan, nil
}
func (s *savedFake) ListSaved(_ context.Context, actor string) ([]recommendations.SavedPlan, error) {
	s.actor = actor
	return []recommendations.SavedPlan{s.plan}, nil
}
func (s *savedFake) WriteSaved(_ context.Context, _, op, actor string, in recommendations.SavedInput) (recommendations.SavedPlan, error) {
	s.actor = actor
	s.writes++
	s.op = op
	s.plan.Revision++
	return s.plan, nil
}
func savedFixture(t *testing.T) (*queryResolver, *savedFake, *practiceFixture) {
	f := &practiceFixture{}
	r := f.resolver(t)
	s := &savedFake{Service: r.Recommendations, plan: recommendations.SavedPlan{ID: practiceTopic, IsOwner: true, Revision: 2, Document: recommendations.SavedDocument{Goal: "Junior AI", Minutes: 60, SessionCount: 3, Materials: []recommendations.SavedMaterial{{ID: "python", Status: "DONE"}}, Tasks: []recommendations.SavedTask{{ID: practiceTask, VersionID: practiceVersion}}}}}
	r.Recommendations = s
	return r, s, f
}
func TestSavedPlanUsesOnlyOwnerEvidence(t *testing.T) {
	r, s, _ := savedFixture(t)
	p, err := r.SavedLearningPlan(practiceContext(), practiceTopic)
	if err != nil || !p.ProgressAvailable || !p.Tasks[0].Completed || s.actor != practiceUser {
		t.Fatal(p, err)
	}
}
func TestPublicSavedPlanDoesNotLoadOwnerHistory(t *testing.T) {
	r, s, f := savedFixture(t)
	s.plan.IsOwner = false
	s.plan.Public = true
	s.plan.Document.Materials[0].Status = "TODO"
	p, err := r.SavedLearningPlan(context.Background(), practiceTopic)
	if err != nil || p.ProgressAvailable || f.calls != 0 || s.actor != "" {
		t.Fatal("public history access", p, err)
	}
}
func TestSavedMutationsRequireAuthAndOwnerBeforeInference(t *testing.T) {
	r, s, f := savedFixture(t)
	m := &mutationResolver{r.Resolver}
	if _, err := m.UpdateLearningMaterial(context.Background(), practiceTopic, 2, "python", model.MaterialProgressDone); err == nil {
		t.Fatal("anonymous write")
	}
	s.plan.IsOwner = false
	if _, err := m.RefreshLearningPlan(practiceContext(), practiceTopic, 2); err == nil || f.calls != 0 || s.writes != 0 {
		t.Fatal("public refresh allowed")
	}
	s.plan.IsOwner = true
	if _, err := m.RefreshLearningPlan(practiceContext(), practiceTopic, 1); err == nil || f.calls != 0 {
		t.Fatal("stale refresh performed inference")
	}
}
func TestSavedPlanHistoryFailureIsNotZeroProgress(t *testing.T) {
	r, _, f := savedFixture(t)
	f.failPath = "/v1/me/submissions"
	p, err := r.SavedLearningPlan(practiceContext(), practiceTopic)
	if err != nil || p.ProgressAvailable || p.Materials[0].Status != model.MaterialProgressDone {
		t.Fatal("history outage hid material progress", p, err)
	}
}

func TestSavedPlanRecordsAcceptedDifferentVersion(t *testing.T) {
	r, s, _ := savedFixture(t)
	s.plan.Document.Tasks[0].VersionID = practiceBase
	p, err := r.SavedLearningPlan(practiceContext(), practiceTopic)
	if err != nil || !p.Tasks[0].Completed || p.Tasks[0].CompletionVersionID == nil || *p.Tasks[0].CompletionVersionID != practiceVersion || p.Tasks[0].VersionID != practiceBase {
		t.Fatal("version provenance lost", p, err)
	}
}

package graphql

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

func savedID(id string) bool { u, err := uuid.Parse(id); return err == nil && u != uuid.Nil }
func (r *Resolver) savedService() (recommendations.SavedService, error) {
	s, ok := r.Recommendations.(recommendations.SavedService)
	if !ok {
		return nil, practiceUnavailable()
	}
	return s, nil
}
func (r *queryResolver) MyLearningPlans(ctx context.Context) ([]*model.SavedLearningPlan, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	s, err := r.savedService()
	if err != nil {
		return nil, err
	}
	plans, err := s.ListSaved(ctx, auth.UserID)
	if err != nil {
		return nil, err
	}
	out := []*model.SavedLearningPlan{}
	// Список не делает 50 запросов истории: процент задач показывается на странице плана.
	for _, p := range plans {
		out = append(out, savedModel(p, false))
	}
	return out, nil
}
func (r *queryResolver) SavedLearningPlan(ctx context.Context, id string) (*model.SavedLearningPlan, error) {
	if !savedID(id) {
		return nil, practiceInvalid()
	}
	s, err := r.savedService()
	if err != nil {
		return nil, err
	}
	auth, _ := middleware.Auth(ctx)
	p, err := s.GetSaved(ctx, id, auth.UserID)
	if err != nil {
		return nil, err
	}
	return r.savedWithProgress(ctx, p), nil
}
func (r *Resolver) savedWithProgress(ctx context.Context, p recommendations.SavedPlan) *model.SavedLearningPlan {
	if p.IsOwner && len(p.Document.Tasks) == 0 {
		return savedModel(p, true)
	}
	available := false
	completedVersions := map[string]string{}
	if p.IsOwner && r.Tasks != nil {
		auth, _ := middleware.Auth(ctx)
		q := &queryResolver{r}
		history, partial, err := q.practiceEvidence(ctx, tasks.Actor{UserID: auth.UserID, Roles: auth.Roles})
		if err == nil && !partial {
			available = true
			accepted := map[string]bool{}
			acceptedByTask := map[string]string{}
			for _, e := range history {
				if e.Verdict == "accepted" {
					accepted[e.TaskID+"/"+e.TaskVersionID] = true
					acceptedByTask[e.TaskID] = e.TaskVersionID
				}
			}
			for i := range p.Document.Tasks {
				task := &p.Document.Tasks[i]
				version := acceptedByTask[task.ID]
				if accepted[task.ID+"/"+task.VersionID] {
					version = task.VersionID
				}
				task.Completed = version != ""
				completedVersions[task.ID] = version
			}
		}
	}
	out := savedModel(p, available)
	for _, task := range out.Tasks {
		if version := completedVersions[task.ID]; version != "" {
			task.CompletionVersionID = &version
		}
	}
	return out
}
func savedModel(p recommendations.SavedPlan, available bool) *model.SavedLearningPlan {
	d := p.Document
	out := &model.SavedLearningPlan{ID: p.ID, Goal: d.Goal, IsOwner: p.IsOwner, Public: p.Public, Revision: int(p.Revision), Minutes: d.Minutes, SessionCount: d.SessionCount, ProgressAvailable: available, Materials: []*model.SavedLearningMaterial{}, Tasks: []*model.SavedLearningTask{}}
	for _, m := range d.Materials {
		out.Materials = append(out.Materials, &model.SavedLearningMaterial{ID: m.ID, Title: m.Title, URL: m.URL, Action: m.Action, Minutes: m.Minutes, Status: model.MaterialProgress(m.Status)})
	}
	for _, t := range d.Tasks {
		out.Tasks = append(out.Tasks, &model.SavedLearningTask{ID: t.ID, VersionID: t.VersionID, Title: t.Title, Minutes: t.Minutes, Completed: t.Completed})
	}
	return out
}
func documentFrom(p *model.LearningPlan, goal string, minutes, sessions int) recommendations.SavedDocument {
	d := recommendations.SavedDocument{Goal: goal, Minutes: minutes, SessionCount: sessions, TopicIDs: []string{}, Materials: []recommendations.SavedMaterial{}, Tasks: []recommendations.SavedTask{}}
	for _, t := range p.Topics {
		d.TopicIDs = append(d.TopicIDs, t.ID)
	}
	for _, m := range p.Materials {
		d.Materials = append(d.Materials, recommendations.SavedMaterial{ID: m.ID, Title: m.Title, URL: m.URL, Action: m.Action, Minutes: m.Minutes, Status: "TODO"})
	}
	if p.Practice != nil {
		for _, session := range p.Practice.Sessions {
			for _, t := range session.Items {
				d.Tasks = append(d.Tasks, recommendations.SavedTask{ID: t.TaskID, VersionID: t.TaskVersionID, Title: t.Title, Minutes: t.EstimatedMinutes})
			}
		}
	}
	return d
}
func (r *mutationResolver) SaveLearningPlan(ctx context.Context, goal string, minutes, sessionCount int, requestID string) (*model.SavedLearningPlan, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if !savedID(requestID) {
		return nil, practiceInvalid()
	}
	s, err := r.savedService()
	if err != nil {
		return nil, err
	}
	existing, getErr := s.GetSaved(ctx, requestID, auth.UserID)
	if getErr == nil {
		if !existing.IsOwner {
			return nil, &recommendations.Error{Code: "NOT_FOUND"}
		}
		return r.savedWithProgress(ctx, existing), nil
	}
	var upstream *recommendations.Error
	if !errors.As(getErr, &upstream) || upstream.Code != "NOT_FOUND" {
		return nil, getErr
	}
	p, err := (&queryResolver{r.Resolver}).LearningPlan(ctx, goal, minutes, sessionCount)
	if err != nil {
		return nil, err
	}
	if p.AiStatus != "GENERATED" && p.AiStatus != "CACHED" {
		return nil, practiceUnavailable()
	}
	saved, err := s.WriteSaved(ctx, "", "", auth.UserID, recommendations.SavedInput{ID: requestID, Document: documentFrom(p, goal, minutes, sessionCount)})
	if err != nil {
		return nil, err
	}
	return r.savedWithProgress(ctx, saved), nil
}
func (r *mutationResolver) UpdateLearningMaterial(ctx context.Context, id string, revision int, materialID string, status model.MaterialProgress) (*model.SavedLearningPlan, error) {
	return r.changeSaved(ctx, id, "material", recommendations.SavedInput{Revision: int64(revision), MaterialID: materialID, Status: string(status)})
}
func (r *mutationResolver) SetLearningPlanPublic(ctx context.Context, id string, revision int, public bool) (*model.SavedLearningPlan, error) {
	return r.changeSaved(ctx, id, "visibility", recommendations.SavedInput{Revision: int64(revision), Public: public})
}
func (r *mutationResolver) changeSaved(ctx context.Context, id, op string, in recommendations.SavedInput) (*model.SavedLearningPlan, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if !savedID(id) || in.Revision < 1 {
		return nil, practiceInvalid()
	}
	s, err := r.savedService()
	if err != nil {
		return nil, err
	}
	p, err := s.WriteSaved(ctx, id, op, auth.UserID, in)
	if err != nil {
		return nil, err
	}
	return r.savedWithProgress(ctx, p), nil
}
func (r *mutationResolver) RefreshLearningPlan(ctx context.Context, id string, revision int) (*model.SavedLearningPlan, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if !savedID(id) || revision < 1 {
		return nil, practiceInvalid()
	}
	s, err := r.savedService()
	if err != nil {
		return nil, err
	}
	old, err := s.GetSaved(ctx, id, auth.UserID)
	if err != nil {
		return nil, err
	}
	if !old.IsOwner {
		return nil, &recommendations.Error{Code: "NOT_FOUND"}
	}
	if old.Revision != int64(revision) {
		return nil, &recommendations.Error{Code: "CONFLICT"}
	}
	done := []string{}
	for _, m := range old.Document.Materials {
		if m.Status == "DONE" {
			done = append(done, m.ID)
		}
	}
	next, err := (&queryResolver{r.Resolver}).learningPlan(ctx, old.Document.Goal, old.Document.Minutes, old.Document.SessionCount, done)
	if err != nil {
		return nil, err
	}
	if next.AiStatus != "GENERATED" && next.AiStatus != "CACHED" && next.AiStatus != "NO_MATCH" {
		return nil, practiceUnavailable()
	}
	// При пустом обновлении сохраняем имеющийся документ и отметки.
	doc := documentFrom(next, old.Document.Goal, old.Document.Minutes, old.Document.SessionCount)
	if len(doc.Materials)+len(doc.Tasks) == 0 {
		doc = old.Document
	}
	return r.changeSaved(ctx, id, "refresh", recommendations.SavedInput{Revision: int64(revision), Document: doc})
}
func (r *mutationResolver) CloneLearningPlan(ctx context.Context, id, requestID string) (*model.SavedLearningPlan, error) {
	auth, err := middleware.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	if !savedID(id) || !savedID(requestID) || id == requestID {
		return nil, practiceInvalid()
	}
	s, err := r.savedService()
	if err != nil {
		return nil, err
	}
	p, err := s.WriteSaved(ctx, id, "clone", auth.UserID, recommendations.SavedInput{ID: requestID})
	if err != nil {
		return nil, err
	}
	return r.savedWithProgress(ctx, p), nil
}

func (r *mutationResolver) RemoveLearningPlanTask(ctx context.Context, id string, revision int, taskID string) (*model.SavedLearningPlan, error) {
	if !savedID(taskID) {
		return nil, practiceInvalid()
	}
	return r.changeSaved(ctx, id, "remove_task", recommendations.SavedInput{Revision: int64(revision), TaskID: taskID})
}

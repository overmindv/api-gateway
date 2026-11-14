package recommendations

import (
	"context"
	"net/http"
	"net/url"

	"github.com/overmindv/api-gateway/internal/client/httpx"
)

type SavedMaterial struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Action  string `json:"action"`
	Minutes int    `json:"minutes"`
	Status  string `json:"status"`
}
type SavedTask struct {
	ID        string `json:"id"`
	VersionID string `json:"version_id"`
	Title     string `json:"title"`
	Minutes   int    `json:"minutes"`
	Completed bool   `json:"completed"`
}
type SavedDocument struct {
	ExcludedTaskIDs []string        `json:"excluded_task_ids,omitempty"`
	Goal            string          `json:"goal"`
	Minutes         int             `json:"minutes"`
	SessionCount    int             `json:"session_count"`
	TopicIDs        []string        `json:"topic_ids"`
	Materials       []SavedMaterial `json:"materials"`
	Tasks           []SavedTask     `json:"tasks"`
}
type SavedPlan struct {
	ID       string        `json:"id"`
	IsOwner  bool          `json:"is_owner"`
	Public   bool          `json:"public"`
	Revision int64         `json:"revision"`
	Document SavedDocument `json:"document"`
}
type SavedInput struct {
	TaskID     string        `json:"task_id"`
	ID         string        `json:"id"`
	Revision   int64         `json:"revision"`
	Document   SavedDocument `json:"document"`
	Public     bool          `json:"public"`
	MaterialID string        `json:"material_id"`
	Status     string        `json:"status"`
}
type SavedService interface {
	GetSaved(context.Context, string, string) (SavedPlan, error)
	ListSaved(context.Context, string) ([]SavedPlan, error)
	WriteSaved(context.Context, string, string, string, SavedInput) (SavedPlan, error)
}

func (c *Client) GetSaved(ctx context.Context, id, actor string) (SavedPlan, error) {
	p, err := httpx.DoJSON[SavedPlan](c.hx, ctx, http.MethodGet, "/v1/learning-plans/"+url.PathEscape(id), nil, httpx.Actor{UserID: actor})
	return p, goalError(err)
}
func (c *Client) ListSaved(ctx context.Context, actor string) ([]SavedPlan, error) {
	p, err := httpx.DoJSON[[]SavedPlan](c.hx, ctx, http.MethodGet, "/v1/learning-plans", nil, httpx.Actor{UserID: actor})
	return p, goalError(err)
}
func (c *Client) WriteSaved(ctx context.Context, id, op, actor string, in SavedInput) (SavedPlan, error) {
	path := "/v1/learning-plans"
	if id != "" {
		path += "/" + url.PathEscape(id) + "/" + url.PathEscape(op)
	}
	p, err := httpx.DoJSON[SavedPlan](c.hx, ctx, http.MethodPost, path, in, httpx.Actor{UserID: actor})
	return p, goalError(err)
}

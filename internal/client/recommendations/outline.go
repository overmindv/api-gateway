package recommendations

import (
	"context"
	"net/http"

	"github.com/overmindv/api-gateway/internal/client/httpx"
)

type OutlineRequest struct {
	CompletedMaterialIDs []string `json:"completed_material_ids,omitempty"`
	Goal                 string   `json:"goal"`
	Topics               []Topic  `json:"topics"`
}
type Material struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Action  string `json:"action"`
	Minutes int    `json:"minutes"`
}
type Outline struct {
	AIStatus  string     `json:"ai_status"`
	TopicIDs  []string   `json:"topic_ids"`
	Materials []Material `json:"materials"`
}
type OutlineService interface {
	LearningOutline(context.Context, OutlineRequest, string) (Outline, error)
}

func (c *Client) LearningOutline(ctx context.Context, in OutlineRequest, actor string) (Outline, error) {
	out, err := httpx.DoJSON[Outline](c.hx, ctx, http.MethodPost, "/v1/learning-outline", in, httpx.Actor{UserID: actor})
	return out, goalError(err)
}

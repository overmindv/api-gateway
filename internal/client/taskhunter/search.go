package taskhunter

import (
	"context"
	"net/http"
	"net/url"
)

type SearchResult struct {
	Title  string   `json:"title"`
	URL    string   `json:"url"`
	Rating int      `json:"rating"`
	Tags   []string `json:"tags"`
}
type SearchService interface {
	Search(context.Context, string, Actor) ([]SearchResult, error)
}

func (c *Client) Search(ctx context.Context, q string, actor Actor) ([]SearchResult, error) {
	return request[[]SearchResult](c, ctx, http.MethodGet, "/v1/admin/task-search?q="+url.QueryEscape(q), nil, actor)
}

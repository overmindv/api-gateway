package graphql

import (
	"context"
	"testing"

	"github.com/overmindv/api-gateway/internal/client/feed"
	"github.com/overmindv/api-gateway/internal/graphql/model"
)

type feedFixture struct {
	page feed.FeedList
}

func (f feedFixture) ListFeed(context.Context, int, int) (feed.FeedList, error) {
	return f.page, nil
}

func (f feedFixture) Health(context.Context) error { return nil }

func TestFeedKeepsMaterialOnMixedPage(t *testing.T) {
	t.Parallel()

	const href = "/materials/a3300000-0000-4000-8000-000000000001"
	r := &queryResolver{&Resolver{Feed: feedFixture{page: feed.FeedList{
		Items: []feed.Item{{ID: "public", Kind: "material.published", Href: href}, {ID: "task", Kind: "task.published"}},
		Limit: 2, Offset: 8,
	}}}}
	page, err := r.Feed(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Items[0].Kind != model.FeedKindMaterialPublished ||
		page.Items[0].Href != href || page.Items[1].Kind != model.FeedKindTaskPublished ||
		page.Limit != 2 || page.Offset != 8 {
		t.Fatal(page)
	}
}

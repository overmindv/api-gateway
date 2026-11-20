package graphql

import (
	"context"
	"testing"

	"github.com/overmindv/api-gateway/internal/client/media"
)

type avatarMedia struct {
	media.Service
	files []media.PublicFile
}

func (m avatarMedia) ResolvePublicFiles(context.Context, []string, []string) ([]media.PublicFile, error) {
	return m.files, nil
}

func TestAvatarUsesOnlyPreviews(t *testing.T) {
	resolver := Resolver{Media: avatarMedia{files: []media.PublicFile{
		{FileID: "small", URLs: map[string]string{"w128": "preview"}},
		{FileID: "legacy", URLs: map[string]string{"original": "original"}},
	}}}
	got := resolver.resolveAvatars(context.Background(), []string{"small", "legacy"})
	if avatar := got["small"]; avatar == nil || avatar.SmallURL != "preview" || avatar.MediumURL != "preview" {
		t.Error("one small preview should serve both sizes")
	}
	if got["legacy"] != nil {
		t.Error("avatar must not fall back to an original")
	}
}

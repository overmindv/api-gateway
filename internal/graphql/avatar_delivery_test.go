package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/graphql/model"
	"github.com/overmindv/api-gateway/internal/middleware"
)

type avatarMedia struct {
	media.Service
	files []media.PublicFile
}

func (m avatarMedia) ResolvePublicFiles(context.Context, []string, []string) ([]media.PublicFile, error) {
	return m.files, nil
}

func (m avatarMedia) DownloadURL(context.Context, string, string, media.Actor) (media.Download, error) {
	return media.Download{URL: "https://media.invalid/signed"}, nil
}

func TestAvatarUsesOnlyPreviews(t *testing.T) {
	t.Parallel()

	const preview = "preview"
	resolver := Resolver{Media: avatarMedia{files: []media.PublicFile{
		{FileID: "small", URLs: map[string]string{"w128": preview}},
		{FileID: "legacy", URLs: map[string]string{"original": "original"}},
	}}}
	got := resolver.resolveAvatars(context.Background(), []string{"small", "legacy"})
	if avatar := got["small"]; avatar == nil || avatar.SmallURL != preview || avatar.MediumURL != preview {
		t.Error("one small preview should serve both sizes")
	}
	if got["legacy"] != nil {
		t.Error("avatar must not fall back to an original")
	}
}

type avatarUsers struct {
	users.UserService
	fileID string
}

func (u avatarUsers) GetPublicUser(_ context.Context, id string) (*users.PublicUser, error) {
	return &users.PublicUser{ID: id, AvatarFileID: &u.fileID}, nil
}

func TestGraphQLMediaDelivery(t *testing.T) {
	t.Parallel()

	client := avatarMedia{files: []media.PublicFile{{
		FileID: "small",
		URLs:   map[string]string{"w128": "https://media.invalid/preview.webp"},
	}}}
	got := queryMediaProfile(t, client, "small")
	if got.SmallURL != "https://media.invalid/preview.webp" || got.SmallURL != got.MediumURL {
		t.Errorf("expected the available preview for both sizes: %+v", got)
	}
}

func TestGraphQLMediaDeliveryWithMedia(t *testing.T) {
	t.Parallel()

	baseURL := os.Getenv("MEDIA_DELIVERY_TEST_URL")
	if baseURL == "" {
		t.Skip("set MEDIA_DELIVERY_TEST_URL to a dedicated Media instance")
	}
	fileID := os.Getenv("MEDIA_DELIVERY_TEST_FILE_ID")
	if fileID == "" {
		t.Fatal("a prepared small avatar is required")
	}
	client := media.New(baseURL, os.Getenv("MEDIA_DELIVERY_TEST_SERVICE_TOKEN"), 5*time.Second, slog.Default())
	got := queryMediaProfile(t, client, fileID)
	if got.SmallURL != got.MediumURL || !strings.HasSuffix(got.SmallURL, "/w128.webp") {
		t.Error("expected a single small WebP for both sizes")
	}
}

func queryMediaProfile(t *testing.T, client media.Service, fileID string) model.UserAvatar {
	t.Helper()

	const owner = "11111111-1111-4111-8111-111111111111"
	body, err := json.Marshal(map[string]any{
		"query": `query ($owner: ID!, $file: ID!) {
			userProfile(id: $owner) { avatar { fileId smallUrl mediumUrl } }
			mediaDownloadUrl(fileId: $file) { url }
		}`,
		"variables": map[string]string{"owner": owner, "file": fileID},
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := HandlerWithMedia(avatarUsers{fileID: fileID}, nil, nil, nil, client, slog.Default())
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.ContextWithAuth(req.Context(), middleware.AuthInfo{UserID: owner, Token: "fixture"}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("GraphQL media responses must not be cached", response.Code)
	}
	var got struct {
		Data struct {
			UserProfile struct {
				Avatar *model.UserAvatar
			}
			MediaDownloadURL struct {
				URL string
			}
		}
		Errors []json.RawMessage
	}
	if err = json.Unmarshal(response.Body.Bytes(), &got); err != nil || len(got.Errors) != 0 ||
		got.Data.UserProfile.Avatar == nil || got.Data.MediaDownloadURL.URL == "" {
		t.Fatal("GraphQL did not return the avatar and download link", err)
	}
	if got.Data.UserProfile.Avatar.FileID != fileID {
		t.Fatal("GraphQL returned another avatar")
	}

	return *got.Data.UserProfile.Avatar
}

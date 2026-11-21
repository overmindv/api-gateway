package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/config"
	graphqldelivery "github.com/overmindv/api-gateway/internal/graphql"
	"github.com/overmindv/api-gateway/internal/middleware"
)

const materialContractOrigin = "http://localhost:5173"

type contractRevision struct {
	ID     string
	Number int
	Source string
}

type contractMaterial struct {
	ID       string
	AuthorID string
	Status   string
	Title    string
	Revision *contractRevision
}

type materialContractResponse struct {
	Data struct {
		CreateMaterialDraft *contractMaterial
		Material            *contractMaterial
		PublishedMaterial   *contractMaterial
		SaveMaterialDraft   *contractRevision
	}
	Errors []struct {
		Extensions struct{ Code string }
	}
}

func TestMaterialContentContract(t *testing.T) {
	t.Parallel()

	address, token := os.Getenv("CONTENT_CONTRACT_GRPC_ADDR"), os.Getenv("CONTENT_CONTRACT_TOKEN")
	if address == "" && token == "" {
		t.Skip("run scripts/test-content-contract.sh with an isolated Postgres fixture")
	}
	if host, _, err := net.SplitHostPort(address); err != nil || host != "127.0.0.1" || token == "" {
		t.Fatal("the Content contract fixture must use an authenticated loopback endpoint")
	}
	handler, sign := materialContractHandler(t, address, token)
	owner := uuid.NewString()
	ownerCookie, readerCookie := sign(owner), sign(uuid.NewString())
	run := func(cookie, query string, vars map[string]any) materialContractResponse {
		t.Helper()
		reqJSON, err := json.Marshal(map[string]any{"query": query, "variables": vars})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", bytes.NewReader(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", materialContractOrigin)
		req.Header.Set("X-User-ID", owner)
		req.Header.Set("X-User-Roles", "admin")
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
		}
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		var decoded materialContractResponse
		if res.Code != http.StatusOK || json.NewDecoder(res.Body).Decode(&decoded) != nil {
			t.Fatal("invalid GraphQL response, HTTP status", res.Code)
		}

		return decoded
	}

	const create = `mutation($input:CreateMaterialDraftInput!) {
	  createMaterialDraft(input:$input) { id authorId status title revision { id number source } }
	}`
	const firstText = "# First draft"
	first := run(ownerCookie, create, map[string]any{"input": map[string]any{
		"type": "note", "title": "Draft title", "slug": "contract-" + uuid.NewString(), "source": firstText,
	}})
	draft := first.Data.CreateMaterialDraft
	if len(first.Errors) != 0 || draft == nil || draft.ID == "" || draft.AuthorID != owner || draft.Status != draftStatus ||
		draft.Revision == nil || draft.Revision.ID == "" || draft.Revision.Number != 1 || draft.Revision.Source != firstText {
		t.Fatal("Content did not return the created draft and its revision")
	}

	const read = `query($id:ID!) { material(id:$id) { id authorId status title revision { id number source } } }`
	const published = `query($id:ID!) { publishedMaterial(id:$id) { id revision { source } } }`
	find := map[string]any{"id": draft.ID}
	for _, tc := range []struct{ session, document, code string }{
		{"", read, "UNAUTHENTICATED"},
		{readerCookie, read, "NOT_FOUND"},
		{ownerCookie, published, "NOT_FOUND"},
	} {
		result := run(tc.session, tc.document, find)
		requireMaterialContractError(t, result, tc.code)
		if result.Data.Material != nil || result.Data.PublishedMaterial != nil {
			t.Error("the response exposed an unpublished draft")
		}
	}

	const save = `mutation($input:SaveMaterialDraftInput!) { saveMaterialDraft(input:$input) { id number source } }`
	saveInput := func(revision, title, source string) map[string]any {
		return map[string]any{"input": map[string]any{
			"materialId": draft.ID, "expectedRevisionId": revision, "source": source,
			"metadata": map[string]any{"title": title, "topicIds": []string{}},
		}}
	}
	requireMaterialContractError(t, run(readerCookie, save, saveInput(draft.Revision.ID, "Wrong actor", "# Not yours")), "NOT_FOUND")
	const nextTitle, nextText = "Renamed draft", "# Second revision"
	next := run(ownerCookie, save, saveInput(draft.Revision.ID, nextTitle, nextText))
	saved := next.Data.SaveMaterialDraft
	if len(next.Errors) != 0 || saved == nil || saved.ID == "" || saved.ID == draft.Revision.ID || saved.Number != 2 || saved.Source != nextText {
		t.Fatal("Content did not save a new draft revision")
	}

	requireMaterialContractError(t, run(ownerCookie, save, saveInput(draft.Revision.ID, "Stale title", "# Old")),
		"REVISION_CONFLICT")
	requireMaterialContractError(t, run(ownerCookie, save, saveInput(saved.ID, "Invalid title", "<div>Not Markdown</div>")),
		"INVALID_ARGUMENT")
	last := run(ownerCookie, read, find)
	stored := last.Data.Material
	if len(last.Errors) != 0 || stored == nil || stored.AuthorID != owner || stored.Status != draftStatus ||
		stored.Title != nextTitle || stored.Revision == nil || *stored.Revision != *saved {
		t.Error("read did not return the saved title and revision after rejected requests")
	}
}

func materialContractHandler(t *testing.T, address, token string) (http.Handler, func(string) string) {
	t.Helper()
	client, err := contentclient.New(config.Content{Address: address, ServiceToken: token, Timeout: 3 * time.Second, Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	key, issuer := uuid.NewString()+uuid.NewString(), "users"
	auth := middleware.NewJWTAuthenticator(key, issuer)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := graphqldelivery.HandlerWithRecommendationsAndMetrics(nil, nil, nil, nil, nil, nil, nil, log, nil,
		graphqldelivery.HandlerOptions{Content: client})
	wrapped := middleware.RequestIDMiddleware(middleware.BrowserSecurity(
		middleware.BrowserSecurityConfig{AllowedOrigins: []string{materialContractOrigin}}, middleware.JWT(auth, handler)))

	return wrapped, func(actor string) string {
		t.Helper()
		cookie, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Issuer: issuer, Subject: actor, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		}).SignedString([]byte(key))
		if err != nil {
			t.Fatal(err)
		}

		return cookie
	}
}

func requireMaterialContractError(t *testing.T, result materialContractResponse, code string) {
	t.Helper()
	if len(result.Errors) != 1 || result.Errors[0].Extensions.Code != code {
		t.Error("unexpected GraphQL errors, want", code, "got", result.Errors)
	}
}

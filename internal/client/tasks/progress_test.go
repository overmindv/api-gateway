package tasks

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProgressClientScopeAndActor(t *testing.T) {
	const actorID = "00000000-0000-4000-8000-000000000003"
	const topicID = "00000000-0000-4000-8000-000000000022"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-ID") != actorID {
			t.Errorf("actor not forwarded: %q", r.Header.Get("X-User-ID"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/users/00000000-0000-4000-8000-000000000004/practice-progress":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"code":"PERMISSION_DENIED","message":"private"}`)
		case "/v1/me/practice-progress/visibility":
			var input struct {
				Public bool `json:"public"`
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || !input.Public {
				t.Errorf("visibility body: %+v %v", input, err)
			}
			_, _ = io.WriteString(w, `{"public":true}`)
		case "/v1/practice-leaderboard":
			if r.URL.Query().Get("scope") != "course" || r.URL.Query().Get("topic_id") != topicID || r.URL.Query().Get("limit") != "50" {
				t.Errorf("course scope: %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `[{"user_id":"`+actorID+`","score":20,"solved_tasks":1}]`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := New(server.URL, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	actor := Actor{UserID: actorID, Roles: []string{"user"}}
	private, err := client.UserProgress(context.Background(), "00000000-0000-4000-8000-000000000004", actor)
	if err != nil || private != nil {
		t.Fatalf("private progress=%+v err=%v", private, err)
	}
	public, err := client.SetMyProgressPublic(context.Background(), true, actor)
	if err != nil || !public {
		t.Fatalf("visibility=%v err=%v", public, err)
	}
	topics := []string{topicID}
	rows, err := client.RatedUsers(context.Background(), &topics, 50, 0, actor)
	if err != nil || len(rows) != 1 || rows[0].Score != 20 {
		t.Fatalf("rating=%+v err=%v", rows, err)
	}
}

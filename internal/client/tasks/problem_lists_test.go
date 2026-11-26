package tasks

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestProblemListClientRoutesByRole проверяет, что обычный пользователь
// обращается к публичной подборке, а администратор — к полной очереди.
func TestProblemListClientRoutesByRole(t *testing.T) {
	t.Parallel()

	const listID = "00000000-0000-4000-8000-000000000010"
	paths := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == publicProblemListsPath || r.URL.Path == adminProblemListsPath {
			_, _ = io.WriteString(w, `{"items":[],"limit":10,"offset":0}`)

			return
		}
		_, _ = io.WriteString(w, `{"id":"`+listID+`","name":"Top 100","source_id":"leetcode",`+
			`"source_url":"https://leetcode.com/problem-list/abc","truncated":true,"member_count":1,`+
			`"pending_candidates":2,"created_at":"2026-11-24T00:00:00Z","updated_at":"2026-11-24T00:00:00Z",`+
			`"members":[{"task_id":"00000000-0000-4000-8000-000000000011","title":"Two Sum","difficulty":"easy","status":"published"}]}`)
	}))
	defer server.Close()
	client := New(server.URL, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	student := Actor{UserID: "00000000-0000-4000-8000-000000000001", Roles: []string{"student"}}
	admin := Actor{UserID: "00000000-0000-4000-8000-000000000002", Roles: []string{"admin"}}

	if _, err := client.ListProblemLists(context.Background(), 10, 0, student); err != nil {
		t.Fatal(err)
	}
	detail, err := client.GetProblemList(context.Background(), listID, student)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.Truncated || detail.PendingCandidates != 2 || len(detail.Members) != 1 {
		t.Fatalf("публичные детали списка: %+v", detail)
	}
	if _, err := client.ListProblemLists(context.Background(), 10, 0, admin); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetProblemList(context.Background(), listID, admin); err != nil {
		t.Fatal(err)
	}

	want := []string{
		publicProblemListsPath, publicProblemListsPath + "/" + listID,
		adminProblemListsPath, adminProblemListsPath + "/" + listID,
	}
	if len(paths) != len(want) {
		t.Fatalf("маршруты: %v", paths)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Fatalf("маршрут %d: %s, ожидался %s", index, paths[index], want[index])
		}
	}
}

// TestActorIsAdmin повторяет правило ролей gateway без учёта регистра.
func TestActorIsAdmin(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		roles []string
		want  bool
	}{
		"empty":     {nil, false},
		"student":   {[]string{"student"}, false},
		"admin":     {[]string{"admin"}, true},
		"superuser": {[]string{"SuperUser"}, true},
		"mixed":     {[]string{"student", "admin"}, true},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := (Actor{Roles: testCase.roles}).IsAdmin(); got != testCase.want {
				t.Fatalf("IsAdmin(%v) = %v", testCase.roles, got)
			}
		})
	}
}

package recommendations

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

func TestPracticePlanUsesServiceTokenAndMinimalContract(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/practice-plan" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer service-secret" || r.Header.Get("X-User-ID") != "verified-user" {
			t.Errorf("wrong credentials")
		}
		if r.Header.Get("X-User-Roles") != "" {
			t.Errorf("roles should not be forwarded")
		}
		if r.Header.Get("X-Request-ID") != "request-id" {
			t.Errorf("missing request ID")
		}
		data, _ := io.ReadAll(r.Body)
		for _, secret := range []string{"user-jwt", "selected_option", "correct_option", "source_code", "hidden_tests"} {
			if strings.Contains(string(data), secret) {
				t.Errorf("private data forwarded: %s", secret)
			}
		}
		var input map[string]json.RawMessage
		if err := json.Unmarshal(data, &input); err != nil {
			t.Fatal(err)
		}
		if len(input) != 6 || string(input["input_truncated"]) != "false" {
			t.Errorf("unexpected payload: %s", data)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"policy_version":"practice-v1","total_minutes":10,"warnings":[],"items":[{"task_id":"task","task_version_id":"version","topic_id":"topic","title":"Task","difficulty":"easy","task_type":"single_choice","estimated_minutes":10,"reason_code":"START_TOPIC"}]}`)
	}))
	defer upstream.Close()
	ctx := middleware.ContextWithAuth(context.Background(), middleware.AuthInfo{UserID: "verified-user", Token: "user-jwt", Roles: []string{"admin"}})
	ctx = middleware.WithRequestID(ctx, "request-id")
	client := New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	result, err := client.PracticePlan(ctx, Request{TopicIDs: []string{"topic"}, Minutes: 60, Topics: []Topic{}, Tasks: []Task{}, Evidence: []Evidence{}}, "verified-user")
	if err != nil {
		t.Fatal(err)
	}
	if result.PolicyVersion != "practice-v1" || len(result.Items) != 1 || result.Items[0].TaskVersionID != "version" {
		t.Fatalf("unexpected plan: %+v", result)
	}
}

func TestPracticePlanHidesUpstreamDetailsAndRejectsRedirect(t *testing.T) {
	var logs strings.Builder
	targetCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls++; w.WriteHeader(200) }))
	defer target.Close()
	for _, status := range []int{400, 503, 307} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status == 307 {
					w.Header().Set("Location", target.URL)
				}
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"code":"INVALID_ARGUMENT","message":"secret upstream host and token"}`)
			}))
			defer upstream.Close()
			client := New(upstream.URL, "service-secret", time.Second, slog.New(slog.NewTextHandler(&logs, nil)))
			_, err := client.PracticePlan(context.Background(), Request{}, "user")
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
		})
	}
	if targetCalls != 0 {
		t.Fatalf("followed redirect carrying actor/token")
	}
	if strings.Contains(logs.String(), "secret upstream") {
		t.Fatal("logged upstream private message")
	}
}

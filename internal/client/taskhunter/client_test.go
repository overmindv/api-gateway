package taskhunter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

// TestClientStartsJobWithTrustedContext проверяет service token и actor headers.
func TestClientStartsJobWithTrustedContext(t *testing.T) {
	t.Parallel()

	client := New("http://task-hunter.local", "service-token", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	client.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/admin/collection-jobs" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer service-token" {
			t.Fatalf("service token is missing")
		}
		if request.Header.Get("X-User-ID") != "admin-id" || request.Header.Get("X-User-Roles") != "admin" {
			t.Fatalf("unexpected actor headers: %v", request.Header)
		}
		var input CreateJobInput
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		if input.IdempotencyKey != "job-key" || len(input.WebsiteURLs) != 1 {
			t.Fatalf("unexpected input: %+v", input)
		}

		return jsonResponse(t, http.StatusAccepted, Job{ID: "job-id", Status: "queued"}), nil
	})

	job, err := client.StartJob(context.Background(), CreateJobInput{
		IdempotencyKey: "job-key",
		WebsiteURLs:    []string{"https://leetcode.com/problems/two-sum"},
	}, Actor{UserID: "admin-id", Roles: []string{"admin"}})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "job-id" || job.Status != "queued" {
		t.Fatalf("unexpected job: %+v", job)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip выполняет тестовый HTTP-запрос без listener.
func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

// jsonResponse создаёт HTTP-ответ с JSON-телом.
func jsonResponse(t *testing.T, status int, body any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

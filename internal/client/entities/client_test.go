package entities

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

// TestCreateRequestsOmitResponseOnlyFields проверяет, что create-запросы не отправляют пустые response-only поля.
func TestCreateRequestsOmitResponseOnlyFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		call     func(context.Context, *Client, Actor) error
		response map[string]any
	}{
		{
			name: "university",
			path: "/v1/universities",
			call: func(ctx context.Context, client *Client, actor Actor) error {
				_, err := client.CreateUniversity(ctx, University{
					Name:   "University",
					Status: "draft",
				}, actor)

				return err
			},
			response: map[string]any{
				"id":           "11111111-1111-1111-1111-111111111111",
				"name":         "University",
				"short_name":   "",
				"city":         "",
				"country":      "",
				"website_url":  "",
				"logo_file_id": nil,
				"status":       "draft",
				"created_at":   "2026-07-30T10:00:00Z",
				"updated_at":   "2026-07-30T10:00:00Z",
			},
		},
		{
			name: "program",
			path: "/v1/programs",
			call: func(ctx context.Context, client *Client, actor Actor) error {
				_, err := client.CreateProgram(ctx, Program{
					Name:        "Program",
					DegreeLevel: "other",
					Status:      "draft",
				}, actor)

				return err
			},
			response: map[string]any{
				"id":            "22222222-2222-2222-2222-222222222222",
				"university_id": nil,
				"name":          "Program",
				"short_name":    "",
				"faculty":       "",
				"degree_level":  "other",
				"start_year":    nil,
				"status":        "draft",
				"created_at":    "2026-07-30T10:00:00Z",
				"updated_at":    "2026-07-30T10:00:00Z",
			},
		},
		{
			name: "course",
			path: "/v1/courses",
			call: func(ctx context.Context, client *Client, actor Actor) error {
				_, err := client.CreateCourse(ctx, Course{
					Name:   "Course",
					Slug:   "course",
					Status: "draft",
				}, actor)

				return err
			},
			response: map[string]any{
				"id":          "33333333-3333-3333-3333-333333333333",
				"program_id":  nil,
				"name":        "Course",
				"slug":        "course",
				"description": "",
				"semester":    nil,
				"year_number": nil,
				"status":      "draft",
				"created_at":  "2026-07-30T10:00:00Z",
				"updated_at":  "2026-07-30T10:00:00Z",
			},
		},
		{
			name: "topic",
			path: "/v1/topics",
			call: func(ctx context.Context, client *Client, actor Actor) error {
				_, err := client.CreateTopic(ctx, Topic{
					Title:      "Topic",
					Slug:       "topic",
					Difficulty: "basic",
					Status:     "draft",
				}, actor)

				return err
			},
			response: map[string]any{
				"id":              "44444444-4444-4444-4444-444444444444",
				"course_id":       nil,
				"parent_topic_id": nil,
				"title":           "Topic",
				"slug":            "topic",
				"description":     "",
				"order_index":     0,
				"difficulty":      "basic",
				"status":          "draft",
				"created_at":      "2026-07-30T10:00:00Z",
				"updated_at":      "2026-07-30T10:00:00Z",
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client := New("http://entities.local", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
			client.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != http.MethodPost || r.URL.Path != test.path {
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("X-User-ID") != "55555555-5555-5555-5555-555555555555" {
					t.Fatalf("unexpected actor header: %q", r.Header.Get("X-User-ID"))
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				for _, field := range []string{"id", "created_at", "updated_at"} {
					if _, ok := body[field]; ok {
						t.Fatalf("%s must not be sent in create request body: %#v", field, body)
					}
				}
				data, err := json.Marshal(test.response)
				if err != nil {
					t.Fatal(err)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(bytes.NewReader(data)),
				}, nil
			})
			err := test.call(context.Background(), client, Actor{
				UserID: "55555555-5555-5555-5555-555555555555",
				Roles:  []string{"admin"},
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip выполняет тестовый HTTP request без открытия сетевого listener.
func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

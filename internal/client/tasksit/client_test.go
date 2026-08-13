package tasksit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
)

// TestClientForwardsFiltersAndActor проверяет query и доверенные actor headers.
func TestClientForwardsFiltersAndActor(t *testing.T) {
	t.Parallel()

	client := testClient(t, func(request *http.Request) *http.Response {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/admin/tasks" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		query := request.URL.Query()
		if query.Get("status") != "draft" || query.Get("task_type") != "single_choice" || query.Get("difficulty") != "easy" {
			t.Fatalf("unexpected filters: %v", query)
		}
		if query.Get("topic_id") != "topic-id" || query.Get("limit") != "15" || query.Get("offset") != "5" {
			t.Fatalf("unexpected pagination: %v", query)
		}
		if request.Header.Get("X-User-ID") != "user-id" || request.Header.Get("X-User-Roles") != "admin,editor" {
			t.Fatalf("unexpected actor headers: %v", request.Header)
		}
		if request.Header.Get(middleware.RequestIDHeader) != "request-id" {
			t.Fatalf("unexpected request id: %q", request.Header.Get(middleware.RequestIDHeader))
		}

		return jsonResponse(t, http.StatusOK, TaskList{
			Items: []TaskSummary{{
				ID:            "task-id",
				TaskVersionID: "version-id",
			}},
			Limit:  15,
			Offset: 5,
		})
	})
	topicID := "topic-id"
	ctx := middleware.WithRequestID(context.Background(), "request-id")
	result, err := client.ListAdmin(ctx, TaskFilter{
		Status:     "draft",
		TaskType:   "single_choice",
		Difficulty: "easy",
		TopicID:    &topicID,
		Limit:      15,
		Offset:     5,
	}, Actor{
		UserID: "user-id",
		Roles:  []string{"admin", "editor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].ID != "task-id" || result.Limit != 15 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// TestClientMapsTaskWrites проверяет JSON для создания и изменения теста.
func TestClientMapsTaskWrites(t *testing.T) {
	t.Parallel()

	requests := 0
	client := testClient(t, func(request *http.Request) *http.Response {
		requests++
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content type is missing")
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		switch request.Method {
		case http.MethodPost:
			if request.URL.Path != "/v1/admin/tasks" || body["task_type"] != "multiple_choice" {
				t.Fatalf("unexpected create request: %s %#v", request.URL.Path, body)
			}
		case http.MethodPatch:
			if request.URL.Path != "/v1/admin/tasks/task-id/status" || body["status"] != "published" {
				t.Fatalf("unexpected status request: %s %#v", request.URL.Path, body)
			}
		default:
			t.Fatalf("unexpected method: %s", request.Method)
		}

		return jsonResponse(t, http.StatusOK, sampleTask())
	})
	actor := Actor{UserID: "admin-id", Roles: []string{"admin"}}
	input := TaskInput{
		Title:      "Sets",
		Statement:  "Select all",
		TaskType:   "multiple_choice",
		Difficulty: "medium",
		Options: []TaskOptionInput{
			{Text: "A", IsCorrect: true},
			{Text: "B", IsCorrect: false},
		},
	}
	if _, err := client.Create(context.Background(), input, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ChangeStatus(context.Background(), "task-id", "published", actor); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d", requests)
	}
}

// TestClientMapsSubmissionAndHistory проверяет отправку и чтение решений.
func TestClientMapsSubmissionAndHistory(t *testing.T) {
	t.Parallel()

	requests := 0
	client := testClient(t, func(request *http.Request) *http.Response {
		requests++
		if request.Header.Get("X-User-ID") != "user-id" {
			t.Fatalf("actor header is missing")
		}
		switch request.URL.Path {
		case "/v1/tasks/task-id/submissions":
			if request.Method != http.MethodPost {
				t.Fatalf("unexpected submit method: %s", request.Method)
			}
			var body SubmissionInput
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.IdempotencyKey != "key-id" || len(body.SelectedOptionIDs) != 1 {
				t.Fatalf("unexpected submit input: %+v", body)
			}

			return jsonResponse(t, http.StatusCreated, sampleSubmission())
		case "/v1/me/submissions":
			if request.URL.Query().Get("task_id") != "task-id" || request.URL.Query().Get("limit") != "10" {
				t.Fatalf("unexpected history query: %v", request.URL.Query())
			}

			return jsonResponse(t, http.StatusOK, SubmissionList{Items: []Submission{sampleSubmission()}, Limit: 10})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)

			return nil
		}
	})
	actor := Actor{UserID: "user-id", Roles: []string{"student"}}
	result, err := client.Submit(context.Background(), "task-id", SubmissionInput{
		TaskVersionID:     "version-id",
		IdempotencyKey:    "key-id",
		SelectedOptionIDs: []string{"option-id"},
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Correct || !result.TaskUpdated || result.LatestVersionNumber != 2 {
		t.Fatalf("unexpected submission: %+v", result)
	}
	taskID := "task-id"
	history, err := client.ListMySubmissions(context.Background(), &taskID, 10, 0, actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Items) != 1 || requests != 2 {
		t.Fatalf("unexpected history: %+v", history)
	}
}

// TestClientMapsCodeSubmission проверяет multipart upload и чтение результата.
func TestClientMapsCodeSubmission(t *testing.T) {
	t.Parallel()

	requests := 0
	client := testClient(t, func(request *http.Request) *http.Response {
		requests++

		switch request.URL.Path {
		case "/v1/tasks/task-id/code-submissions":
			if request.Method != http.MethodPost {
				t.Fatalf("unexpected submit method: %s", request.Method)
			}
			if err := request.ParseMultipartForm(512 << 10); err != nil {
				t.Fatal(err)
			}
			if request.FormValue("language") != "python" || request.FormValue("idempotency_key") != "key-id" {
				t.Fatalf("unexpected multipart fields: %v", request.MultipartForm.Value)
			}

			file, header, err := request.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = file.Close()
			}()

			source, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			if header.Filename != "solution.py" || string(source) != "print(42)" {
				t.Fatalf("unexpected source file: name=%s body=%q", header.Filename, source)
			}

			return jsonResponse(t, http.StatusAccepted, sampleCodeSubmission())
		case "/v1/code-submissions/code-submission-id":
			return jsonResponse(t, http.StatusOK, sampleCodeSubmission())
		case "/v1/me/code-submissions":
			if request.URL.Query().Get("task_id") != "task-id" || request.URL.Query().Get("limit") != "5" {
				t.Fatalf("unexpected code history query: %v", request.URL.Query())
			}

			return jsonResponse(t, http.StatusOK, CodeSubmissionList{
				Items: []CodeSubmission{sampleCodeSubmission()},
				Limit: 5,
			})
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)

			return nil
		}
	})

	actor := Actor{UserID: "user-id", Roles: []string{"student"}}
	created, err := client.SubmitCode(context.Background(), "task-id", CodeSubmissionInput{
		TaskVersionID:  "version-id",
		IdempotencyKey: "key-id",
		Language:       "python",
		FileName:       "../solution.py",
		File:           bytes.NewReader([]byte("print(42)")),
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != "queued" || created.SourceFileName != "solution.py" {
		t.Fatalf("unexpected created submission: %+v", created)
	}

	if _, err := client.GetCodeSubmission(context.Background(), "code-submission-id", actor); err != nil {
		t.Fatal(err)
	}

	taskID := "task-id"
	history, err := client.ListMyCodeSubmissions(context.Background(), &taskID, 5, 0, actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.Items) != 1 || requests != 3 {
		t.Fatalf("unexpected code history: %+v", history)
	}
}

// TestCodeSubmissionBodyRejectsLargeFile проверяет лимит до сетевого запроса.
func TestCodeSubmissionBodyRejectsLargeFile(t *testing.T) {
	t.Parallel()

	_, _, err := codeSubmissionBody(CodeSubmissionInput{
		TaskVersionID:  "version-id",
		IdempotencyKey: "key-id",
		Language:       "python",
		FileName:       "solution.py",
		File:           bytes.NewReader(bytes.Repeat([]byte("x"), maxCodeSourceSize+1)),
	})

	var upstream *Error
	if !errors.As(err, &upstream) || upstream.Code != "INVALID_SOURCE_FILE" {
		t.Fatalf("unexpected oversized file error: %v", err)
	}
}

// TestClientMapsUpstreamError проверяет сохранение технического error code.
func TestClientMapsUpstreamError(t *testing.T) {
	t.Parallel()

	client := testClient(t, func(_ *http.Request) *http.Response {
		return jsonResponse(t, http.StatusConflict, Error{
			Code:    "IDEMPOTENCY_KEY_CONFLICT",
			Message: "conflict",
		})
	})
	_, err := client.Submit(context.Background(), "task-id", SubmissionInput{}, Actor{UserID: "user-id"})
	upstream, ok := err.(*Error)
	if !ok || upstream.Code != "IDEMPOTENCY_KEY_CONFLICT" || upstream.StatusCode != http.StatusConflict {
		t.Fatalf("unexpected error: %#v", err)
	}
}

// TestClientHandlesSoftDelete проверяет ответ 204 без JSON-тела.
func TestClientHandlesSoftDelete(t *testing.T) {
	t.Parallel()

	client := testClient(t, func(request *http.Request) *http.Response {
		if request.Method != http.MethodDelete || request.URL.Path != "/v1/admin/tasks/task-id" {
			t.Fatalf("unexpected delete request: %s %s", request.Method, request.URL.Path)
		}

		return &http.Response{
			StatusCode: http.StatusNoContent,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}
	})
	if err := client.Delete(context.Background(), "task-id", Actor{UserID: "admin-id", Roles: []string{"admin"}}); err != nil {
		t.Fatal(err)
	}
}

// TestClientUsesAllReadAndUpdateRoutes проверяет оставшиеся основные маршруты.
func TestClientUsesAllReadAndUpdateRoutes(t *testing.T) {
	t.Parallel()

	seen := map[string]int{}
	client := testClient(t, func(request *http.Request) *http.Response {
		key := request.Method + " " + request.URL.Path
		seen[key]++
		switch key {
		case "GET /v1/tasks":
			if request.Header.Get("X-User-ID") != "" || request.URL.Query().Get("status") != "" {
				t.Fatalf("public list contains protected data: %v", request.Header)
			}

			return jsonResponse(t, http.StatusOK, TaskList{})
		case "GET /v1/tasks/task-id", "GET /v1/admin/tasks/task-id":
			return jsonResponse(t, http.StatusOK, sampleTask())
		case "PUT /v1/admin/tasks/task-id":
			return jsonResponse(t, http.StatusOK, sampleTask())
		case "GET /v1/submissions/submission-id":
			return jsonResponse(t, http.StatusOK, sampleSubmission())
		case "GET /ready":
			return jsonResponse(t, http.StatusOK, map[string]string{"status": "ready"})
		default:
			t.Fatalf("unexpected route: %s", key)

			return nil
		}
	})
	actor := Actor{UserID: "admin-id", Roles: []string{"admin"}}
	if _, err := client.ListPublished(context.Background(), TaskFilter{Status: "draft", Limit: 20}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetPublished(context.Background(), "task-id"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetAdmin(context.Background(), "task-id", actor); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Update(context.Background(), "task-id", TaskInput{}, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSubmission(context.Background(), "submission-id", actor); err != nil {
		t.Fatal(err)
	}
	if err := client.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 6 {
		t.Fatalf("not all routes were called: %v", seen)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip выполняет тестовый HTTP-запрос без listener.
func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

// testClient создаёт клиент с управляемым transport.
func testClient(t *testing.T, handler func(*http.Request) *http.Response) *Client {
	t.Helper()

	client := New("http://tasks-it.local", time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	client.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return handler(request), nil
	})

	return client
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

// sampleCodeSubmission возвращает поставленное в очередь программное решение.
func sampleCodeSubmission() CodeSubmission {
	return CodeSubmission{
		ID:                "code-submission-id",
		UserID:            "user-id",
		TaskID:            "task-id",
		TaskVersionID:     "version-id",
		TaskVersionNumber: 1,
		ExecutionID:       "execution-id",
		CorrelationID:     "correlation-id",
		Language:          "python",
		SourceFileName:    "solution.py",
		Status:            "queued",
		Tests:             []ExecutionTestResult{},
	}
}

// sampleTask возвращает полную тестовую задачу.
func sampleTask() Task {
	correct := true

	return Task{
		ID:            "task-id",
		Status:        "draft",
		TaskVersionID: "version-id",
		VersionNumber: 1,
		Title:         "Sets",
		Statement:     "Select all",
		TaskType:      "multiple_choice",
		Difficulty:    "medium",
		Options: []TaskOption{{
			ID:        "option-id",
			Text:      "A",
			Position:  0,
			IsCorrect: &correct,
		}},
	}
}

// sampleSubmission возвращает сохранённый тестовый результат.
func sampleSubmission() Submission {
	return Submission{
		ID:                  "submission-id",
		UserID:              "user-id",
		TaskID:              "task-id",
		TaskVersionID:       "version-id",
		TaskVersionNumber:   1,
		SelectedOptionIDs:   []string{"option-id"},
		CorrectOptionIDs:    []string{"option-id"},
		Correct:             true,
		Verdict:             "accepted",
		TaskUpdated:         true,
		LatestTaskVersionID: "version-id-2",
		LatestVersionNumber: 2,
	}
}

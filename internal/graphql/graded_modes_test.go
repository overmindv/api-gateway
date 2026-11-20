package graphql

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/tasks"
)

func TestGradedProjectionPrivacy(t *testing.T) {
	t.Parallel()

	raw := `{"run_mode":"graded","status":"completed","verdict":"accepted","suite_reference":{"id":"PRIVATE_CANARY"},"source_code":"PRIVATE_CANARY","tests":[{"test_id":"hidden-PRIVATE_CANARY","verdict":"accepted","stdout":"PRIVATE_CANARY","stderr":"PRIVATE_CANARY"}],"execution":{"stdout":"PRIVATE_CANARY","stderr":"PRIVATE_CANARY","duration_ms":12},"compilation":{"stdout":"PRIVATE_CANARY","stderr":"PRIVATE_CANARY"},"failure":{"code":"PRIVATE_CANARY","message":"PRIVATE_CANARY"}}`
	var upstream tasks.CodeSubmission
	if err := json.Unmarshal([]byte(raw), &upstream); err != nil {
		t.Fatal(err)
	}
	result := codeSubmissionModel(upstream)
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), testPrivateCanary) || len(result.Tests) != 0 || result.Execution.DurationMs != 12 {
		t.Fatalf("private projection leaked: %s", encoded)
	}
}

func TestGraphQLRunModeContract(t *testing.T) {
	t.Parallel()

	mode := ""
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-User-ID") != testUserID {
			t.Error("verified actor missing")
		}
		if r.Method == http.MethodPost {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			defer func() { _ = r.MultipartForm.RemoveAll() }()
			mode = r.FormValue("run_mode")
			if r.FormValue("source_code") != "print(1)" {
				t.Error("source lost")
			}
		}
		item := sampleUpstreamCodeSubmission()
		item.RunMode = mode
		if mode == runModeGraded {
			item.Tests[0].TestID = "hidden-1"
			item.Tests[0].Stdout = testPrivateCanary
			item.Execution.Stdout = testPrivateCanary
			item.Failure = &tasks.ExecutionFailure{Code: testPrivateCanary, Message: testPrivateCanary}
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/me/code-submissions") {
			_ = json.NewEncoder(w).Encode(tasks.CodeSubmissionList{Items: []tasks.CodeSubmission{item}})
		} else {
			_ = json.NewEncoder(w).Encode(item)
		}
	}))
	defer upstream.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := tasks.New(upstream.URL, time.Second, logger)
	handler := Handler(nil, nil, client, logger)
	send := func(query string) string {
		t.Helper()
		raw, _ := json.Marshal(map[string]string{"query": query})
		r := httptest.NewRequest("POST", "/graphql", bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r = r.WithContext(tasksContext(testUserID, []string{"student"}))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

		return w.Body.String()
	}
	selection := `id runMode status executionId verdict tests{testId stdout stderr} execution{stdout stderr durationMs} failure{code message}`
	for _, requested := range []string{"", "samples", runModeGraded} {
		field := ""
		if requested != "" {
			field = ",runMode:" + requested
		}
		result := send(`mutation{submitITTaskCode(taskId:"task-id",input:{taskVersionId:"version-id",idempotencyKey:"key-id",language:python,sourceCode:"print(1)"` + field + `}){` + selection + `}}`)
		expected := requested
		if expected == "" {
			expected = "samples"
		}
		if mode != expected || strings.Contains(result, `"errors"`) || !strings.Contains(result, `"runMode":"`+expected+`"`) {
			t.Fatalf("mode contract: %s", result)
		}
		if requested == "graded" && (strings.Contains(result, "PRIVATE_CANARY") || !strings.Contains(result, `"tests":[]`)) {
			t.Fatal("mutation leaked graded payload")
		}
	}
	for _, query := range []string{`{itCodeSubmission(id:"code-id"){` + selection + `}}`, `{myITCodeSubmissions{items{` + selection + `}}}`} {
		result := send(query)
		if strings.Contains(result, `"errors"`) || strings.Contains(result, "PRIVATE_CANARY") || !strings.Contains(result, `"runMode":"graded"`) || !strings.Contains(result, `"tests":[]`) {
			t.Fatalf("read contract: %s", result)
		}
	}
	before := calls
	result := send(`mutation{submitITTaskCode(taskId:"task-id",input:{taskVersionId:"version-id",idempotencyKey:"key-id",language:python,sourceCode:"print(1)",runMode:unknown}){id}}`)
	if calls != before || !strings.Contains(result, `"errors"`) {
		t.Fatal("invalid mode reached tasks")
	}
}

func TestProjectionFailsClosed(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"samples", "", "future-mode"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			item := sampleUpstreamCodeSubmission()
			item.RunMode = mode
			for _, id := range []string{"hidden-1", "open-0", "open-21", "open-01", "open-+1", "open-PRIVATE_CANARY"} {
				item.Tests = append(item.Tests, tasks.ExecutionTestResult{TestID: id, Verdict: verdictAccepted, Stdout: testPrivateCanary})
			}
			result := codeSubmissionModel(item)
			raw, _ := json.Marshal(result)
			if strings.Contains(string(raw), testPrivateCanary) {
				t.Fatal("unrecognized case leaked")
			}
			want := 1
			if mode == "future-mode" {
				want = 0
			}
			if len(result.Tests) != want {
				t.Fatal("unexpected case projection")
			}
		})
	}
}

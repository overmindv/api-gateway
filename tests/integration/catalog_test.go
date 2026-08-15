package integration

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/users"
	"github.com/overmindv/api-gateway/internal/config"
	graphqldelivery "github.com/overmindv/api-gateway/internal/graphql"
	"github.com/overmindv/api-gateway/internal/middleware"
)

// fakeEntitiesCatalog эмулирует HTTP API Entities и проверяет, что gateway передаёт actor и request_id.
type fakeEntitiesCatalog struct {
	t        *testing.T
	requests []fakeEntitiesRequest
}

// fakeEntitiesRequest хранит важные поля внутреннего запроса для проверок интеграционного сценария.
type fakeEntitiesRequest struct {
	Method    string
	Path      string
	RequestID string
	UserID    string
	Roles     string
}

// ServeHTTP обрабатывает минимальный набор endpoints каталога для полного GraphQL flow.
func (f *fakeEntitiesCatalog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.requests = append(f.requests, fakeEntitiesRequest{
		Method:    r.Method,
		Path:      r.URL.Path,
		RequestID: r.Header.Get(middleware.RequestIDHeader),
		UserID:    r.Header.Get("X-User-ID"),
		Roles:     r.Header.Get("X-User-Roles"),
	})
	if !strings.Contains(r.Header.Get("X-User-Roles"), "admin") || r.Header.Get("X-User-ID") == "" {
		writeEntitiesJSON(w, http.StatusForbidden, map[string]string{"code": "PERMISSION_DENIED", "message": "операция доступна только администратору"})
		return
	}

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/universities":
		var input entities.University
		f.decode(r, &input)
		input.ID = "33333333-3333-3333-3333-333333333333"
		input.Status = "draft"
		input.CreatedAt = "2026-07-18T10:10:00Z"
		input.UpdatedAt = input.CreatedAt
		writeEntitiesJSON(w, http.StatusCreated, input)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/programs":
		var input entities.Program
		f.decode(r, &input)
		input.ID = "44444444-4444-4444-4444-444444444444"
		input.DegreeLevel = "other"
		input.Status = "draft"
		input.CreatedAt = "2026-07-18T10:11:00Z"
		input.UpdatedAt = input.CreatedAt
		writeEntitiesJSON(w, http.StatusCreated, input)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/courses":
		var input entities.Course
		f.decode(r, &input)
		input.ID = "55555555-5555-5555-5555-555555555555"
		input.Slug = "algorithms"
		input.Status = "draft"
		input.CreatedAt = "2026-07-18T10:12:00Z"
		input.UpdatedAt = input.CreatedAt
		writeEntitiesJSON(w, http.StatusCreated, input)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/topics":
		var input entities.Topic
		f.decode(r, &input)
		if input.Title == "Intro" {
			input.ID = "66666666-6666-6666-6666-666666666666"
		} else {
			input.ID = "77777777-7777-7777-7777-777777777777"
		}
		input.Slug = strings.ToLower(input.Title)
		input.Difficulty = "basic"
		input.Status = "draft"
		input.CreatedAt = "2026-07-18T10:13:00Z"
		input.UpdatedAt = input.CreatedAt
		writeEntitiesJSON(w, http.StatusCreated, input)
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prerequisites"):
		var input struct {
			PrerequisiteTopicID string `json:"prerequisite_topic_id"`
		}
		f.decode(r, &input)
		topicID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/topics/"), "/prerequisites")
		writeEntitiesJSON(w, http.StatusCreated, entities.TopicPrerequisite{
			TopicID:             topicID,
			PrerequisiteTopicID: input.PrerequisiteTopicID,
			CreatedAt:           "2026-07-18T10:14:00Z",
		})
	default:
		writeEntitiesJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND", "message": "endpoint не найден"})
	}
}

// decode читает JSON body fake Entities и падает тестом при невалидном payload.
func (f *fakeEntitiesCatalog) decode(r *http.Request, target any) {
	f.t.Helper()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		f.t.Fatalf("decode entities request: %v", err)
	}
}

// TestGatewayFullCatalogFlow повторяет пользовательскую цепочку: регистрация, назначение админом и создание каталога.
func TestGatewayFullCatalogFlow(t *testing.T) {
	usersUpstream := newFakeUsersGraphQL()
	usersServer := httptest.NewServer(usersUpstream)
	defer usersServer.Close()

	entitiesUpstream := &fakeEntitiesCatalog{t: t}
	entitiesServer := httptest.NewServer(entitiesUpstream)
	defer entitiesServer.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	users := users.New(config.Users{
		GraphQLURL: usersServer.URL,
		HealthURL:  usersServer.URL,
		Timeout:    time.Second,
	}, logger)
	catalog := entities.New(entitiesServer.URL, time.Second, logger)
	authenticator := middleware.NewJWTAuthenticator(usersUpstream.secret, usersUpstream.issuer)
	handler := graphqldelivery.Handler(users, catalog, nil, logger)
	handler = middleware.JWT(authenticator, handler)
	handler = middleware.RequestIDMiddleware(handler)

	registered := gatewayRegister(t, handler)
	superuser := gatewayLogin(t, handler, "admin@example.com", "password")
	gatewaySetAdminByUsername(t, handler, superuser.Token, registered.User.Username, true)
	admin := gatewayLogin(t, handler, registered.User.Email, "password")

	university := gatewayCreateUniversity(t, handler, admin.Token)
	program := gatewayCreateProgram(t, handler, admin.Token, university.ID)
	course := gatewayCreateCourse(t, handler, admin.Token, program.ID)
	intro := gatewayCreateTopic(t, handler, admin.Token, course.ID, nil, "Intro")
	next := gatewayCreateTopic(t, handler, admin.Token, course.ID, &intro.ID, "Practice")
	prerequisite := gatewayAddPrerequisite(t, handler, admin.Token, next.ID, intro.ID)

	if program.UniversityID == nil || *program.UniversityID != university.ID {
		t.Fatalf("program is not attached to university: %+v", program)
	}
	if course.ProgramID == nil || *course.ProgramID != program.ID {
		t.Fatalf("course is not attached to program: %+v", course)
	}
	if next.CourseID == nil || *next.CourseID != course.ID || next.ParentTopicID == nil || *next.ParentTopicID != intro.ID {
		t.Fatalf("topic is not attached to course/parent: %+v", next)
	}
	if prerequisite.TopicID != next.ID || prerequisite.PrerequisiteTopicID != intro.ID {
		t.Fatalf("prerequisite is not attached correctly: %+v", prerequisite)
	}
	for _, request := range entitiesUpstream.requests {
		if request.RequestID == "" || request.UserID != fakeStudentID || !strings.Contains(request.Roles, "admin") {
			t.Fatalf("gateway did not pass actor/request metadata to entities: %+v", request)
		}
	}
}

type catalogUniversityPayload struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type catalogProgramPayload struct {
	ID           string  `json:"id"`
	UniversityID *string `json:"universityId"`
	Name         string  `json:"name"`
}

type catalogCoursePayload struct {
	ID        string  `json:"id"`
	ProgramID *string `json:"programId"`
	Name      string  `json:"name"`
}

type catalogTopicPayload struct {
	ID            string  `json:"id"`
	CourseID      *string `json:"courseId"`
	ParentTopicID *string `json:"parentTopicId"`
	Title         string  `json:"title"`
}

type catalogPrerequisitePayload struct {
	TopicID             string `json:"topicId"`
	PrerequisiteTopicID string `json:"prerequisiteTopicId"`
}

// gatewayCreateUniversity создаёт университет через GraphQL API gateway.
func gatewayCreateUniversity(t *testing.T, handler http.Handler, token string) catalogUniversityPayload {
	t.Helper()
	response := gatewayPostGraphQL(t, handler, token, `mutation CreateUniversity($input: CreateUniversityInput!) {
		createUniversity(input: $input) { id name }
	}`, map[string]any{"input": map[string]any{"name": "Test University", "logoFileId": ""}})
	if len(response.Errors) > 0 {
		t.Fatalf("create university errors: %+v", response.Errors)
	}
	var data struct {
		CreateUniversity catalogUniversityPayload `json:"createUniversity"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.CreateUniversity
}

// gatewayCreateProgram создаёт программу через GraphQL API gateway.
func gatewayCreateProgram(t *testing.T, handler http.Handler, token, universityID string) catalogProgramPayload {
	t.Helper()
	response := gatewayPostGraphQL(t, handler, token, `mutation CreateProgram($input: CreateProgramInput!) {
		createProgram(input: $input) { id universityId name }
	}`, map[string]any{"input": map[string]any{"universityId": universityID, "name": "Computer Science"}})
	if len(response.Errors) > 0 {
		t.Fatalf("create program errors: %+v", response.Errors)
	}
	var data struct {
		CreateProgram catalogProgramPayload `json:"createProgram"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.CreateProgram
}

// gatewayCreateCourse создаёт курс через GraphQL API gateway.
func gatewayCreateCourse(t *testing.T, handler http.Handler, token, programID string) catalogCoursePayload {
	t.Helper()
	response := gatewayPostGraphQL(t, handler, token, `mutation CreateCourse($input: CreateCourseInput!) {
		createCourse(input: $input) { id programId name }
	}`, map[string]any{"input": map[string]any{"programId": programID, "name": "Algorithms"}})
	if len(response.Errors) > 0 {
		t.Fatalf("create course errors: %+v", response.Errors)
	}
	var data struct {
		CreateCourse catalogCoursePayload `json:"createCourse"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.CreateCourse
}

// gatewayCreateTopic создаёт тему курса через GraphQL API gateway.
func gatewayCreateTopic(t *testing.T, handler http.Handler, token, courseID string, parentTopicID *string, title string) catalogTopicPayload {
	t.Helper()
	input := map[string]any{"courseId": courseID, "title": title}
	if parentTopicID != nil {
		input["parentTopicId"] = *parentTopicID
	}
	response := gatewayPostGraphQL(t, handler, token, `mutation CreateTopic($input: CreateTopicInput!) {
		createTopic(input: $input) { id courseId parentTopicId title }
	}`, map[string]any{"input": input})
	if len(response.Errors) > 0 {
		t.Fatalf("create topic errors: %+v", response.Errors)
	}
	var data struct {
		CreateTopic catalogTopicPayload `json:"createTopic"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.CreateTopic
}

// gatewayAddPrerequisite создаёт связь prerequisite через GraphQL API gateway.
func gatewayAddPrerequisite(t *testing.T, handler http.Handler, token, topicID, prerequisiteTopicID string) catalogPrerequisitePayload {
	t.Helper()
	response := gatewayPostGraphQL(t, handler, token, `mutation AddTopicPrerequisite($input: TopicPrerequisiteInput!) {
		addTopicPrerequisite(input: $input) { topicId prerequisiteTopicId }
	}`, map[string]any{"input": map[string]any{"topicId": topicID, "prerequisiteTopicId": prerequisiteTopicID}})
	if len(response.Errors) > 0 {
		t.Fatalf("add prerequisite errors: %+v", response.Errors)
	}
	var data struct {
		AddTopicPrerequisite catalogPrerequisitePayload `json:"addTopicPrerequisite"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.AddTopicPrerequisite
}

// writeEntitiesJSON пишет JSON response из fake Entities.
func writeEntitiesJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

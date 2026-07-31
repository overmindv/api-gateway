package integration

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

	"github.com/golang-jwt/jwt/v5"
	"github.com/overmindv/laserbeak/internal/client/arcee"
	"github.com/overmindv/laserbeak/internal/config"
	graphqldelivery "github.com/overmindv/laserbeak/internal/graphql"
	"github.com/overmindv/laserbeak/internal/middleware"
)

type gatewayUser struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Username    string   `json:"username"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	BirthDate   *string  `json:"birthDate"`
	Phone       *string  `json:"phone"`
	Roles       []string `json:"roles"`
	IsAdmin     bool     `json:"isAdmin"`
	IsSuperuser bool     `json:"isSuperuser"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
	Password    string   `json:"-"`
}

type fakeArceeGraphQL struct {
	secret string
	issuer string
	users  map[string]*gatewayUser
}

const (
	fakeAdminID   = "11111111-1111-1111-1111-111111111111"
	fakeStudentID = "22222222-2222-2222-2222-222222222222"
)

func newFakeArceeGraphQL() *fakeArceeGraphQL {
	return &fakeArceeGraphQL{
		secret: "gateway-test-secret",
		issuer: "arcee",
		users: map[string]*gatewayUser{
			"admin@example.com": {
				ID:          fakeAdminID,
				Email:       "admin@example.com",
				Username:    "superadmin",
				Roles:       []string{"admin"},
				IsAdmin:     true,
				IsSuperuser: true,
				CreatedAt:   "2026-07-18T10:00:00Z",
				UpdatedAt:   "2026-07-18T10:00:00Z",
				Password:    "password",
			},
		},
	}
}

func (f *fakeArceeGraphQL) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(request.Query, "register(input:"):
		f.register(w, request.Variables)
	case strings.Contains(request.Query, "login(input:"):
		f.login(w, request.Variables)
	case strings.Contains(request.Query, "updateUser(id:"):
		if !f.hasBearer(r) {
			writeGraphQLError(w, "UNAUTHENTICATED")
			return
		}
		f.updateUser(w, request.Variables)
	case strings.Contains(request.Query, "setUserAdminByUsername(username:"):
		if !f.hasBearer(r) {
			writeGraphQLError(w, "UNAUTHENTICATED")
			return
		}
		f.setUserAdminByUsername(w, request.Variables)
	default:
		writeGraphQLError(w, "NOT_FOUND")
	}
}

func (f *fakeArceeGraphQL) register(w http.ResponseWriter, variables map[string]any) {
	input := variables["input"].(map[string]any)
	email := input["email"].(string)
	user := &gatewayUser{
		ID:        fakeStudentID,
		Email:     email,
		Username:  input["username"].(string),
		FirstName: stringValueFromMap(input, "firstName"),
		LastName:  stringValueFromMap(input, "lastName"),
		Roles:     []string{},
		CreatedAt: "2026-07-18T10:00:00Z",
		UpdatedAt: "2026-07-18T10:00:00Z",
		Password:  input["password"].(string),
	}
	f.users[email] = user
	writeGraphQLData(w, map[string]any{
		"register": map[string]any{
			"token":     f.token(user),
			"expiresAt": "2026-07-18T11:00:00Z",
			"user":      user,
		},
	})
}

func (f *fakeArceeGraphQL) login(w http.ResponseWriter, variables map[string]any) {
	input := variables["input"].(map[string]any)
	user := f.users[input["email"].(string)]
	if user == nil || user.Password != input["password"].(string) {
		writeGraphQLError(w, "UNAUTHENTICATED")
		return
	}
	writeGraphQLData(w, map[string]any{
		"login": map[string]any{
			"token":     f.token(user),
			"expiresAt": "2026-07-18T11:00:00Z",
			"user":      user,
		},
	})
}

func (f *fakeArceeGraphQL) updateUser(w http.ResponseWriter, variables map[string]any) {
	input := variables["input"].(map[string]any)
	for _, user := range f.users {
		if user.ID == variables["id"].(string) {
			user.FirstName = stringValueFromMap(input, "firstName")
			user.UpdatedAt = "2026-07-18T10:05:00Z"
			writeGraphQLData(w, map[string]any{"updateUser": user})
			return
		}
	}
	writeGraphQLError(w, "NOT_FOUND")
}

func (f *fakeArceeGraphQL) setUserAdminByUsername(w http.ResponseWriter, variables map[string]any) {
	username := variables["username"].(string)
	admin := variables["admin"].(bool)
	for _, user := range f.users {
		if user.Username == username {
			user.IsAdmin = admin
			if admin {
				user.Roles = []string{"admin"}
			} else {
				user.Roles = []string{}
			}
			writeGraphQLData(w, map[string]any{"setUserAdminByUsername": user})
			return
		}
	}
	writeGraphQLError(w, "NOT_FOUND")
}

func (f *fakeArceeGraphQL) token(user *gatewayUser) string {
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   user.ID,
		"iss":   f.issuer,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
		"roles": user.Roles,
	}).SignedString([]byte(f.secret))
	if err != nil {
		panic(err)
	}

	return token
}

func (f *fakeArceeGraphQL) hasBearer(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func TestGatewayAuthFlowThroughArceeClient(t *testing.T) {
	upstream := newFakeArceeGraphQL()
	upstreamServer := httptest.NewServer(upstream)
	defer upstreamServer.Close()

	users := arcee.New(config.Arcee{
		GraphQLURL: upstreamServer.URL,
		HealthURL:  upstreamServer.URL,
		Timeout:    time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	authenticator := middleware.NewJWTAuthenticator(upstream.secret, upstream.issuer)
	handler := middleware.JWT(
		authenticator,
		graphqldelivery.Handler(users, nil, slog.New(slog.NewTextHandler(io.Discard, nil))),
	)

	registered := gatewayRegister(t, handler)
	if registered.User.ID != fakeStudentID || registered.User.IsAdmin {
		t.Fatalf("unexpected registered user: %+v", registered.User)
	}

	loggedIn := gatewayLogin(t, handler, "student@example.com", "password")
	if loggedIn.User.ID != registered.User.ID {
		t.Fatalf("login returned a different user: %+v", loggedIn.User)
	}

	updated := gatewayUpdateUser(t, handler, registered.Token, registered.User.ID)
	if updated.FirstName != "Updated" {
		t.Fatalf("gateway update did not reach arcee: %+v", updated)
	}

	admin := gatewayLogin(t, handler, "admin@example.com", "password")
	promoted := gatewaySetAdminByUsername(t, handler, admin.Token, "student", true)
	if !promoted.IsAdmin {
		t.Fatalf("gateway admin promotion failed: %+v", promoted)
	}

	response := gatewayPostGraphQL(t, handler, registered.Token, `mutation SetUserAdminByUsername($username: String!, $admin: Boolean!) {
		setUserAdminByUsername(username: $username, admin: $admin) { id }
	}`, map[string]any{"username": "superadmin", "admin": true})
	if len(response.Errors) == 0 || response.Errors[0].Extensions["code"] != "PERMISSION_DENIED" {
		t.Fatalf("expected gateway to block regular admin mutation, got %+v", response.Errors)
	}
}

type gatewayAuthPayload struct {
	Token string      `json:"token"`
	User  gatewayUser `json:"user"`
}

func gatewayRegister(t *testing.T, handler http.Handler) gatewayAuthPayload {
	t.Helper()

	response := gatewayPostGraphQL(t, handler, "", `mutation Register($input: RegisterInput!) {
		register(input: $input) { token user { id email username firstName lastName roles isAdmin isSuperuser createdAt updatedAt } }
	}`, map[string]any{"input": map[string]any{
		"email":     "student@example.com",
		"password":  "password",
		"username":  "student",
		"firstName": "Student",
		"lastName":  "",
		"phone":     "",
	}})
	if len(response.Errors) > 0 {
		t.Fatalf("register errors: %+v", response.Errors)
	}

	var data struct {
		Register gatewayAuthPayload `json:"register"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.Register
}

func gatewayLogin(t *testing.T, handler http.Handler, email, password string) gatewayAuthPayload {
	t.Helper()

	response := gatewayPostGraphQL(t, handler, "", `mutation Login($input: LoginInput!) {
		login(input: $input) { token user { id email username roles isAdmin isSuperuser createdAt updatedAt } }
	}`, map[string]any{"input": map[string]any{"email": email, "password": password}})
	if len(response.Errors) > 0 {
		t.Fatalf("login errors: %+v", response.Errors)
	}

	var data struct {
		Login gatewayAuthPayload `json:"login"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.Login
}

func gatewayUpdateUser(t *testing.T, handler http.Handler, token, id string) gatewayUser {
	t.Helper()

	response := gatewayPostGraphQL(t, handler, token, `mutation UpdateUser($id: ID!, $input: UpdateUserInput!) {
		updateUser(id: $id, input: $input) { id email username firstName lastName roles isAdmin isSuperuser createdAt updatedAt }
	}`, map[string]any{"id": id, "input": map[string]any{"firstName": "Updated"}})
	if len(response.Errors) > 0 {
		t.Fatalf("update errors: %+v", response.Errors)
	}

	var data struct {
		UpdateUser gatewayUser `json:"updateUser"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.UpdateUser
}

func gatewaySetAdminByUsername(t *testing.T, handler http.Handler, token, username string, admin bool) gatewayUser {
	t.Helper()

	response := gatewayPostGraphQL(t, handler, token, `mutation SetUserAdminByUsername($username: String!, $admin: Boolean!) {
		setUserAdminByUsername(username: $username, admin: $admin) { id email username firstName lastName roles isAdmin isSuperuser createdAt updatedAt }
	}`, map[string]any{"username": username, "admin": admin})
	if len(response.Errors) > 0 {
		t.Fatalf("set admin errors: %+v", response.Errors)
	}

	var data struct {
		SetUserAdminByUsername gatewayUser `json:"setUserAdminByUsername"`
	}
	decodeGatewayData(t, response.Data, &data)

	return data.SetUserAdminByUsername
}

type gatewayGraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func gatewayPostGraphQL(t *testing.T, handler http.Handler, token, query string, variables map[string]any) gatewayGraphQLResponse {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected HTTP status %d: %s", recorder.Code, recorder.Body.String())
	}

	var response gatewayGraphQLResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	return response
}

func decodeGatewayData(t *testing.T, data json.RawMessage, target any) {
	t.Helper()

	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}

func writeGraphQLData(w http.ResponseWriter, data map[string]any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func writeGraphQLError(w http.ResponseWriter, code string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]any{{
			"message":    "upstream error",
			"extensions": map[string]any{"code": code},
		}},
		"data": nil,
	})
}

func stringValueFromMap(values map[string]any, key string) string {
	value, _ := values[key].(string)

	return value
}

package entities

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/overmindv/api-gateway/internal/client/httpx"
)

type CatalogService interface {
	ListUniversities(context.Context, ListOptions) ([]University, error)
	GetUniversity(context.Context, string) (University, error)
	CreateUniversity(context.Context, University, Actor) (University, error)
	UpdateUniversity(context.Context, string, University, Actor) (University, error)
	DeleteUniversity(context.Context, string, Actor) error
	ChangeUniversityStatus(context.Context, string, string, Actor) (University, error)
	ListPrograms(context.Context, string, ListOptions) ([]Program, error)
	GetProgram(context.Context, string) (Program, error)
	CreateProgram(context.Context, Program, Actor) (Program, error)
	UpdateProgram(context.Context, string, Program, Actor) (Program, error)
	DeleteProgram(context.Context, string, Actor) error
	ChangeProgramStatus(context.Context, string, string, Actor) (Program, error)
	ListCourses(context.Context, string, ListOptions) ([]Course, error)
	GetCourse(context.Context, string) (Course, error)
	CreateCourse(context.Context, Course, Actor) (Course, error)
	UpdateCourse(context.Context, string, Course, Actor) (Course, error)
	DeleteCourse(context.Context, string, Actor) error
	ChangeCourseStatus(context.Context, string, string, Actor) (Course, error)
	ListTopics(context.Context, string, ListOptions) ([]Topic, error)
	GetTopic(context.Context, string) (Topic, error)
	CreateTopic(context.Context, Topic, Actor) (Topic, error)
	UpdateTopic(context.Context, string, Topic, Actor) (Topic, error)
	DeleteTopic(context.Context, string, Actor) error
	ChangeTopicStatus(context.Context, string, string, Actor) (Topic, error)
	TopicTree(context.Context, string) ([]*TopicTreeNode, error)
	ListPrerequisites(context.Context, string) ([]TopicPrerequisite, error)
	AddPrerequisite(context.Context, string, string, Actor) (TopicPrerequisite, error)
	RemovePrerequisite(context.Context, string, string, Actor) error
	ValidateBinding(context.Context, Binding) (ValidationResult, error)
	Health(context.Context) error
}

type Client struct {
	http *http.Client
	hx   *httpx.Client
}

func New(baseURL string, timeout time.Duration, log *slog.Logger) *Client {
	httpClient := &http.Client{Timeout: timeout}

	return &Client{
		http: httpClient,
		hx: httpx.New("entities", baseURL, httpClient, log, func(_ int) error {
			return &Error{}
		}),
	}
}

func (c *Client) ListUniversities(ctx context.Context, options ListOptions) ([]University, error) {
	return request[[]University](c, ctx, http.MethodGet, "/v1/universities?"+query(options, "", ""), nil, Actor{})
}

func (c *Client) GetUniversity(ctx context.Context, id string) (University, error) {
	return request[University](c, ctx, http.MethodGet, "/v1/universities/"+url.PathEscape(id), nil, Actor{})
}

func (c *Client) CreateUniversity(ctx context.Context, input University, actor Actor) (University, error) {
	return request[University](c, ctx, http.MethodPost, "/v1/universities", input, actor)
}

func (c *Client) UpdateUniversity(ctx context.Context, id string, input University, actor Actor) (University, error) {
	return request[University](c, ctx, http.MethodPut, "/v1/universities/"+url.PathEscape(id), input, actor)
}

func (c *Client) DeleteUniversity(ctx context.Context, id string, actor Actor) error {
	_, err := request[map[string]bool](c, ctx, http.MethodDelete, "/v1/universities/"+url.PathEscape(id), nil, actor)

	return err
}

func (c *Client) ChangeUniversityStatus(ctx context.Context, id, status string, actor Actor) (University, error) {
	return request[University](c, ctx, http.MethodPatch, "/v1/universities/"+url.PathEscape(id)+"/status", map[string]string{"status": status}, actor)
}

func (c *Client) ListPrograms(ctx context.Context, universityID string, options ListOptions) ([]Program, error) {
	return request[[]Program](c, ctx, http.MethodGet, "/v1/programs?"+query(options, "university_id", universityID), nil, Actor{})
}

func (c *Client) GetProgram(ctx context.Context, id string) (Program, error) {
	return request[Program](c, ctx, http.MethodGet, "/v1/programs/"+url.PathEscape(id), nil, Actor{})
}

func (c *Client) CreateProgram(ctx context.Context, input Program, actor Actor) (Program, error) {
	return request[Program](c, ctx, http.MethodPost, "/v1/programs", input, actor)
}

func (c *Client) UpdateProgram(ctx context.Context, id string, input Program, actor Actor) (Program, error) {
	return request[Program](c, ctx, http.MethodPut, "/v1/programs/"+url.PathEscape(id), input, actor)
}

func (c *Client) DeleteProgram(ctx context.Context, id string, actor Actor) error {
	_, err := request[map[string]bool](c, ctx, http.MethodDelete, "/v1/programs/"+url.PathEscape(id), nil, actor)

	return err
}

func (c *Client) ChangeProgramStatus(ctx context.Context, id, status string, actor Actor) (Program, error) {
	return request[Program](c, ctx, http.MethodPatch, "/v1/programs/"+url.PathEscape(id)+"/status", map[string]string{"status": status}, actor)
}

func (c *Client) ListCourses(ctx context.Context, programID string, options ListOptions) ([]Course, error) {
	return request[[]Course](c, ctx, http.MethodGet, "/v1/courses?"+query(options, "program_id", programID), nil, Actor{})
}

func (c *Client) GetCourse(ctx context.Context, id string) (Course, error) {
	return request[Course](c, ctx, http.MethodGet, "/v1/courses/"+url.PathEscape(id), nil, Actor{})
}

func (c *Client) CreateCourse(ctx context.Context, input Course, actor Actor) (Course, error) {
	return request[Course](c, ctx, http.MethodPost, "/v1/courses", input, actor)
}

func (c *Client) UpdateCourse(ctx context.Context, id string, input Course, actor Actor) (Course, error) {
	return request[Course](c, ctx, http.MethodPut, "/v1/courses/"+url.PathEscape(id), input, actor)
}

func (c *Client) DeleteCourse(ctx context.Context, id string, actor Actor) error {
	_, err := request[map[string]bool](c, ctx, http.MethodDelete, "/v1/courses/"+url.PathEscape(id), nil, actor)

	return err
}

func (c *Client) ChangeCourseStatus(ctx context.Context, id, status string, actor Actor) (Course, error) {
	return request[Course](c, ctx, http.MethodPatch, "/v1/courses/"+url.PathEscape(id)+"/status", map[string]string{"status": status}, actor)
}

func (c *Client) ListTopics(ctx context.Context, courseID string, options ListOptions) ([]Topic, error) {
	return request[[]Topic](c, ctx, http.MethodGet, "/v1/topics?"+query(options, "course_id", courseID), nil, Actor{})
}

func (c *Client) GetTopic(ctx context.Context, id string) (Topic, error) {
	return request[Topic](c, ctx, http.MethodGet, "/v1/topics/"+url.PathEscape(id), nil, Actor{})
}

func (c *Client) CreateTopic(ctx context.Context, input Topic, actor Actor) (Topic, error) {
	return request[Topic](c, ctx, http.MethodPost, "/v1/topics", input, actor)
}

func (c *Client) UpdateTopic(ctx context.Context, id string, input Topic, actor Actor) (Topic, error) {
	return request[Topic](c, ctx, http.MethodPut, "/v1/topics/"+url.PathEscape(id), input, actor)
}

func (c *Client) DeleteTopic(ctx context.Context, id string, actor Actor) error {
	_, err := request[map[string]bool](c, ctx, http.MethodDelete, "/v1/topics/"+url.PathEscape(id), nil, actor)

	return err
}

func (c *Client) ChangeTopicStatus(ctx context.Context, id, status string, actor Actor) (Topic, error) {
	return request[Topic](c, ctx, http.MethodPatch, "/v1/topics/"+url.PathEscape(id)+"/status", map[string]string{"status": status}, actor)
}

func (c *Client) TopicTree(ctx context.Context, courseID string) ([]*TopicTreeNode, error) {
	if courseID == "" {
		return request[[]*TopicTreeNode](c, ctx, http.MethodGet, "/v1/topic-tree", nil, Actor{})
	}

	return request[[]*TopicTreeNode](c, ctx, http.MethodGet, "/v1/courses/"+url.PathEscape(courseID)+"/topic-tree", nil, Actor{})
}

func (c *Client) ListPrerequisites(ctx context.Context, topicID string) ([]TopicPrerequisite, error) {
	return request[[]TopicPrerequisite](c, ctx, http.MethodGet, "/v1/topics/"+url.PathEscape(topicID)+"/prerequisites", nil, Actor{})
}

func (c *Client) AddPrerequisite(ctx context.Context, topicID, prerequisiteID string, actor Actor) (TopicPrerequisite, error) {
	return request[TopicPrerequisite](c, ctx, http.MethodPost, "/v1/topics/"+url.PathEscape(topicID)+"/prerequisites", map[string]string{"prerequisite_topic_id": prerequisiteID}, actor)
}

func (c *Client) RemovePrerequisite(ctx context.Context, topicID, prerequisiteID string, actor Actor) error {
	_, err := request[map[string]bool](c, ctx, http.MethodDelete, "/v1/topics/"+url.PathEscape(topicID)+"/prerequisites/"+url.PathEscape(prerequisiteID), nil, actor)

	return err
}

func (c *Client) ValidateBinding(ctx context.Context, input Binding) (ValidationResult, error) {
	return request[ValidationResult](c, ctx, http.MethodPost, "/v1/validate/binding", input, Actor{})
}

func (c *Client) Health(ctx context.Context) error {
	return c.hx.Health(ctx)
}

// request выполняет типизированный запрос к entities.
func request[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	return httpx.DoJSON[T](c.hx, ctx, method, path, input, hxActor(actor))
}

// hxActor приводит Actor клиента к httpx.Actor для проброса заголовков.
func hxActor(actor Actor) httpx.Actor {
	return httpx.Actor{UserID: actor.UserID, Roles: actor.Roles}
}

func query(options ListOptions, parentKey, parentID string) string {
	values := url.Values{}
	values.Set("search", options.Search)
	values.Set("status", options.Status)
	values.Set("limit", strconv.Itoa(httpx.ClampLimit(options.Limit)))
	values.Set("offset", strconv.Itoa(options.Offset))
	if parentKey != "" {
		values.Set(parentKey, parentID)
	}

	return values.Encode()
}

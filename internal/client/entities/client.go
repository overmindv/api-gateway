package entities

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/overmindv/api-gateway/internal/middleware"
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
	baseURL string
	http    *http.Client
	log     *slog.Logger
}

func New(baseURL string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
		log: log,
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ready", nil)
	if err != nil {
		return fmt.Errorf("create entities health request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if requestID := middleware.RequestID(ctx); requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	started := time.Now()
	response, err := c.http.Do(req)
	if err != nil {
		c.log.WarnContext(ctx, "entities health call failed", "request_id", middleware.RequestID(ctx), "error", err, "duration", time.Since(started))

		return fmt.Errorf("call entities health: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)

		return fmt.Errorf("entities readiness status %d", response.StatusCode)
	}
	c.log.InfoContext(ctx, "entities health ok", "request_id", middleware.RequestID(ctx), "status", response.StatusCode, "duration", time.Since(started))

	return nil
}

// request выполняет typed HTTP-запрос в Entities.
// На вход получает client, context, HTTP method/path, optional body и actor, на выход возвращает decoded response или upstream error.
func request[T any](c *Client, ctx context.Context, method, path string, input any, actor Actor) (T, error) {
	var zero T
	started := time.Now()
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return zero, fmt.Errorf("marshal entities request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return zero, fmt.Errorf("create entities request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requestID := middleware.RequestID(ctx); requestID != "" {
		req.Header.Set(middleware.RequestIDHeader, requestID)
	}
	if actor.UserID != "" {
		req.Header.Set("X-User-ID", actor.UserID)
		req.Header.Set("X-User-Roles", strings.Join(actor.Roles, ","))
	}
	response, err := c.http.Do(req)
	if err != nil {
		c.log.WarnContext(ctx, "entities http call failed", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "error", err, "duration", time.Since(started))

		return zero, fmt.Errorf("call entities: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var upstream Error
		if decodeErr := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&upstream); decodeErr != nil {
			c.log.WarnContext(ctx, "entities http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "error", decodeErr, "duration", time.Since(started))

			return zero, fmt.Errorf("entities returned HTTP %d", response.StatusCode)
		}
		c.log.WarnContext(ctx, "entities http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "code", upstream.Code, "message", upstream.Message, "duration", time.Since(started))

		return zero, &upstream
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&zero); err != nil {
		c.log.WarnContext(ctx, "entities http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "error", err, "duration", time.Since(started))

		return zero, fmt.Errorf("decode entities response: %w", err)
	}
	c.log.InfoContext(ctx, "entities http call", "request_id", middleware.RequestID(ctx), "method", method, "path", path, "status", response.StatusCode, "duration", time.Since(started))

	return zero, nil
}

func query(options ListOptions, parentKey, parentID string) string {
	values := url.Values{}
	values.Set("search", options.Search)
	values.Set("status", options.Status)
	values.Set("limit", strconv.Itoa(options.Limit))
	values.Set("offset", strconv.Itoa(options.Offset))
	if parentKey != "" {
		values.Set(parentKey, parentID)
	}

	return values.Encode()
}

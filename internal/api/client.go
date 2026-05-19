package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	publicAPIPath = "/api/v1"
	userAgent     = "supervisible-cli"
)

// Client is a typed client for Supervisible public API v1.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	apiKey     string
	userAgent  string
}

// NewClient configures the client with an API key and a base URL.
func NewClient(baseURL, apiKey string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("missing api key")
	}

	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    normalized,
		apiKey:     apiKey,
		userAgent:  userAgent,
	}, nil
}

type apiErrorEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	} `json:"error"`
}

type apiDataEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// APIError is a normalized API failure.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("api error (%d %s): %s [request_id=%s]", e.StatusCode, e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("api error (%d %s): %s", e.StatusCode, e.Code, e.Message)
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = "https://app.supervisible.com"
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid base url: missing host")
	}

	trimmedPath := strings.TrimSuffix(u.Path, "/")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/api/public/v1")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/public/v1")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/api/v1")
	u.Path = path.Join(trimmedPath, publicAPIPath)
	if !strings.HasPrefix(u.Path, "/") {
		u.Path = "/" + u.Path
	}

	u.RawQuery = ""
	u.Fragment = ""

	return u, nil
}

// NormalizeBaseURL returns a canonical API base URL ending in /api/v1.
func NormalizeBaseURL(raw string) (string, error) {
	u, err := normalizeBaseURL(raw)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *Client) request(ctx context.Context, method, endpoint string, query url.Values, body any, out any) error {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, strings.TrimPrefix(endpoint, "/"))
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), payload)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Code:       "http_error",
			Message:    strings.TrimSpace(string(responseBody)),
			RequestID:  resp.Header.Get("x-request-id"),
		}

		var envelope apiErrorEnvelope
		if err := json.Unmarshal(responseBody, &envelope); err == nil && envelope.Error.Message != "" {
			apiErr.Code = envelope.Error.Code
			apiErr.Message = envelope.Error.Message
			if envelope.Error.RequestID != "" {
				apiErr.RequestID = envelope.Error.RequestID
			}
		}
		return apiErr
	}

	if out == nil || len(responseBody) == 0 {
		return nil
	}

	var envelope apiDataEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err == nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("decode response data: %w", err)
		}
		return nil
	}

	if err := json.Unmarshal(responseBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

// Do executes a raw API request against /api/v1 endpoints.
func (c *Client) Do(ctx context.Context, method, endpoint string, query url.Values, body any, out any) error {
	return c.request(ctx, method, endpoint, query, body, out)
}

type Pagination struct {
	Limit  int
	Offset int
}

func paginationQuery(p Pagination) url.Values {
	query := url.Values{}
	if p.Limit > 0 {
		query.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		query.Set("offset", strconv.Itoa(p.Offset))
	}
	return query
}

func (c *Client) Me(ctx context.Context) (Identity, error) {
	var out Identity
	if err := c.request(ctx, http.MethodGet, "/me", nil, nil, &out); err != nil {
		return Identity{}, err
	}
	return out, nil
}

func (c *Client) ListUsers(ctx context.Context, p Pagination) ([]User, error) {
	var out []User
	if err := c.request(ctx, http.MethodGet, "/users", paginationQuery(p), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UpdateUser(ctx context.Context, userID string, input UpdateUserInput) (User, error) {
	var out User
	if err := c.request(ctx, http.MethodPatch, "/users/"+userID, nil, input, &out); err != nil {
		return User{}, err
	}
	return out, nil
}

func (c *Client) ListClients(ctx context.Context, p Pagination) ([]ClientResource, error) {
	var out []ClientResource
	if err := c.request(ctx, http.MethodGet, "/clients", paginationQuery(p), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateClient(ctx context.Context, input CreateClientInput) (ClientResource, error) {
	var out ClientResource
	if err := c.request(ctx, http.MethodPost, "/clients", nil, input, &out); err != nil {
		return ClientResource{}, err
	}
	return out, nil
}

func (c *Client) UpdateClient(ctx context.Context, clientID string, input UpdateClientInput) (ClientResource, error) {
	var out ClientResource
	if err := c.request(ctx, http.MethodPatch, "/clients/"+clientID, nil, input, &out); err != nil {
		return ClientResource{}, err
	}
	return out, nil
}

func (c *Client) ListProjects(ctx context.Context, p Pagination) ([]Project, error) {
	var out []Project
	if err := c.request(ctx, http.MethodGet, "/projects", paginationQuery(p), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateProject(ctx context.Context, input CreateProjectInput) (Project, error) {
	var out Project
	if err := c.request(ctx, http.MethodPost, "/projects", nil, input, &out); err != nil {
		return Project{}, err
	}
	return out, nil
}

func (c *Client) UpdateProject(ctx context.Context, projectID string, input UpdateProjectInput) (Project, error) {
	var out Project
	if err := c.request(ctx, http.MethodPatch, "/projects/"+projectID, nil, input, &out); err != nil {
		return Project{}, err
	}
	return out, nil
}

type AssignmentFilters struct {
	UserID    string
	ProjectID string
	StartDate string
	EndDate   string
	Pagination
}

func (c *Client) ListAssignments(ctx context.Context, filters AssignmentFilters) ([]Assignment, error) {
	q := paginationQuery(filters.Pagination)
	if filters.UserID != "" {
		q.Set("user_id", filters.UserID)
	}
	if filters.ProjectID != "" {
		q.Set("project_id", filters.ProjectID)
	}
	if filters.StartDate != "" {
		q.Set("start_date", filters.StartDate)
	}
	if filters.EndDate != "" {
		q.Set("end_date", filters.EndDate)
	}

	var out []Assignment
	if err := c.request(ctx, http.MethodGet, "/assignments", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UpsertAssignments(ctx context.Context, input AssignmentUpsertInput) ([]Assignment, error) {
	var out []Assignment
	if err := c.request(ctx, http.MethodPost, "/assignments", nil, input, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type ActualHourFilters struct {
	UserID    string
	ProjectID string
	StartDate string
	EndDate   string
	Pagination
}

func (c *Client) ListActualHours(ctx context.Context, filters ActualHourFilters) ([]ActualHour, error) {
	q := paginationQuery(filters.Pagination)
	if filters.UserID != "" {
		q.Set("user_id", filters.UserID)
	}
	if filters.ProjectID != "" {
		q.Set("project_id", filters.ProjectID)
	}
	if filters.StartDate != "" {
		q.Set("start_date", filters.StartDate)
	}
	if filters.EndDate != "" {
		q.Set("end_date", filters.EndDate)
	}

	var out []ActualHour
	if err := c.request(ctx, http.MethodGet, "/actual-hours", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) UpsertActualHours(ctx context.Context, input ActualHourUpsertInput) ([]ActualHour, error) {
	var out []ActualHour
	if err := c.request(ctx, http.MethodPost, "/actual-hours", nil, input, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteActualHour(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "/actual-hours/"+id, nil, nil, nil)
}

func (c *Client) DeleteAssignment(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "/assignments/"+id, nil, nil, nil)
}

type TimeOffFilters struct {
	UserID    string
	Status    string
	StartDate string
	EndDate   string
	Pagination
}

func (c *Client) ListTimeOff(ctx context.Context, filters TimeOffFilters) ([]TimeOffRequest, error) {
	q := paginationQuery(filters.Pagination)
	if filters.UserID != "" {
		q.Set("user_id", filters.UserID)
	}
	if filters.Status != "" {
		q.Set("status", filters.Status)
	}
	if filters.StartDate != "" {
		q.Set("start_date", filters.StartDate)
	}
	if filters.EndDate != "" {
		q.Set("end_date", filters.EndDate)
	}

	var out []TimeOffRequest
	if err := c.request(ctx, http.MethodGet, "/time-off", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) CreateTimeOff(ctx context.Context, input CreateTimeOffInput) (TimeOffRequest, error) {
	var out TimeOffRequest
	if err := c.request(ctx, http.MethodPost, "/time-off", nil, input, &out); err != nil {
		return TimeOffRequest{}, err
	}
	return out, nil
}

func (c *Client) UpdateTimeOff(ctx context.Context, requestID string, input UpdateTimeOffInput) (TimeOffRequest, error) {
	var out TimeOffRequest
	if err := c.request(ctx, http.MethodPatch, "/time-off/"+requestID, nil, input, &out); err != nil {
		return TimeOffRequest{}, err
	}
	return out, nil
}

func (c *Client) DeleteTimeOff(ctx context.Context, requestID string) (map[string]string, error) {
	var out map[string]string
	if err := c.request(ctx, http.MethodDelete, "/time-off/"+requestID, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ApproveTimeOff(ctx context.Context, requestID string) (TimeOffRequest, error) {
	var out TimeOffRequest
	if err := c.request(ctx, http.MethodPost, "/time-off/"+requestID+"/approve", nil, nil, &out); err != nil {
		return TimeOffRequest{}, err
	}
	return out, nil
}

func (c *Client) RejectTimeOff(ctx context.Context, requestID string, input RejectTimeOffInput) (TimeOffRequest, error) {
	var out TimeOffRequest
	if err := c.request(ctx, http.MethodPost, "/time-off/"+requestID+"/reject", nil, input, &out); err != nil {
		return TimeOffRequest{}, err
	}
	return out, nil
}

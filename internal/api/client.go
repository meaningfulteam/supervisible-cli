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

// Client is a thin HTTP transport for the Supervisible public API.
//
// Public surface: NewClient/NewClientWithOptions, Client.Do.
// Typed helpers (Me, DeleteActualHour, DeleteAssignment) exist only where
// they earn their keep — auth-specific calls and DELETEs without bodies.
// Everything else routes through Do.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	apiKey     string
	userAgent  string
}

// ClientOptions configures optional client behavior.
type ClientOptions struct {
	Timeout  time.Duration
	Verbose  bool
	DebugOut io.Writer
}

// NewClient configures the client with an API key and a base URL.
func NewClient(baseURL, apiKey string, timeout time.Duration) (*Client, error) {
	return NewClientWithOptions(baseURL, apiKey, ClientOptions{Timeout: timeout})
}

// NewClientWithOptions configures the client with extended options.
func NewClientWithOptions(baseURL, apiKey string, opts ClientOptions) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("missing api key")
	}

	normalized, err := normalizeBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}

	httpClient := &http.Client{Timeout: opts.Timeout}
	if opts.Verbose {
		dest := opts.DebugOut
		if dest == nil {
			dest = io.Discard
		}
		httpClient.Transport = &debugRoundTripper{base: http.DefaultTransport, out: dest}
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    normalized,
		apiKey:     apiKey,
		userAgent:  userAgent,
	}, nil
}

// debugRoundTripper dumps HTTP request/response pairs to its writer.
// Authorization is masked; the response body is restored for downstream readers.
type debugRoundTripper struct {
	base http.RoundTripper
	out  io.Writer
}

func (d *debugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	d.dumpRequest(req)
	resp, err := d.base.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(d.out, "[supervisible] transport error: %v\n", err)
		return nil, err
	}
	d.dumpResponse(resp)
	return resp, nil
}

func (d *debugRoundTripper) dumpRequest(req *http.Request) {
	fmt.Fprintf(d.out, "[supervisible] > %s %s\n", req.Method, req.URL.String())
	for key, values := range req.Header {
		if strings.EqualFold(key, "Authorization") {
			for _, v := range values {
				fmt.Fprintf(d.out, "[supervisible] > %s: %s\n", key, maskAuthorization(v))
			}
			continue
		}
		for _, v := range values {
			fmt.Fprintf(d.out, "[supervisible] > %s: %s\n", key, v)
		}
	}
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
			if len(body) > 0 {
				fmt.Fprintf(d.out, "[supervisible] > body: %s\n", string(body))
			}
		}
	}
}

func (d *debugRoundTripper) dumpResponse(resp *http.Response) {
	fmt.Fprintf(d.out, "[supervisible] < %s\n", resp.Status)
	for key, values := range resp.Header {
		for _, v := range values {
			fmt.Fprintf(d.out, "[supervisible] < %s: %s\n", key, v)
		}
	}
	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			resp.Body = io.NopCloser(bytes.NewReader(body))
			if len(body) > 0 {
				fmt.Fprintf(d.out, "[supervisible] < body: %s\n", string(body))
			}
		}
	}
}

func maskAuthorization(raw string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(raw, prefix) {
		return "<redacted>"
	}
	token := raw[len(prefix):]
	if len(token) <= 10 {
		return prefix + "**********"
	}
	return prefix + token[:6] + "..." + token[len(token)-4:]
}

type apiErrorEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	} `json:"error"`
}

type apiDataEnvelope struct {
	Data     json.RawMessage `json:"data"`
	Warnings []Warning       `json:"warnings,omitempty"`
}

// Warning mirrors the server's PublicApiWarning envelope sibling. Surfaced
// when the server attaches soft signals to a successful response (e.g.
// time_off_overlap on POST /assignments).
type Warning struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
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
	_, err := c.requestWithWarnings(ctx, method, endpoint, query, body, out)
	return err
}

// requestWithWarnings is the underlying request executor. It returns any
// server-side warnings attached to the response envelope. Existing callers
// can keep using request() to discard them.
func (c *Client) requestWithWarnings(ctx context.Context, method, endpoint string, query url.Values, body any, out any) ([]Warning, error) {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, strings.TrimPrefix(endpoint, "/"))
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), payload)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
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
		return nil, apiErr
	}

	if len(responseBody) == 0 {
		return nil, nil
	}

	var envelope apiDataEnvelope
	if err := json.Unmarshal(responseBody, &envelope); err == nil && (len(envelope.Data) > 0 || len(envelope.Warnings) > 0) {
		if out != nil && len(envelope.Data) > 0 {
			if err := json.Unmarshal(envelope.Data, out); err != nil {
				return envelope.Warnings, fmt.Errorf("decode response data: %w", err)
			}
		}
		return envelope.Warnings, nil
	}

	if out == nil {
		return nil, nil
	}

	if err := json.Unmarshal(responseBody, out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return nil, nil
}

// Do executes a raw API request against /api/v1 endpoints. Server warnings
// are discarded — use DoWithWarnings to surface them.
func (c *Client) Do(ctx context.Context, method, endpoint string, query url.Values, body any, out any) error {
	return c.request(ctx, method, endpoint, query, body, out)
}

// DoWithWarnings is identical to Do but returns the server's `warnings`
// envelope sibling (e.g. time_off_overlap on POST /assignments). Empty slice
// when the server didn't attach any.
func (c *Client) DoWithWarnings(ctx context.Context, method, endpoint string, query url.Values, body any, out any) ([]Warning, error) {
	return c.requestWithWarnings(ctx, method, endpoint, query, body, out)
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

func (c *Client) DeleteActualHour(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "/actual-hours/"+id, nil, nil, nil)
}

func (c *Client) DeleteAssignment(ctx context.Context, id string) error {
	return c.request(ctx, http.MethodDelete, "/assignments/"+id, nil, nil, nil)
}

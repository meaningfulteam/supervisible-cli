package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/config"
	"github.com/supervisible/supervisible-cli/internal/output"
	"github.com/supervisible/supervisible-cli/internal/schema"
)

func newTestApp(t *testing.T, baseURL string, jsonMode, dryRun bool) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	store, err := config.NewStore(testConfigPath(t))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	cfg, _ := store.Load()
	prov, err := schema.NewProvider(context.Background())
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return &App{
		store:   store,
		cfg:     cfg,
		printer: output.NewPrinter(&stdout, &stderr, jsonMode),
		baseURL: baseURL,
		apiKey:  "test-token",
		timeout: 5 * time.Second,
		dryRun:  dryRun,
		schema:  prov,
	}, &stdout, &stderr
}

func TestAppExecute_DryRunReturnsFalseAndPrintsPlan(t *testing.T) {
	app, stdout, stderr := newTestApp(t, "http://example.invalid", false, true)

	executed, err := app.Execute(context.Background(), ExecuteOpts{
		CommandPath: "users list",
		Method:      "GET",
		Endpoint:    "/users",
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if executed {
		t.Fatalf("expected executed=false in dry-run, got true")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout in non-JSON dry-run, got: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Dry-run: GET /users") {
		t.Fatalf("expected dry-run preview on stderr, got: %q", stderr.String())
	}
}

func TestAppExecute_RealCallHitsServerOnce(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"u1"}]}`))
	}))
	defer server.Close()

	app, _, _ := newTestApp(t, server.URL, false, false)

	var users []map[string]any
	executed, err := app.Execute(context.Background(), ExecuteOpts{
		CommandPath: "users list",
		Method:      "GET",
		Endpoint:    "/users",
		Out:         &users,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !executed {
		t.Fatalf("expected executed=true on real call")
	}
	if calls != 1 {
		t.Fatalf("expected 1 server call, got %d", calls)
	}
	if len(users) != 1 || users[0]["id"] != "u1" {
		t.Fatalf("expected decoded user, got %#v", users)
	}
}

func TestAppExecute_SurfacesServerWarningsToStderr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{"id":"a1"}],
			"warnings": [
				{"code":"time_off_overlap","message":"Assignment on 2026-07-07 for user X overlaps approved PTO."},
				{"code":"time_off_overlap","message":"Assignment on 2026-07-08 for user X overlaps approved PTO."}
			]
		}`))
	}))
	defer server.Close()

	app, _, stderr := newTestApp(t, server.URL, false, false)

	var out []map[string]any
	executed, err := app.Execute(context.Background(), ExecuteOpts{
		CommandPath: "assignments upsert",
		Method:      "POST",
		Endpoint:    "/assignments",
		Out:         &out,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !executed {
		t.Fatalf("expected executed=true")
	}
	got := stderr.String()
	if !strings.Contains(got, "warning: time_off_overlap — Assignment on 2026-07-07") {
		t.Fatalf("expected first warning on stderr, got: %q", got)
	}
	if !strings.Contains(got, "warning: time_off_overlap — Assignment on 2026-07-08") {
		t.Fatalf("expected second warning on stderr, got: %q", got)
	}
}

func TestAppExecute_QuietWhenNoWarnings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"a1"}]}`))
	}))
	defer server.Close()

	app, _, stderr := newTestApp(t, server.URL, false, false)

	var out []map[string]any
	_, err := app.Execute(context.Background(), ExecuteOpts{
		CommandPath: "assignments upsert",
		Method:      "POST",
		Endpoint:    "/assignments",
		Out:         &out,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if got := stderr.String(); strings.Contains(got, "warning:") {
		t.Fatalf("expected no warning lines on stderr, got: %q", got)
	}
}

func TestAppExecute_PropagatesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "not_found", "message": "missing"},
		})
	}))
	defer server.Close()

	app, _, _ := newTestApp(t, server.URL, false, false)

	_, err := app.Execute(context.Background(), ExecuteOpts{
		CommandPath: "users get",
		Method:      "GET",
		Endpoint:    "/users/{user_id}",
		Path:        "/users/abc",
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *api.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", apiErr.StatusCode)
	}
}

func TestAppExecute_RequireClientErrorWhenNoAPIKey(t *testing.T) {
	app, _, _ := newTestApp(t, "http://example.invalid", false, false)
	app.apiKey = ""

	executed, err := app.Execute(context.Background(), ExecuteOpts{
		CommandPath: "users list",
		Method:      "GET",
		Endpoint:    "/users",
	})
	if err == nil {
		t.Fatalf("expected missing-api-key error, got nil")
	}
	if executed {
		t.Fatalf("expected executed=false on auth error")
	}
}

func TestPtrGeneric(t *testing.T) {
	if got := *ptr("x"); got != "x" {
		t.Fatalf("ptr(string) = %q, want x", got)
	}
	if got := *ptr(42); got != 42 {
		t.Fatalf("ptr(int) = %d, want 42", got)
	}
	if got := *ptr(true); got != true {
		t.Fatalf("ptr(bool) = %v, want true", got)
	}
	if got := *ptr(3.14); got != 3.14 {
		t.Fatalf("ptr(float64) = %v, want 3.14", got)
	}
}

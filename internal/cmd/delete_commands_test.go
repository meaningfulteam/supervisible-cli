package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testActualHourID = "550e8400-e29b-41d4-a716-446655440010"
const testAssignmentID = "550e8400-e29b-41d4-a716-446655440011"

// ── actual-hours delete ───────────────────────────────────────────────────────

func TestActualHoursDeleteSendsDeleteRequest(t *testing.T) {
	var capturedMethod, capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, _, err := executeCLI(t, "--config", testConfigPath(t), "actual-hours", "delete", testActualHourID)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", capturedMethod)
	}
	expectedPath := "/api/v1/actual-hours/" + testActualHourID
	if capturedPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, capturedPath)
	}
}

func TestActualHoursDeletePrintsMessageOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, stderr, err := executeCLI(t, "--config", testConfigPath(t), "actual-hours", "delete", testActualHourID)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if !strings.Contains(stderr, testActualHourID) {
		t.Fatalf("expected stderr to contain ID %q, got: %q", testActualHourID, stderr)
	}
}

func TestActualHoursDeleteJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t, "--config", testConfigPath(t), "--json", "actual-hours", "delete", testActualHourID)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid json output: %v (stdout=%q)", err, stdout)
	}
	if out["id"] != testActualHourID {
		t.Fatalf("expected id %q in json output, got %v", testActualHourID, out)
	}
}

func TestActualHoursDeleteInvalidUUID(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://localhost:9999")

	_, _, err := executeCLI(t, "--config", testConfigPath(t), "actual-hours", "delete", "not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
}

// ── assignments delete ────────────────────────────────────────────────────────

func TestAssignmentsDeleteSendsDeleteRequest(t *testing.T) {
	var capturedMethod, capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, _, err := executeCLI(t, "--config", testConfigPath(t), "assignments", "delete", testAssignmentID)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if capturedMethod != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", capturedMethod)
	}
	expectedPath := "/api/v1/assignments/" + testAssignmentID
	if capturedPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, capturedPath)
	}
}

func TestAssignmentsDeleteJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t, "--config", testConfigPath(t), "--json", "assignments", "delete", testAssignmentID)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var out map[string]string
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid json output: %v (stdout=%q)", err, stdout)
	}
	if out["id"] != testAssignmentID {
		t.Fatalf("expected id %q in json output, got %v", testAssignmentID, out)
	}
}

func TestAssignmentsDeleteInvalidUUID(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://localhost:9999")

	_, _, err := executeCLI(t, "--config", testConfigPath(t), "assignments", "delete", "not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
}

// ── upsert table output ───────────────────────────────────────────────────────

func TestActualHoursUpsertPrintsTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":[{"id":"ah-001","userId":"u-001","projectId":"p-001","date":"2026-02-01","hours":5}]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"actual-hours", "upsert",
		"--payload", `{"items":[{"userId":"u-001","projectId":"p-001","date":"2026-02-01","hours":5}]}`,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Table output should include column headers and the returned ID
	if !strings.Contains(stdout, "ID") {
		t.Fatalf("expected table header ID in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "ah-001") {
		t.Fatalf("expected row ID ah-001 in output, got: %q", stdout)
	}
}

func TestAssignmentsUpsertPrintsTable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":[{"id":"as-001","userId":"u-001","projectId":"p-001","date":"2026-02-01","hours":8}]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"assignments", "upsert",
		"--payload", `{"items":[{"userId":"u-001","projectId":"p-001","date":"2026-02-01","hours":8}]}`,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if !strings.Contains(stdout, "ID") {
		t.Fatalf("expected table header ID in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "as-001") {
		t.Fatalf("expected row ID as-001 in output, got: %q", stdout)
	}
}

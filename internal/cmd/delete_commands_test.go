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

	var out []map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid json output: %v (stdout=%q)", err, stdout)
	}
	if len(out) != 1 || out[0]["id"] != testActualHourID || out[0]["deleted"] != true {
		t.Fatalf("expected single deleted=true row for %q, got %v", testActualHourID, out)
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

	var out []map[string]any
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("invalid json output: %v (stdout=%q)", err, stdout)
	}
	if len(out) != 1 || out[0]["id"] != testAssignmentID || out[0]["deleted"] != true {
		t.Fatalf("expected single deleted=true row for %q, got %v", testAssignmentID, out)
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

// ── assignments delete: batch ─────────────────────────────────────────────────

func TestAssignmentsDelete_BatchSendsOneDeletePerID(t *testing.T) {
	var capturedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			capturedPaths = append(capturedPaths, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	id1 := "550e8400-e29b-41d4-a716-446655440021"
	id2 := "550e8400-e29b-41d4-a716-446655440022"
	id3 := "550e8400-e29b-41d4-a716-446655440023"

	stdout, stderr, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "delete", id1, id2, id3,
	)
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, stderr)
	}

	if len(capturedPaths) != 3 {
		t.Fatalf("expected 3 DELETE calls, got %d: %v", len(capturedPaths), capturedPaths)
	}
	if !strings.Contains(stderr, "batch delete: 3/3 succeeded") {
		t.Fatalf("expected batch summary on stderr, got: %q", stderr)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 result rows, got %d: %v", len(rows), rows)
	}
	for i, row := range rows {
		if row["deleted"] != true {
			t.Fatalf("row %d should be deleted=true, got %v", i, row)
		}
	}
}

func TestAssignmentsDelete_BatchStopsAtFirstError(t *testing.T) {
	id1 := "550e8400-e29b-41d4-a716-446655440031"
	id2 := "550e8400-e29b-41d4-a716-446655440032"
	id3 := "550e8400-e29b-41d4-a716-446655440033"

	var attempts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			attempts = append(attempts, r.URL.Path)
			if strings.HasSuffix(r.URL.Path, id2) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"missing"}}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, _, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"assignments", "delete", id1, id2, id3,
	)
	if err == nil {
		t.Fatalf("expected error on mid-batch failure")
	}
	if len(attempts) != 2 {
		t.Fatalf("expected 2 DELETE attempts before stop, got %d: %v", len(attempts), attempts)
	}
}

func TestAssignmentsDelete_BatchContinueOnError(t *testing.T) {
	id1 := "550e8400-e29b-41d4-a716-446655440041"
	id2 := "550e8400-e29b-41d4-a716-446655440042"
	id3 := "550e8400-e29b-41d4-a716-446655440043"

	var attempts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			attempts = append(attempts, r.URL.Path)
			if strings.HasSuffix(r.URL.Path, id2) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"missing"}}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, stderr, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "delete", "--continue-on-error", id1, id2, id3,
	)
	if err != nil {
		t.Fatalf("expected success with continue-on-error (had partial success): %v\n%s", err, stderr)
	}
	if len(attempts) != 3 {
		t.Fatalf("expected 3 DELETE attempts with --continue-on-error, got %d: %v", len(attempts), attempts)
	}
	if !strings.Contains(stderr, "batch delete: 2/3 succeeded") {
		t.Fatalf("expected 2/3 summary on stderr, got: %q", stderr)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 result rows, got %d: %v", len(rows), rows)
	}
	if rows[0]["deleted"] != true || rows[1]["deleted"] != false || rows[2]["deleted"] != true {
		t.Fatalf("expected [ok, fail, ok] pattern, got %v", rows)
	}
	if _, ok := rows[1]["error"].(string); !ok {
		t.Fatalf("expected error field on the failed row, got %v", rows[1])
	}
}

func TestAssignmentsDelete_BatchDryRunSkipsServerCalls(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	id1 := "550e8400-e29b-41d4-a716-446655440051"
	id2 := "550e8400-e29b-41d4-a716-446655440052"

	_, stderr, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"--dry-run",
		"assignments", "delete", id1, id2,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if attempts != 0 {
		t.Fatalf("expected 0 server calls on dry-run, got %d", attempts)
	}
	if !strings.Contains(stderr, "Dry-run: DELETE /assignments/"+id1) {
		t.Fatalf("expected per-ID dry-run line for %s, got: %q", id1, stderr)
	}
	if !strings.Contains(stderr, "Dry-run: DELETE /assignments/"+id2) {
		t.Fatalf("expected per-ID dry-run line for %s, got: %q", id2, stderr)
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

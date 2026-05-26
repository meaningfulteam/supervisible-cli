package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	removeUserID    = "550e8400-e29b-41d4-a716-446655440100"
	removeProjectID = "550e8400-e29b-41d4-a716-446655440200"
	removeCapID     = "550e8400-e29b-41d4-a716-446655440300"
)

// fakeRemoveServer answers GET /assignments by filtering rows that have
// date >= the start_date query param. DELETE /assignments/<id> records
// the path and returns 204 (unless deleteStatus is overridden).
type fakeRemoveServer struct {
	server         *httptest.Server
	rows           []map[string]any
	listQueries    atomic.Int32
	deletes        []string
	deleteStatus   int            // override for testing failures
	deletePerIDErr map[string]int // per-ID status override
}

func newRemoveServer(t *testing.T, rows []map[string]any) *fakeRemoveServer {
	t.Helper()
	s := &fakeRemoveServer{
		rows:         rows,
		deleteStatus: http.StatusNoContent,
	}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/assignments"):
			s.listQueries.Add(1)
			start := r.URL.Query().Get("start_date")
			filtered := make([]map[string]any, 0, len(s.rows))
			for _, row := range s.rows {
				date, _ := row["date"].(string)
				if start == "" || date >= start {
					filtered = append(filtered, row)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": filtered})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/assignments/"):
			parts := strings.Split(r.URL.Path, "/")
			id := parts[len(parts)-1]
			s.deletes = append(s.deletes, id)
			if status, ok := s.deletePerIDErr[id]; ok {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"missing"}}`))
				return
			}
			w.WriteHeader(s.deleteStatus)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func assignmentRow(id, date string, hours int) map[string]any {
	return map[string]any{
		"id":           id,
		"userId":       removeUserID,
		"projectId":    removeProjectID,
		"capabilityId": removeCapID,
		"date":         date,
		"hours":        hours,
		"createdAt":    "2026-01-01T00:00:00Z",
		"updatedAt":    "2026-01-01T00:00:00Z",
	}
}

func TestNextWeekSunday(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{"Sunday", time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC), "2026-05-31"},
		{"Tuesday", time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC), "2026-05-31"},
		{"Saturday", time.Date(2026, 5, 30, 23, 59, 0, 0, time.UTC), "2026-05-31"},
		{"NextSunday", time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC), "2026-06-07"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextWeekSunday(tc.now)
			if got != tc.want {
				t.Fatalf("nextWeekSunday(%s): want %s, got %s", tc.now.Format("2006-01-02 Mon"), tc.want, got)
			}
		})
	}
}

func TestAssignmentsRemoveFromProject_DefaultCutoffPreservesCurrentAndPast(t *testing.T) {
	// We can't easily inject a clock; instead, pass an explicit --since that
	// matches what nextWeekSunday would compute for "this week's Sunday + 7"
	// in real usage. This test relies on the explicit cutoff to verify the
	// boundary semantics; TestNextWeekSunday covers the default formula.
	rows := []map[string]any{
		assignmentRow("past-1", "2026-04-12", 10),
		assignmentRow("past-2", "2026-05-17", 10),
		assignmentRow("current", "2026-05-24", 10),
		assignmentRow("fut-1", "2026-05-31", 10),
		assignmentRow("fut-2", "2026-06-07", 10),
		assignmentRow("fut-3", "2026-06-14", 10),
	}
	s := newRemoveServer(t, rows)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "remove-from-project",
		"--user-id", removeUserID,
		"--project-id", removeProjectID,
		"--since", "2026-05-31",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if got := len(s.deletes); got != 3 {
		t.Fatalf("expected 3 DELETEs (only future rows), got %d: %v", got, s.deletes)
	}
	for _, id := range []string{"past-1", "past-2", "current"} {
		for _, deleted := range s.deletes {
			if deleted == id {
				t.Fatalf("unexpected DELETE of preserved row %s; calls=%v", id, s.deletes)
			}
		}
	}
}

func TestAssignmentsRemoveFromProject_NoFutureRowsExitsCleanly(t *testing.T) {
	rows := []map[string]any{
		assignmentRow("past-1", "2026-04-12", 10),
		assignmentRow("current", "2026-05-24", 10),
	}
	s := newRemoveServer(t, rows)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	stdout, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "remove-from-project",
		"--user-id", removeUserID,
		"--project-id", removeProjectID,
		"--since", "2026-05-31",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if len(s.deletes) != 0 {
		t.Fatalf("expected zero deletes when nothing future, got: %v", s.deletes)
	}
	if !strings.Contains(stderr, "no future assignments found") {
		t.Fatalf("expected informational stderr, got: %q", stderr)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected stdout=[] on no-match, got: %q", stdout)
	}
}

func TestAssignmentsRemoveFromProject_DryRunSkipsDeletes(t *testing.T) {
	rows := []map[string]any{
		assignmentRow("fut-1", "2026-05-31", 10),
		assignmentRow("fut-2", "2026-06-07", 10),
	}
	s := newRemoveServer(t, rows)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--dry-run",
		"assignments", "remove-from-project",
		"--user-id", removeUserID,
		"--project-id", removeProjectID,
		"--since", "2026-05-31",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if len(s.deletes) != 0 {
		t.Fatalf("expected zero DELETEs on dry-run, got: %v", s.deletes)
	}
	if !strings.Contains(stderr, "remove-from-project: 2 rows") {
		t.Fatalf("expected plan summary on stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "Dry-run: DELETE /assignments/fut-1") {
		t.Fatalf("expected per-ID dry-run preview for fut-1, got: %q", stderr)
	}
}

func TestAssignmentsRemoveFromProject_OverrideSinceWipesHistory(t *testing.T) {
	rows := []map[string]any{
		assignmentRow("hist-1", "2026-02-01", 10),
		assignmentRow("hist-2", "2026-03-15", 10),
		assignmentRow("recent", "2026-05-17", 10),
	}
	s := newRemoveServer(t, rows)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "remove-from-project",
		"--user-id", removeUserID,
		"--project-id", removeProjectID,
		"--since", "2026-02-01",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if len(s.deletes) != 3 {
		t.Fatalf("expected 3 DELETEs with backdated --since, got %d: %v", len(s.deletes), s.deletes)
	}
}

func TestAssignmentsRemoveFromProject_StopsAtFirstError(t *testing.T) {
	rows := []map[string]any{
		assignmentRow("a", "2026-05-31", 10),
		assignmentRow("b", "2026-06-07", 10),
		assignmentRow("c", "2026-06-14", 10),
	}
	s := newRemoveServer(t, rows)
	s.deletePerIDErr = map[string]int{"b": http.StatusNotFound}
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "remove-from-project",
		"--user-id", removeUserID,
		"--project-id", removeProjectID,
		"--since", "2026-05-31",
	)
	if err == nil {
		t.Fatalf("expected error on mid-batch failure")
	}
	if len(s.deletes) != 2 {
		t.Fatalf("expected 2 attempts before stop, got %d: %v", len(s.deletes), s.deletes)
	}
}

func TestAssignmentsRemoveFromProject_ContinueOnError(t *testing.T) {
	rows := []map[string]any{
		assignmentRow("a", "2026-05-31", 10),
		assignmentRow("b", "2026-06-07", 10),
		assignmentRow("c", "2026-06-14", 10),
	}
	s := newRemoveServer(t, rows)
	s.deletePerIDErr = map[string]int{"b": http.StatusNotFound}
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "remove-from-project",
		"--user-id", removeUserID,
		"--project-id", removeProjectID,
		"--since", "2026-05-31",
		"--continue-on-error",
	)
	if err != nil {
		t.Fatalf("expected success with partial: %v\n%s", err, stderr)
	}
	if len(s.deletes) != 3 {
		t.Fatalf("expected 3 attempts with --continue-on-error, got %d: %v", len(s.deletes), s.deletes)
	}
	if !strings.Contains(stderr, "batch delete: 2/3 succeeded") {
		t.Fatalf("expected partial-success summary on stderr, got: %q", stderr)
	}
}

func TestAssignmentsRemoveFromProject_RejectsBadDate(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://localhost:9999")

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "remove-from-project",
		"--user-id", removeUserID,
		"--project-id", removeProjectID,
		"--since", "tomorrow",
	)
	if err == nil {
		t.Fatalf("expected error for invalid --since")
	}
}

func TestAssignmentsRemoveFromProject_RequiresUserAndProject(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://localhost:9999")

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "remove-from-project",
		"--user-id", removeUserID,
	)
	if err == nil {
		t.Fatalf("expected error for missing --project-id")
	}
}

package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeMoveServer mocks just enough of /assignments to drive the move flow.
// fixtures contains rows returned by GET; mutate it to express server state.
type moveFixture struct {
	rows     []map[string]any // returned by GET /assignments
	posts    []map[string]any // captured POST bodies
	deletes  []string         // captured DELETE assignment IDs
	postFail bool
	delFail  bool
	postResp []map[string]any // returned by POST /assignments
}

func newMoveServer(t *testing.T, fx *moveFixture) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-off"):
			// Pre-flight time-off check (only fires in dry-run); return nothing.
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/assignments"):
			// Scope by user_id so the same fixture serves both source-row lookup
			// and target existing-hours lookup without cross-contamination.
			userID := r.URL.Query().Get("user_id")
			filtered := make([]map[string]any, 0, len(fx.rows))
			for _, row := range fx.rows {
				if userID == "" || row["userId"] == userID {
					filtered = append(filtered, row)
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": filtered})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/assignments"):
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			fx.posts = append(fx.posts, body)
			if fx.postFail {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
				return
			}
			resp := fx.postResp
			if resp == nil {
				resp = []map[string]any{{"id": "new", "userId": "u-new", "projectId": "p1", "date": "2026-05-21", "hours": 4}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": resp})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/assignments/"):
			parts := strings.Split(r.URL.Path, "/")
			id := parts[len(parts)-1]
			fx.deletes = append(fx.deletes, id)
			if fx.delFail {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	return srv, &calls
}

const (
	moveAssignmentID = "019d27a4-0000-7000-8000-000000000001"
	moveFromUser     = "019d27a4-0000-7000-8000-000000000002"
	moveToUser       = "019d27a4-0000-7000-8000-000000000003"
	moveProject      = "019d27a4-0000-7000-8000-000000000004"
	moveCapability   = "019d27a4-0000-7000-8000-000000000005"
)

func sourceRow(t *testing.T, hours int) map[string]any {
	t.Helper()
	return map[string]any{
		"id":           moveAssignmentID,
		"userId":       moveFromUser,
		"projectId":    moveProject,
		"capabilityId": moveCapability,
		"date":         "2026-05-21",
		"hours":        hours,
		"createdAt":    "2026-01-01T00:00:00Z",
		"updatedAt":    "2026-01-01T00:00:00Z",
	}
}

func TestAssignmentsMove_RejectsSameUser(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://example.invalid")

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "move", moveAssignmentID,
		"--from-user", moveFromUser,
		"--to-user", moveFromUser,
		"--capability-id", moveCapability,
	)
	if err == nil || !strings.Contains(err.Error(), "different users") {
		t.Fatalf("expected same-user rejection, got: %v", err)
	}
}

func TestAssignmentsMove_HoursExceedSourceRejectsBeforeWrite(t *testing.T) {
	fx := &moveFixture{rows: []map[string]any{sourceRow(t, 4)}}
	srv, _ := newMoveServer(t, fx)
	defer srv.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", srv.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "move", moveAssignmentID,
		"--from-user", moveFromUser,
		"--to-user", moveToUser,
		"--capability-id", moveCapability,
		"--hours", "10",
	)
	if err == nil || !strings.Contains(err.Error(), "exceeds source") {
		t.Fatalf("expected hours-exceed rejection, got: %v", err)
	}
	if len(fx.posts) != 0 || len(fx.deletes) != 0 {
		t.Fatalf("expected no writes; got posts=%v deletes=%v", fx.posts, fx.deletes)
	}
}

func TestAssignmentsMove_AllHoursDeletesSource(t *testing.T) {
	fx := &moveFixture{
		rows: []map[string]any{sourceRow(t, 6)},
	}
	srv, _ := newMoveServer(t, fx)
	defer srv.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", srv.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "move", moveAssignmentID,
		"--from-user", moveFromUser,
		"--to-user", moveToUser,
		"--capability-id", moveCapability,
	)
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, stderr)
	}
	// Exactly one POST (target upsert with full 6h).
	if len(fx.posts) != 1 {
		t.Fatalf("expected 1 POST (target upsert), got %d", len(fx.posts))
	}
	items, _ := fx.posts[0]["items"].([]any)
	first, _ := items[0].(map[string]any)
	if first["userId"] != moveToUser || first["hours"].(float64) != 6 {
		t.Fatalf("target upsert body wrong: %+v", first)
	}
	// Source row deleted.
	if len(fx.deletes) != 1 || fx.deletes[0] != moveAssignmentID {
		t.Fatalf("expected DELETE %s, got %v", moveAssignmentID, fx.deletes)
	}
}

func TestAssignmentsMove_PartialHoursDecrementsSource(t *testing.T) {
	fx := &moveFixture{
		rows: []map[string]any{sourceRow(t, 8)},
	}
	srv, _ := newMoveServer(t, fx)
	defer srv.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", srv.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "move", moveAssignmentID,
		"--from-user", moveFromUser,
		"--to-user", moveToUser,
		"--capability-id", moveCapability,
		"--hours", "3",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	// Two POSTs: target upsert (+3) then source upsert (5h remaining).
	if len(fx.posts) != 2 {
		t.Fatalf("expected 2 POSTs, got %d: %+v", len(fx.posts), fx.posts)
	}
	if len(fx.deletes) != 0 {
		t.Fatalf("expected no DELETE on partial, got %v", fx.deletes)
	}

	targetItems, _ := fx.posts[0]["items"].([]any)
	targetFirst, _ := targetItems[0].(map[string]any)
	if targetFirst["userId"] != moveToUser || targetFirst["hours"].(float64) != 3 {
		t.Fatalf("target upsert wrong: %+v", targetFirst)
	}
	sourceItems, _ := fx.posts[1]["items"].([]any)
	sourceFirst, _ := sourceItems[0].(map[string]any)
	if sourceFirst["userId"] != moveFromUser || sourceFirst["hours"].(float64) != 5 {
		t.Fatalf("source decrement wrong: %+v", sourceFirst)
	}
}

func TestAssignmentsMove_TargetUpsertFailureLeavesSourceUntouched(t *testing.T) {
	fx := &moveFixture{
		rows:     []map[string]any{sourceRow(t, 6)},
		postFail: true,
	}
	srv, _ := newMoveServer(t, fx)
	defer srv.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", srv.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "move", moveAssignmentID,
		"--from-user", moveFromUser,
		"--to-user", moveToUser,
		"--capability-id", moveCapability,
	)
	if err == nil {
		t.Fatalf("expected error on target upsert failure")
	}
	if len(fx.deletes) != 0 {
		t.Fatalf("source must remain untouched on target failure; got DELETE %v", fx.deletes)
	}
}

func TestAssignmentsMove_DeleteFailureEmitsPartialFailureWarning(t *testing.T) {
	fx := &moveFixture{
		rows:    []map[string]any{sourceRow(t, 6)},
		delFail: true,
	}
	srv, _ := newMoveServer(t, fx)
	defer srv.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", srv.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "move", moveAssignmentID,
		"--from-user", moveFromUser,
		"--to-user", moveToUser,
		"--capability-id", moveCapability,
	)
	if err == nil {
		t.Fatalf("expected non-zero exit on partial failure")
	}
	if !strings.Contains(stderr, "PARTIAL FAILURE") {
		t.Fatalf("expected PARTIAL FAILURE warning on stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, moveAssignmentID) {
		t.Fatalf("partial-failure warning must reference source ID, got: %q", stderr)
	}
}

func TestAssignmentsMove_DryRunSkipsWrites(t *testing.T) {
	fx := &moveFixture{
		rows: []map[string]any{sourceRow(t, 6)},
	}
	srv, _ := newMoveServer(t, fx)
	defer srv.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", srv.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"--dry-run",
		"assignments", "move", moveAssignmentID,
		"--from-user", moveFromUser,
		"--to-user", moveToUser,
		"--capability-id", moveCapability,
	)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if len(fx.posts) != 0 || len(fx.deletes) != 0 {
		t.Fatalf("dry-run must not write; got posts=%v deletes=%v", fx.posts, fx.deletes)
	}
	var plan map[string]any
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("invalid dry-run json: %v\n%s", err, stdout)
	}
	if plan["target_upsert"] == nil || plan["source"] == nil {
		t.Fatalf("expected dry-run plan to include target_upsert and source, got: %v", plan)
	}
}

func TestAssignmentsMove_SourceWithZeroHoursRejected(t *testing.T) {
	fx := &moveFixture{
		rows: []map[string]any{sourceRow(t, 0)},
	}
	srv, _ := newMoveServer(t, fx)
	defer srv.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", srv.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "move", moveAssignmentID,
		"--from-user", moveFromUser,
		"--to-user", moveToUser,
		"--capability-id", moveCapability,
	)
	if err == nil || !strings.Contains(err.Error(), "nothing to move") {
		t.Fatalf("expected nothing-to-move rejection, got: %v", err)
	}
}

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/supervisible/supervisible-cli/internal/api"
)

const (
	addUserID    = "550e8400-e29b-41d4-a716-446655440100"
	addProjectID = "550e8400-e29b-41d4-a716-446655440200"
	addCapID     = "550e8400-e29b-41d4-a716-446655440300"
)

type addServer struct {
	server         *httptest.Server
	existing       []api.Assignment // returned by GET /assignments
	gotUpsertBody  atomic.Value     // last POST body (string)
	gotDeletePath  atomic.Value     // last DELETE path (string)
	upsertResponse []api.Assignment
}

func newAddServer(t *testing.T, existing []api.Assignment) *addServer {
	t.Helper()
	s := &addServer{existing: existing}
	s.upsertResponse = []api.Assignment{
		{ID: "new-id", UserID: addUserID, ProjectID: addProjectID, Date: "2026-05-24", Hours: 4},
	}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/assignments"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": s.existing})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/assignments"):
			body := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(body)
			s.gotUpsertBody.Store(string(body))
			_ = json.NewEncoder(w).Encode(map[string]any{"data": s.upsertResponse})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/assignments/"):
			s.gotDeletePath.Store(r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func setupAddEnv(t *testing.T, server *httptest.Server) {
	t.Helper()
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)
}

func TestAssignmentsAdd_NoExistingRow_DeltaBecomesTotal(t *testing.T) {
	s := newAddServer(t, nil)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "add",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--capability-id", addCapID,
		"--date", "2026-05-24",
		"--hours", "3",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "0h + 3h = 3h") {
		t.Fatalf("expected summary '0h + 3h = 3h' on stderr, got: %q", stderr)
	}
	body, _ := s.gotUpsertBody.Load().(string)
	if !strings.Contains(body, `"hours":3`) {
		t.Fatalf("expected upsert body to contain hours=3, got: %q", body)
	}
}

func TestAssignmentsAdd_WithExistingRow_AddsDelta(t *testing.T) {
	cap := addCapID
	existing := []api.Assignment{
		{ID: "existing-id", UserID: addUserID, ProjectID: addProjectID, CapabilityID: &cap, Date: "2026-05-24", Hours: 2},
	}
	s := newAddServer(t, existing)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "add",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--capability-id", addCapID,
		"--date", "2026-05-24",
		"--hours", "2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "2h + 2h = 4h") {
		t.Fatalf("expected summary '2h + 2h = 4h' on stderr, got: %q", stderr)
	}
	body, _ := s.gotUpsertBody.Load().(string)
	if !strings.Contains(body, `"hours":4`) {
		t.Fatalf("expected upsert body to contain hours=4, got: %q", body)
	}
}

func TestAssignmentsAdd_NegativeResultRefusesToWrite(t *testing.T) {
	cap := addCapID
	existing := []api.Assignment{
		{ID: "existing-id", UserID: addUserID, ProjectID: addProjectID, CapabilityID: &cap, Date: "2026-05-24", Hours: 2},
	}
	s := newAddServer(t, existing)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "add",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--capability-id", addCapID,
		"--date", "2026-05-24",
		"--hours", "-10",
	)
	if err == nil {
		t.Fatalf("expected negative-hours error, got nil")
	}
	if body, _ := s.gotUpsertBody.Load().(string); body != "" {
		t.Fatalf("expected no upsert when result would be negative, got body: %q", body)
	}
}

func TestAssignmentsAdd_ZeroResultDeletesExistingRow(t *testing.T) {
	cap := addCapID
	existing := []api.Assignment{
		{ID: "existing-id", UserID: addUserID, ProjectID: addProjectID, CapabilityID: &cap, Date: "2026-05-24", Hours: 2},
	}
	s := newAddServer(t, existing)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "add",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--capability-id", addCapID,
		"--date", "2026-05-24",
		"--hours", "-2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body, _ := s.gotUpsertBody.Load().(string); body != "" {
		t.Fatalf("expected no upsert when result is 0, got body: %q", body)
	}
	deletePath, _ := s.gotDeletePath.Load().(string)
	if !strings.HasSuffix(deletePath, "/assignments/existing-id") {
		t.Fatalf("expected DELETE /assignments/existing-id, got: %q", deletePath)
	}
	if !strings.Contains(stderr, "deleting assignment existing-id") {
		t.Fatalf("expected delete summary on stderr, got: %q", stderr)
	}
}

func TestAssignmentsAdd_ZeroResultDryRunSkipsDelete(t *testing.T) {
	cap := addCapID
	existing := []api.Assignment{
		{ID: "existing-id", UserID: addUserID, ProjectID: addProjectID, CapabilityID: &cap, Date: "2026-05-24", Hours: 2},
	}
	s := newAddServer(t, existing)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--dry-run",
		"assignments", "add",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--capability-id", addCapID,
		"--date", "2026-05-24",
		"--hours", "-2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path, _ := s.gotDeletePath.Load().(string); path != "" {
		t.Fatalf("expected no DELETE on dry-run, got: %q", path)
	}
	if !strings.Contains(stderr, "Dry-run: DELETE /assignments/existing-id") {
		t.Fatalf("expected dry-run preview, got: %q", stderr)
	}
}

func TestAssignmentsUpsert_AutoCapabilityFillsItem(t *testing.T) {
	historyCap := addCapID
	history := []api.Assignment{
		{ID: "h1", UserID: addUserID, ProjectID: addProjectID, CapabilityID: &historyCap, Date: "2026-05-01", Hours: 4},
	}
	s := newAddServer(t, history)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "upsert",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--date", "2026-05-24",
		"--hours", "8",
		"--auto-capability",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body, _ := s.gotUpsertBody.Load().(string)
	if !strings.Contains(body, addCapID) {
		t.Fatalf("expected upsert body to carry resolved capability %q, got: %q", addCapID, body)
	}
}

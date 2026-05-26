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

const (
	moveAssignmentID = "019d27a4-0000-7000-8000-000000000001"
	moveToUser       = "019d27a4-0000-7000-8000-000000000003"
	moveCapability   = "019d27a4-0000-7000-8000-000000000005"
)

type moveServer struct {
	server     *httptest.Server
	gotPath    atomic.Value // string
	gotBody    atomic.Value // map[string]any
	status     int
	respSource any
	respTarget map[string]any
}

func newMoveServer(t *testing.T) *moveServer {
	t.Helper()
	s := &moveServer{
		status:     200,
		respTarget: map[string]any{"id": "new-target-id", "userId": moveToUser, "projectId": "p1", "date": "2026-05-21", "hours": 6},
	}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/move") {
			http.NotFound(w, r)
			return
		}
		s.gotPath.Store(r.URL.Path)
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		s.gotBody.Store(body)

		if s.status != 200 {
			w.WriteHeader(s.status)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"bad"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"source": s.respSource,
				"target": s.respTarget,
			},
		})
	}))
	return s
}

func TestAssignmentsMove_PostsToServerMoveEndpoint(t *testing.T) {
	s := newMoveServer(t)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "move", moveAssignmentID,
		"--to-user", moveToUser,
		"--capability-id", moveCapability,
		"--hours", "4",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	path, _ := s.gotPath.Load().(string)
	if !strings.HasSuffix(path, "/assignments/"+moveAssignmentID+"/move") {
		t.Fatalf("expected POST to /assignments/<id>/move, got: %q", path)
	}

	body, _ := s.gotBody.Load().(map[string]any)
	if body["toUserId"] != moveToUser {
		t.Fatalf("expected toUserId=%s, got: %v", moveToUser, body["toUserId"])
	}
	if body["capabilityId"] != moveCapability {
		t.Fatalf("expected capabilityId=%s, got: %v", moveCapability, body["capabilityId"])
	}
	if hrs, ok := body["hours"].(float64); !ok || int(hrs) != 4 {
		t.Fatalf("expected hours=4, got: %v", body["hours"])
	}
}

func TestAssignmentsMove_OmitsOptionalFlagsFromBody(t *testing.T) {
	s := newMoveServer(t)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "move", moveAssignmentID,
		"--to-user", moveToUser,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	body, _ := s.gotBody.Load().(map[string]any)
	if _, has := body["hours"]; has {
		t.Fatalf("expected no hours field when flag unset, got: %v", body)
	}
	if _, has := body["capabilityId"]; has {
		t.Fatalf("expected no capabilityId field when flag unset, got: %v", body)
	}
}

func TestAssignmentsMove_JSONOutputCarriesServerResponse(t *testing.T) {
	s := newMoveServer(t)
	s.respSource = nil // server tells us source was fully consumed
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"assignments", "move", moveAssignmentID,
		"--to-user", moveToUser,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if got["source"] != nil {
		t.Fatalf("expected source=null on full move, got: %v", got["source"])
	}
	target, _ := got["target"].(map[string]any)
	if target["id"] != "new-target-id" {
		t.Fatalf("expected target.id passthrough, got: %v", target)
	}
}

func TestAssignmentsMove_NonJSONPrintsHumanSummary(t *testing.T) {
	s := newMoveServer(t)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "move", moveAssignmentID,
		"--to-user", moveToUser,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(stderr, "moved:") {
		t.Fatalf("expected 'moved:' summary on stderr, got: %q", stderr)
	}
}

func TestAssignmentsMove_DryRunSkipsServerCall(t *testing.T) {
	s := newMoveServer(t)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--dry-run",
		"assignments", "move", moveAssignmentID,
		"--to-user", moveToUser,
		"--hours", "4",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if path, _ := s.gotPath.Load().(string); path != "" {
		t.Fatalf("expected no POST on dry-run, got: %q", path)
	}
	if !strings.Contains(stderr, "Dry-run: POST /assignments/"+moveAssignmentID+"/move") {
		t.Fatalf("expected dry-run preview on stderr, got: %q", stderr)
	}
}

func TestAssignmentsMove_RejectsNegativeHoursBeforeServerCall(t *testing.T) {
	s := newMoveServer(t)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "move", moveAssignmentID,
		"--to-user", moveToUser,
		"--hours", "-5",
	)
	if err == nil {
		t.Fatalf("expected error for negative hours")
	}
	if path, _ := s.gotPath.Load().(string); path != "" {
		t.Fatalf("expected no server call on validation error, got POST to %q", path)
	}
}

func TestAssignmentsMove_ServerErrorPropagates(t *testing.T) {
	s := newMoveServer(t)
	s.status = 400
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"assignments", "move", moveAssignmentID,
		"--to-user", moveToUser,
	)
	if err == nil {
		t.Fatalf("expected error when server returns 400")
	}
}

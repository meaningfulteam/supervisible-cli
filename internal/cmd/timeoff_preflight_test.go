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

type preflightServer struct {
	server       *httptest.Server
	timeOff      []api.TimeOffRequest
	timeOffCalls atomic.Int32
}

func newPreflightServer(t *testing.T, timeOff []api.TimeOffRequest, existing []api.Assignment) *preflightServer {
	t.Helper()
	s := &preflightServer{timeOff: timeOff}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-off"):
			s.timeOffCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"data": s.timeOff})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/assignments"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": existing})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/assignments"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []api.Assignment{}})
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func TestPreflightTimeOff_WarnsOnOverlap(t *testing.T) {
	name := "Juan Mendez"
	timeOff := []api.TimeOffRequest{{
		ID:        "to-1",
		UserID:    addUserID,
		Status:    "approved",
		StartDate: "2026-05-10",
		EndDate:   "2026-07-03",
		User:      &api.ExpandedUser{ID: addUserID, Name: &name},
	}}
	s := newPreflightServer(t, timeOff, nil)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--dry-run",
		"assignments", "upsert",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--capability-id", addCapID,
		"--date", "2026-05-24",
		"--hours", "2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr, "warning: time-off overlap") {
		t.Fatalf("expected overlap warning, got stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "Juan Mendez") {
		t.Fatalf("expected user name in warning, got: %q", stderr)
	}
}

func TestPreflightTimeOff_NoWarningWhenNoOverlap(t *testing.T) {
	name := "Juan Mendez"
	timeOff := []api.TimeOffRequest{{
		ID:        "to-1",
		UserID:    addUserID,
		Status:    "approved",
		StartDate: "2026-08-10",
		EndDate:   "2026-08-15",
		User:      &api.ExpandedUser{ID: addUserID, Name: &name},
	}}
	s := newPreflightServer(t, timeOff, nil)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--dry-run",
		"assignments", "upsert",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--capability-id", addCapID,
		"--date", "2026-05-24",
		"--hours", "2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(stderr, "time-off overlap") {
		t.Fatalf("expected no warning for non-overlapping range, got: %q", stderr)
	}
}

func TestPreflightTimeOff_SkippedWhenNotDryRun(t *testing.T) {
	name := "Juan Mendez"
	timeOff := []api.TimeOffRequest{{
		ID:        "to-1",
		UserID:    addUserID,
		Status:    "approved",
		StartDate: "2026-05-10",
		EndDate:   "2026-07-03",
		User:      &api.ExpandedUser{ID: addUserID, Name: &name},
	}}
	s := newPreflightServer(t, timeOff, nil)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		// no --dry-run
		"assignments", "upsert",
		"--user-id", addUserID,
		"--project-id", addProjectID,
		"--capability-id", addCapID,
		"--date", "2026-05-24",
		"--hours", "2",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.timeOffCalls.Load() != 0 {
		t.Fatalf("expected zero /time-off calls outside dry-run, got %d", s.timeOffCalls.Load())
	}
}

func TestPreflightTimeOff_AggregatesPerTimeOffEntry(t *testing.T) {
	name := "Juan Mendez"
	timeOff := []api.TimeOffRequest{{
		ID:        "to-1",
		UserID:    addUserID,
		Status:    "approved",
		StartDate: "2026-05-10",
		EndDate:   "2026-07-03",
		User:      &api.ExpandedUser{ID: addUserID, Name: &name},
	}}
	s := newPreflightServer(t, timeOff, nil)
	defer s.server.Close()
	setupAddEnv(t, s.server)

	bulk := `{"items":[
        {"userId":"` + addUserID + `","projectId":"` + addProjectID + `","capabilityId":"` + addCapID + `","date":"2026-05-20","hours":2},
        {"userId":"` + addUserID + `","projectId":"` + addProjectID + `","capabilityId":"` + addCapID + `","date":"2026-05-21","hours":2},
        {"userId":"` + addUserID + `","projectId":"` + addProjectID + `","capabilityId":"` + addCapID + `","date":"2026-05-22","hours":2}
    ]}`

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--dry-run",
		"assignments", "upsert",
		"--body", bulk,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := strings.Count(stderr, "warning: time-off overlap")
	if count != 1 {
		t.Fatalf("expected exactly 1 aggregated warning, got %d. stderr: %q", count, stderr)
	}
}

package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeCapabilitiesServer mirrors GET /capabilities including the
// for_project filter that PR #885 added: when set, only rows tagged with
// that projectID are returned.
type fakeCapabilitiesServer struct {
	server     *httptest.Server
	capsByProj map[string][]map[string]any // projectID -> rows; "" key = unfiltered org-wide
	gotQuery   string
}

func newCapabilitiesServer(t *testing.T, capsByProj map[string][]map[string]any) *fakeCapabilitiesServer {
	t.Helper()
	s := &fakeCapabilitiesServer{capsByProj: capsByProj}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/capabilities") {
			http.NotFound(w, r)
			return
		}
		s.gotQuery = r.URL.RawQuery
		proj := r.URL.Query().Get("for_project")
		rows, ok := s.capsByProj[proj]
		if !ok {
			rows = []map[string]any{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
	}))
	return s
}

func capRow(id, name string) map[string]any {
	return map[string]any{
		"id":          id,
		"name":        name,
		"description": nil,
		"createdAt":   "2026-01-01T00:00:00Z",
		"updatedAt":   "2026-01-01T00:00:00Z",
	}
}

func TestCapabilitiesList_UnfilteredHitsServer(t *testing.T) {
	s := newCapabilitiesServer(t, map[string][]map[string]any{
		"": {capRow("c1", "Project Management"), capRow("c2", "Design")},
	})
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"capabilities", "list",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 org-wide rows, got %d: %v", len(rows), rows)
	}
	if strings.Contains(s.gotQuery, "for_project=") {
		t.Fatalf("expected no for_project param when --for-project absent, got query: %q", s.gotQuery)
	}
}

func TestCapabilitiesList_ForProjectPassesParamThrough(t *testing.T) {
	const projID = "019c885e-576c-7a7d-ba22-13a3778fb8cb"
	s := newCapabilitiesServer(t, map[string][]map[string]any{
		projID: {capRow("c-on-proj", "Project Management")},
	})
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	stdout, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"capabilities", "list",
		"--for-project", projID,
	)
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, stderr)
	}
	if !strings.Contains(s.gotQuery, "for_project="+projID) {
		t.Fatalf("expected for_project param in query, got: %q", s.gotQuery)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if len(rows) != 1 || rows[0]["id"] != "c-on-proj" {
		t.Fatalf("expected single Project Management row, got %v", rows)
	}
	// Critical: no more "derived from assignment history" warning.
	if strings.Contains(stderr, "derived from") {
		t.Fatalf("expected no derived-view warning, got stderr: %q", stderr)
	}
	// Critical: no more "source" field in the JSON.
	if _, hasSource := rows[0]["source"]; hasSource {
		t.Fatalf("expected no 'source' field in canonical output, got %v", rows[0])
	}
}

func TestCapabilitiesList_EmptyForProjectShowsNote(t *testing.T) {
	const projID = "019c885e-576c-7a7d-ba22-13a3778fb8cb"
	s := newCapabilitiesServer(t, map[string][]map[string]any{
		projID: {}, // server returns empty for this project
	})
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	stdout, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"capabilities", "list",
		"--for-project", projID,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected stdout=[] on empty result, got: %q", stdout)
	}
	if !strings.Contains(stderr, "no capabilities attached to project") {
		t.Fatalf("expected per-project empty note, got: %q", stderr)
	}
}

func TestCapabilitiesList_RejectsBadProjectUUID(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://localhost:9999")

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"capabilities", "list",
		"--for-project", "not-a-uuid",
	)
	if err == nil {
		t.Fatalf("expected error for bad --for-project UUID")
	}
}

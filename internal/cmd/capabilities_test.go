package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/supervisible/supervisible-cli/internal/api"
)

func TestAggregateDerivedCapabilities_GroupsAndCounts(t *testing.T) {
	a := "cap-a"
	b := "cap-b"
	rows := []api.Assignment{
		{ID: "1", CapabilityID: &a, Hours: 4, Capability: &api.ExpandedCapability{ID: "cap-a", Name: "PM"}},
		{ID: "2", CapabilityID: &a, Hours: 8, Capability: &api.ExpandedCapability{ID: "cap-a", Name: "PM"}},
		{ID: "3", CapabilityID: &b, Hours: 2, Capability: &api.ExpandedCapability{ID: "cap-b", Name: "Design"}},
	}
	got := aggregateDerivedCapabilities(rows)
	if len(got) != 2 {
		t.Fatalf("expected 2 derived rows, got %d: %+v", len(got), got)
	}
	if got[0].CapabilityID != "cap-a" || got[0].UsageCount != 2 || got[0].Name != "PM" {
		t.Fatalf("expected cap-a/PM/2 first, got %+v", got[0])
	}
	for _, d := range got {
		if d.Source != capabilitySourceDerived {
			t.Fatalf("expected source=%s, got %q", capabilitySourceDerived, d.Source)
		}
	}
}

func TestAggregateDerivedCapabilities_SkipsZombieAndNullCap(t *testing.T) {
	a := "cap-a"
	rows := []api.Assignment{
		{ID: "zombie", CapabilityID: &a, Hours: 0, Capability: &api.ExpandedCapability{ID: "cap-a", Name: "PM"}},
		{ID: "nilcap", CapabilityID: nil, Hours: 4},
		{ID: "live", CapabilityID: &a, Hours: 4, Capability: &api.ExpandedCapability{ID: "cap-a", Name: "PM"}},
	}
	got := aggregateDerivedCapabilities(rows)
	if len(got) != 1 || got[0].UsageCount != 1 {
		t.Fatalf("expected single cap-a row with usage 1, got %+v", got)
	}
}

func TestCapabilitiesList_DerivedWarningOnNonEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"a1","userId":"u1","projectId":"019d27a4-0000-7000-8000-000000000000","capabilityId":"cap-1","date":"2026-05-21","hours":4,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","capability":{"id":"cap-1","name":"Project Management"}}
		]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"capabilities", "list",
		"--for-project", "019d27a4-0000-7000-8000-000000000000",
	)
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "derived from assignment history") {
		t.Fatalf("expected derived-view warning on stderr, got: %q", stderr)
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 derived row, got %d: %v", len(rows), rows)
	}
	if rows[0]["source"] != capabilitySourceDerived {
		t.Fatalf("expected source=%s, got %v", capabilitySourceDerived, rows[0]["source"])
	}
	if rows[0]["name"] != "Project Management" {
		t.Fatalf("expected expanded capability name, got %v", rows[0]["name"])
	}
}

func TestCapabilitiesList_EmptyEmitsInformationalNote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"capabilities", "list",
		"--for-project", "019d27a4-0000-7000-8000-000000000000",
	)
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "no capabilities found via assignment history") {
		t.Fatalf("expected informational note on empty, got: %q", stderr)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected empty array stdout, got: %q", stdout)
	}
}

func TestCapabilitiesList_RequiresForProject(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://example.invalid")

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"capabilities", "list",
	)
	if err == nil {
		t.Fatalf("expected error when --for-project missing")
	}
}

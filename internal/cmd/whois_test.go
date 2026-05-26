package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/supervisible/supervisible-cli/internal/api"
)

func TestBuildWhoisReport_FiltersZeroHourAssignments(t *testing.T) {
	user := api.User{ID: "u1", Email: "u1@test.com", DefaultAvailability: 40}
	cap1 := "cap-1"
	cap2 := "cap-2"
	assignments := []api.Assignment{
		{ID: "a1", UserID: "u1", ProjectID: "p1", CapabilityID: &cap1, Date: "2026-05-18", Hours: 0},
		{ID: "a2", UserID: "u1", ProjectID: "p1", CapabilityID: &cap2, Date: "2026-05-18", Hours: 2},
	}
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := weekStart.AddDate(0, 0, 6)

	report := buildWhoisReport(user, assignments, nil, weekStart, weekEnd)

	if len(report.Assignments) != 1 {
		t.Fatalf("expected 1 assignment after zombie filter, got %d", len(report.Assignments))
	}
	if report.Assignments[0].ID != "a2" {
		t.Fatalf("expected a2 to survive, got %s", report.Assignments[0].ID)
	}
	if report.WeekSummary.AssignedHours != 2 {
		t.Fatalf("expected assignedHours=2 (zero row excluded), got %d", report.WeekSummary.AssignedHours)
	}
}

func TestBuildWhoisReport_PopulatesActionableIDs(t *testing.T) {
	user := api.User{ID: "u1", Email: "u1@test.com", DefaultAvailability: 40}
	cap := "cap-xyz"
	assignments := []api.Assignment{
		{ID: "a-abc", UserID: "u1", ProjectID: "p-123", CapabilityID: &cap, Date: "2026-05-18", Hours: 4},
	}
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := weekStart.AddDate(0, 0, 6)

	report := buildWhoisReport(user, assignments, nil, weekStart, weekEnd)

	if len(report.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(report.Assignments))
	}
	got := report.Assignments[0]
	if got.ID != "a-abc" || got.ProjectID != "p-123" || got.CapabilityID != "cap-xyz" {
		t.Fatalf("missing actionable IDs: %+v", got)
	}
}

func TestWhoisWeeksFlag_RejectsOutOfBounds(t *testing.T) {
	// Validation fires before any API call, so no stub needed.
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://example.invalid")

	for _, weeks := range []string{"0", "13", "-1", "100"} {
		t.Run("weeks="+weeks, func(t *testing.T) {
			_, _, err := executeCLI(t,
				"--config", testConfigPath(t),
				"whois", "alice@test.com",
				"--weeks", weeks,
			)
			if err == nil {
				t.Fatalf("expected validation error for --weeks=%s, got nil", weeks)
			}
		})
	}
}

func TestBuildWhoisReport_WeekSummaryIgnoresFutureWeeks(t *testing.T) {
	// Regression: with --weeks N, the assignment fetch spans the full window but
	// WeekSummary must only count the *current* week. Caught live against dev:
	// Juan Méndez --weeks 4 showed assignedHours=97 when capacity reported 27h
	// this week.
	user := api.User{ID: "u1", Email: "u1@test.com", DefaultAvailability: 40}
	cap := "cap-1"
	assignments := []api.Assignment{
		{ID: "this-week-1", UserID: "u1", ProjectID: "p1", CapabilityID: &cap, Date: "2026-05-18", Hours: 8},
		{ID: "this-week-2", UserID: "u1", ProjectID: "p1", CapabilityID: &cap, Date: "2026-05-22", Hours: 4},
		{ID: "next-week", UserID: "u1", ProjectID: "p1", CapabilityID: &cap, Date: "2026-05-26", Hours: 20},
		{ID: "month-out", UserID: "u1", ProjectID: "p1", CapabilityID: &cap, Date: "2026-06-15", Hours: 30},
	}
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := weekStart.AddDate(0, 0, 6)

	report := buildWhoisReport(user, assignments, nil, weekStart, weekEnd)

	if len(report.Assignments) != 4 {
		t.Fatalf("expected all 4 assignments in the array, got %d", len(report.Assignments))
	}
	if report.WeekSummary.AssignedHours != 12 {
		t.Fatalf("WeekSummary.AssignedHours = %d, want 12 (only this-week rows count)", report.WeekSummary.AssignedHours)
	}
}

func TestBuildWhoisReport_PopulatesClientFromExpandedAssignment(t *testing.T) {
	user := api.User{ID: "u1", Email: "u1@test.com", DefaultAvailability: 40}
	cap := "cap-1"
	assignments := []api.Assignment{
		{
			ID:           "a1",
			UserID:       "u1",
			ProjectID:    "p1",
			CapabilityID: &cap,
			Date:         "2026-05-18",
			Hours:        4,
			Project:      &api.ExpandedProject{ID: "p1", Name: "Marketplace"},
			Client:       &api.ExpandedClient{ID: "c1", CompanyName: "EdVisorly"},
		},
		{
			ID:           "a2",
			UserID:       "u1",
			ProjectID:    "p2",
			CapabilityID: &cap,
			Date:         "2026-05-18",
			Hours:        2,
			Project:      &api.ExpandedProject{ID: "p2", Name: "Avask Web"},
			// No Client expansion — should leave Client nil.
		},
	}
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := weekStart.AddDate(0, 0, 6)

	report := buildWhoisReport(user, assignments, nil, weekStart, weekEnd)

	if len(report.Assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(report.Assignments))
	}
	if report.Assignments[0].Client == nil || report.Assignments[0].Client.Name != "EdVisorly" {
		t.Fatalf("expected client EdVisorly on a1, got %+v", report.Assignments[0].Client)
	}
	if report.Assignments[1].Client != nil {
		t.Fatalf("expected nil client on a2 (no expand), got %+v", report.Assignments[1].Client)
	}
}

func TestWhoisAssignment_JSONOmitsNilClient(t *testing.T) {
	a := WhoisAssignment{ID: "a1", ProjectID: "p1", Date: "2026-05-21", Hours: 4}
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "\"client\"") {
		t.Fatalf("expected client field omitted, got %s", raw)
	}
}

func TestWhoisAssignment_JSONIncludesClientWhenSet(t *testing.T) {
	a := WhoisAssignment{
		ID: "a1", ProjectID: "p1", Date: "2026-05-21", Hours: 4,
		Client: &WhoisClient{ID: "c1", Name: "EdVisorly"},
	}
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"client":{"id":"c1","name":"EdVisorly"}`) {
		t.Fatalf("expected client embedded, got %s", raw)
	}
}

func TestBuildWhoisReport_NilCapabilityIsEmptyString(t *testing.T) {
	user := api.User{ID: "u1", Email: "u1@test.com", DefaultAvailability: 40}
	assignments := []api.Assignment{
		{ID: "a1", UserID: "u1", ProjectID: "p1", CapabilityID: nil, Date: "2026-05-18", Hours: 3},
	}
	weekStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := weekStart.AddDate(0, 0, 6)

	report := buildWhoisReport(user, assignments, nil, weekStart, weekEnd)

	if report.Assignments[0].CapabilityID != "" {
		t.Fatalf("expected empty capabilityId for nil pointer, got %q", report.Assignments[0].CapabilityID)
	}
}

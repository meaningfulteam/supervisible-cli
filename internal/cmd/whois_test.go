package cmd

import (
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

package cmd

import (
	"testing"
	"time"

	"github.com/supervisible/supervisible-cli/internal/api"
)

func TestComputeCapacity_NoAssignmentsNoTimeOff(t *testing.T) {
	users := []api.User{
		{ID: "u1", Name: ptr("Alice"), Email: "alice@test.com", DefaultAvailability: 40, IsActive: true},
	}
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(users, nil, nil, weekStart, weekEnd, 21, 2026)

	if len(report.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(report.Users))
	}
	u := report.Users[0]
	if u.AssignedHours != 0 {
		t.Errorf("expected 0 assigned, got %d", u.AssignedHours)
	}
	if u.TimeOffHours != 0 {
		t.Errorf("expected 0 timeoff, got %d", u.TimeOffHours)
	}
	if u.AvailableHours != 40 {
		t.Errorf("expected 40 available, got %d", u.AvailableHours)
	}
	if u.FreeHours != 40 {
		t.Errorf("expected 40 free, got %d", u.FreeHours)
	}
}

func TestComputeCapacity_AssignedHours(t *testing.T) {
	users := []api.User{
		{ID: "u1", Name: ptr("Alice"), Email: "alice@test.com", DefaultAvailability: 40, IsActive: true},
	}
	assignments := []api.Assignment{
		{ID: "a1", UserID: "u1", ProjectID: "p1", Date: "2026-05-19", Hours: 20, Project: &api.ExpandedProject{ID: "p1", Name: "Aplazo"}},
		{ID: "a2", UserID: "u1", ProjectID: "p2", Date: "2026-05-20", Hours: 12, Project: &api.ExpandedProject{ID: "p2", Name: "Zetta"}},
	}
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(users, assignments, nil, weekStart, weekEnd, 21, 2026)

	u := report.Users[0]
	if u.AssignedHours != 32 {
		t.Errorf("expected 32 assigned, got %d", u.AssignedHours)
	}
	if u.FreeHours != 8 {
		t.Errorf("expected 8 free, got %d", u.FreeHours)
	}
	if len(u.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(u.Projects))
	}
	// Sorted by hours descending
	if u.Projects[0].Name != "Aplazo" || u.Projects[0].Hours != 20 {
		t.Errorf("expected Aplazo 20h first, got %s %dh", u.Projects[0].Name, u.Projects[0].Hours)
	}
	if u.Projects[1].Name != "Zetta" || u.Projects[1].Hours != 12 {
		t.Errorf("expected Zetta 12h second, got %s %dh", u.Projects[1].Name, u.Projects[1].Hours)
	}
}

func TestComputeCapacity_TimeOff_FullDays(t *testing.T) {
	users := []api.User{
		{ID: "u1", Name: ptr("Alice"), Email: "alice@test.com", DefaultAvailability: 40, IsActive: true},
	}
	// 2 business days off (Wed-Thu), availability=0
	timeOff := []api.TimeOffRequest{
		{
			ID:            "to1",
			UserID:        "u1",
			TimeOffTypeID: "tt1",
			StartDate:     "2026-05-20", // Wednesday
			EndDate:       "2026-05-21", // Thursday
			Availability:  0,
			Status:        "approved",
			TimeOffType:   &api.ExpandedTimeOffType{ID: "tt1", Name: "Vacation"},
		},
	}
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(users, nil, timeOff, weekStart, weekEnd, 21, 2026)

	u := report.Users[0]
	// 40h/week = 8h/day. 2 days off at 0 availability = 16h time-off
	if u.TimeOffHours != 16 {
		t.Errorf("expected 16 timeoff hours, got %d", u.TimeOffHours)
	}
	if u.AvailableHours != 24 {
		t.Errorf("expected 24 available, got %d", u.AvailableHours)
	}
	if u.FreeHours != 24 {
		t.Errorf("expected 24 free (no assignments), got %d", u.FreeHours)
	}
	if len(u.TimeOff) != 1 {
		t.Fatalf("expected 1 time-off entry, got %d", len(u.TimeOff))
	}
	if u.TimeOff[0].Type != "Vacation" {
		t.Errorf("expected Vacation type, got %s", u.TimeOff[0].Type)
	}
}

func TestComputeCapacity_TimeOff_PartialDay(t *testing.T) {
	users := []api.User{
		{ID: "u1", Name: ptr("Alice"), Email: "alice@test.com", DefaultAvailability: 40, IsActive: true},
	}
	// 1 day, availability=4 (half day off)
	timeOff := []api.TimeOffRequest{
		{
			ID:            "to1",
			UserID:        "u1",
			TimeOffTypeID: "tt1",
			StartDate:     "2026-05-19",
			EndDate:       "2026-05-19",
			Availability:  4,
			Status:        "approved",
		},
	}
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(users, nil, timeOff, weekStart, weekEnd, 21, 2026)

	u := report.Users[0]
	// 8h/day default, 4h available → 4h time-off per day, 1 day = 4h
	if u.TimeOffHours != 4 {
		t.Errorf("expected 4 timeoff hours, got %d", u.TimeOffHours)
	}
	if u.AvailableHours != 36 {
		t.Errorf("expected 36 available, got %d", u.AvailableHours)
	}
}

func TestComputeCapacity_TimeOff_SpanningWeekend(t *testing.T) {
	users := []api.User{
		{ID: "u1", Name: ptr("Alice"), Email: "alice@test.com", DefaultAvailability: 40, IsActive: true},
	}
	// Time-off Thu to next Mon (Thu, Fri = 2 business days in this week)
	timeOff := []api.TimeOffRequest{
		{
			ID:            "to1",
			UserID:        "u1",
			TimeOffTypeID: "tt1",
			StartDate:     "2026-05-21", // Thursday
			EndDate:       "2026-05-25", // Monday (next week)
			Availability:  0,
			Status:        "approved",
		},
	}
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(users, nil, timeOff, weekStart, weekEnd, 21, 2026)

	u := report.Users[0]
	// Thu + Fri = 2 business days × 8h = 16h
	if u.TimeOffHours != 16 {
		t.Errorf("expected 16 timeoff hours (Thu+Fri only), got %d", u.TimeOffHours)
	}
}

func TestComputeCapacity_MultipleAssignmentsSameProject(t *testing.T) {
	users := []api.User{
		{ID: "u1", Name: ptr("Alice"), Email: "alice@test.com", DefaultAvailability: 40, IsActive: true},
	}
	assignments := []api.Assignment{
		{ID: "a1", UserID: "u1", ProjectID: "p1", Date: "2026-05-19", Hours: 8, Project: &api.ExpandedProject{ID: "p1", Name: "Aplazo"}},
		{ID: "a2", UserID: "u1", ProjectID: "p1", Date: "2026-05-20", Hours: 8, Project: &api.ExpandedProject{ID: "p1", Name: "Aplazo"}},
		{ID: "a3", UserID: "u1", ProjectID: "p1", Date: "2026-05-21", Hours: 8, Project: &api.ExpandedProject{ID: "p1", Name: "Aplazo"}},
	}
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(users, assignments, nil, weekStart, weekEnd, 21, 2026)

	u := report.Users[0]
	if u.AssignedHours != 24 {
		t.Errorf("expected 24 assigned, got %d", u.AssignedHours)
	}
	if len(u.Projects) != 1 {
		t.Fatalf("expected 1 project (aggregated), got %d", len(u.Projects))
	}
	if u.Projects[0].Hours != 24 {
		t.Errorf("expected 24h for aggregated project, got %d", u.Projects[0].Hours)
	}
}

func TestComputeCapacity_Overallocated(t *testing.T) {
	users := []api.User{
		{ID: "u1", Name: ptr("Alice"), Email: "alice@test.com", DefaultAvailability: 40, IsActive: true},
	}
	assignments := []api.Assignment{
		{ID: "a1", UserID: "u1", ProjectID: "p1", Date: "2026-05-19", Hours: 48, Project: &api.ExpandedProject{ID: "p1", Name: "Overload"}},
	}
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(users, assignments, nil, weekStart, weekEnd, 21, 2026)

	u := report.Users[0]
	if u.FreeHours != -8 {
		t.Errorf("expected -8 free (overallocated), got %d", u.FreeHours)
	}
}

func TestComputeCapacity_ReportMetadata(t *testing.T) {
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(nil, nil, nil, weekStart, weekEnd, 21, 2026)

	if report.Week != "2026-W21" {
		t.Errorf("expected 2026-W21, got %s", report.Week)
	}
	if report.StartDate != "2026-05-18" {
		t.Errorf("expected 2026-05-18, got %s", report.StartDate)
	}
	if report.EndDate != "2026-05-24" {
		t.Errorf("expected 2026-05-24, got %s", report.EndDate)
	}
	if report.Users == nil {
		t.Error("expected empty slice, got nil")
	}
}

func TestComputeCapacity_ProjectFallbackToID(t *testing.T) {
	users := []api.User{
		{ID: "u1", Name: ptr("Alice"), Email: "alice@test.com", DefaultAvailability: 40, IsActive: true},
	}
	// Assignment without expanded project
	assignments := []api.Assignment{
		{ID: "a1", UserID: "u1", ProjectID: "proj-uuid-123", Date: "2026-05-19", Hours: 8},
	}
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)

	report := computeCapacity(users, assignments, nil, weekStart, weekEnd, 21, 2026)

	if report.Users[0].Projects[0].Name != "proj-uuid-123" {
		t.Errorf("expected project ID as fallback, got %s", report.Users[0].Projects[0].Name)
	}
}

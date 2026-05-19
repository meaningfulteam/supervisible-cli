package cmd

import (
	"testing"
	"time"
)

func TestParseWeekFlag_ISOWeek(t *testing.T) {
	start, end, week, year, err := parseWeekFlag("2026-W21")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if year != 2026 || week != 21 {
		t.Fatalf("expected 2026-W21, got %d-W%d", year, week)
	}
	expectDate(t, "start", start, 2026, time.May, 18)
	expectDate(t, "end", end, 2026, time.May, 24)
}

func TestParseWeekFlag_Date(t *testing.T) {
	// Wednesday May 20, 2026 → should return Mon May 18 – Sun May 24
	start, end, week, year, err := parseWeekFlag("2026-05-20")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if year != 2026 || week != 21 {
		t.Fatalf("expected 2026-W21, got %d-W%d", year, week)
	}
	expectDate(t, "start", start, 2026, time.May, 18)
	expectDate(t, "end", end, 2026, time.May, 24)
}

func TestParseWeekFlag_Empty(t *testing.T) {
	start, end, week, year, err := parseWeekFlag("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be current week
	now := time.Now()
	expectedYear, expectedWeek := now.ISOWeek()
	if year != expectedYear || week != expectedWeek {
		t.Fatalf("expected %d-W%02d, got %d-W%02d", expectedYear, expectedWeek, year, week)
	}
	// start should be a Monday
	if start.Weekday() != time.Monday {
		t.Fatalf("expected start to be Monday, got %s", start.Weekday())
	}
	// end should be a Sunday
	if end.Weekday() != time.Sunday {
		t.Fatalf("expected end to be Sunday, got %s", end.Weekday())
	}
}

func TestParseWeekFlag_W01YearBoundary(t *testing.T) {
	// 2026-W01 starts on Dec 29, 2025
	start, _, week, year, err := parseWeekFlag("2026-W01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if year != 2026 || week != 1 {
		t.Fatalf("expected 2026-W01, got %d-W%d", year, week)
	}
	expectDate(t, "start", start, 2025, time.December, 29)
}

func TestParseWeekFlag_InvalidWeek53(t *testing.T) {
	// 2025 starts on Wednesday (not a leap year) → only 52 ISO weeks
	_, _, _, _, err := parseWeekFlag("2025-W53")
	if err == nil {
		t.Fatal("expected error for invalid week 53 in 2025")
	}
}

func TestParseWeekFlag_ValidWeek53(t *testing.T) {
	// 2026 starts on Thursday → has 53 ISO weeks
	start, _, week, year, err := parseWeekFlag("2026-W53")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if year != 2026 || week != 53 {
		t.Fatalf("expected 2026-W53, got %d-W%d", year, week)
	}
	expectDate(t, "start", start, 2026, time.December, 28)
}

func TestParseWeekFlag_Invalid(t *testing.T) {
	cases := []string{"invalid", "2026-W0", "2026-W00", "W21", "2026W21"}
	for _, input := range cases {
		_, _, _, _, err := parseWeekFlag(input)
		if err == nil {
			t.Errorf("expected error for input %q", input)
		}
	}
}

func TestFormatWeekHeader(t *testing.T) {
	start := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	got := formatWeekHeader(21, 2026, start, end)
	expected := "Week 21 (May 18–24, 2026)"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestFormatWeekHeader_CrossMonth(t *testing.T) {
	start := time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC)
	got := formatWeekHeader(22, 2026, start, end)
	expected := "Week 22 (May 25–31, 2026)"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}

	// Week that crosses month boundary
	start2 := time.Date(2026, time.June, 29, 0, 0, 0, 0, time.UTC)
	end2 := time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)
	got2 := formatWeekHeader(27, 2026, start2, end2)
	expected2 := "Week 27 (Jun 29 – Jul 5, 2026)"
	if got2 != expected2 {
		t.Fatalf("expected %q, got %q", expected2, got2)
	}
}

func TestFormatDate(t *testing.T) {
	d := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	got := formatDate(d)
	if got != "2026-05-18" {
		t.Fatalf("expected 2026-05-18, got %s", got)
	}
}

func TestBusinessDaysOverlap_FullWeek(t *testing.T) {
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC) // Monday
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)   // Sunday
	got := businessDaysOverlap(weekStart, weekEnd, weekStart, weekEnd)
	if got != 5 {
		t.Fatalf("expected 5 business days, got %d", got)
	}
}

func TestBusinessDaysOverlap_PartialWeek(t *testing.T) {
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	// Time off Wed-Fri (3 business days)
	toStart := time.Date(2026, time.May, 20, 0, 0, 0, 0, time.UTC) // Wednesday
	toEnd := time.Date(2026, time.May, 22, 0, 0, 0, 0, time.UTC)   // Friday
	got := businessDaysOverlap(weekStart, weekEnd, toStart, toEnd)
	if got != 3 {
		t.Fatalf("expected 3 business days, got %d", got)
	}
}

func TestBusinessDaysOverlap_WeekendOnly(t *testing.T) {
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	// Sat-Sun only
	toStart := time.Date(2026, time.May, 23, 0, 0, 0, 0, time.UTC) // Saturday
	toEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)   // Sunday
	got := businessDaysOverlap(weekStart, weekEnd, toStart, toEnd)
	if got != 0 {
		t.Fatalf("expected 0 business days for weekend, got %d", got)
	}
}

func TestBusinessDaysOverlap_NoOverlap(t *testing.T) {
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	toStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	toEnd := time.Date(2026, time.June, 5, 0, 0, 0, 0, time.UTC)
	got := businessDaysOverlap(weekStart, weekEnd, toStart, toEnd)
	if got != 0 {
		t.Fatalf("expected 0 for no overlap, got %d", got)
	}
}

func TestBusinessDaysOverlap_SpansWeek(t *testing.T) {
	weekStart := time.Date(2026, time.May, 18, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, time.May, 24, 0, 0, 0, 0, time.UTC)
	// Time-off spans beyond the week on both sides
	toStart := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	toEnd := time.Date(2026, time.May, 28, 0, 0, 0, 0, time.UTC)
	got := businessDaysOverlap(weekStart, weekEnd, toStart, toEnd)
	if got != 5 {
		t.Fatalf("expected 5 for full overlap, got %d", got)
	}
}

func expectDate(t *testing.T, label string, got time.Time, year int, month time.Month, day int) {
	t.Helper()
	if got.Year() != year || got.Month() != month || got.Day() != day {
		t.Fatalf("%s: expected %d-%02d-%02d, got %s", label, year, month, day, got.Format("2006-01-02"))
	}
}

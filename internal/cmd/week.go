package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var isoWeekRegex = regexp.MustCompile(`^(\d{4})-W(\d{2})$`)

// parseWeekFlag parses a --week flag value into a date range.
// Accepts: empty (current week), ISO week "YYYY-Www", or date "YYYY-MM-DD".
// Returns the Monday and Sunday of the target week.
func parseWeekFlag(value string) (weekStart, weekEnd time.Time, isoWeek, isoYear int, err error) {
	if value == "" {
		now := time.Now()
		weekStart = mondayOf(now)
		weekEnd = weekStart.AddDate(0, 0, 6)
		isoYear, isoWeek = now.ISOWeek()
		return
	}

	if m := isoWeekRegex.FindStringSubmatch(value); m != nil {
		year, _ := strconv.Atoi(m[1])
		week, _ := strconv.Atoi(m[2])
		if week < 1 || week > isoWeeksInYear(year) {
			err = fmt.Errorf("invalid ISO week: %s (year %d has %d weeks)", value, year, isoWeeksInYear(year))
			return
		}
		weekStart = mondayOfISOWeek(year, week)
		weekEnd = weekStart.AddDate(0, 0, 6)
		isoYear = year
		isoWeek = week
		return
	}

	t, parseErr := time.Parse("2006-01-02", value)
	if parseErr == nil {
		weekStart = mondayOf(t)
		weekEnd = weekStart.AddDate(0, 0, 6)
		isoYear, isoWeek = t.ISOWeek()
		return
	}

	err = fmt.Errorf("invalid --week value %q: expected YYYY-Www (e.g. 2026-W21) or YYYY-MM-DD", value)
	return
}

// mondayOf returns the Monday of the week containing t.
func mondayOf(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	return t.AddDate(0, 0, -int(weekday-time.Monday))
}

// mondayOfISOWeek returns the Monday of the given ISO year/week.
// Jan 4 is always in ISO week 1. We find week 1's Monday, then offset.
func mondayOfISOWeek(year, week int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	week1Monday := mondayOf(jan4)
	return week1Monday.AddDate(0, 0, (week-1)*7)
}

// isoWeeksInYear returns the number of ISO weeks in a given year (52 or 53).
func isoWeeksInYear(year int) int {
	dec28 := time.Date(year, time.December, 28, 0, 0, 0, 0, time.UTC)
	_, w := dec28.ISOWeek()
	return w
}

// formatWeekHeader returns a human-readable week header like "Week 21 (May 18–24, 2026)".
func formatWeekHeader(isoWeek, isoYear int, weekStart, weekEnd time.Time) string {
	startMonth := weekStart.Format("Jan")
	endMonth := weekEnd.Format("Jan")

	if startMonth == endMonth {
		return fmt.Sprintf("Week %d (%s %d–%d, %d)",
			isoWeek, startMonth, weekStart.Day(), weekEnd.Day(), isoYear)
	}
	return fmt.Sprintf("Week %d (%s %d – %s %d, %d)",
		isoWeek, startMonth, weekStart.Day(), endMonth, weekEnd.Day(), isoYear)
}

// formatDate formats a time.Time as YYYY-MM-DD.
func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// businessDaysOverlap counts the number of business days (Mon-Fri) in the
// intersection of [rangeStart, rangeEnd] and [periodStart, periodEnd].
// All dates are inclusive.
func businessDaysOverlap(rangeStart, rangeEnd, periodStart, periodEnd time.Time) int {
	// Normalize to date-only (strip time)
	rangeStart = truncateToDate(rangeStart)
	rangeEnd = truncateToDate(rangeEnd)
	periodStart = truncateToDate(periodStart)
	periodEnd = truncateToDate(periodEnd)

	// Find overlap
	start := rangeStart
	if periodStart.After(start) {
		start = periodStart
	}
	end := rangeEnd
	if periodEnd.Before(end) {
		end = periodEnd
	}

	if start.After(end) {
		return 0
	}

	count := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		wd := d.Weekday()
		if wd >= time.Monday && wd <= time.Friday {
			count++
		}
	}
	return count
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

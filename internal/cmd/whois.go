package cmd

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/output"
)

// --- Output types for whois command ---

type WhoisAssignment struct {
	Project string `json:"project"`
	Date    string `json:"date"`
	Hours   int    `json:"hours"`
}

type WhoisTimeOff struct {
	Type      string `json:"type"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Status    string `json:"status"`
}

type WhoisWeekSummary struct {
	AssignedHours  int `json:"assignedHours"`
	AvailableHours int `json:"availableHours"`
	FreeHours      int `json:"freeHours"`
}

type WhoisUser struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email string  `json:"email"`
	Image *string `json:"image,omitempty"`
}

type WhoisReport struct {
	User        WhoisUser         `json:"user"`
	Assignments []WhoisAssignment `json:"assignments"`
	TimeOff     []WhoisTimeOff    `json:"timeOff"`
	WeekSummary WhoisWeekSummary  `json:"weekSummary"`
}

func newWhoisCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whois <name-or-email>",
		Short: "Look up a team member by name or email",
		Long: `Resolve a team member and show their current assignments and upcoming time-off.

Matches by case-insensitive substring on name, or exact match on email
(if the input contains @).

Examples:
  supervisible whois juan
  supervisible whois juan@m8l.com --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			query := args[0]
			ctx := cmd.Context()

			// Fetch all users
			userQuery := url.Values{}
			userQuery.Set("limit", fetchLimit)
			userQuery = app.ResolvedQuery("GET", "/users", userQuery)

			var allUsers []api.User
			if err := client.Do(ctx, "GET", "/users", userQuery, nil, &allUsers); err != nil {
				return fmt.Errorf("fetch users: %w", err)
			}

			// Match user
			matched := matchUser(allUsers, query)
			if len(matched) == 0 {
				return fmt.Errorf("no user found matching %q", query)
			}
			if len(matched) > 1 {
				names := make([]string, 0, len(matched))
				for _, u := range matched {
					names = append(names, output.CoalesceString(u.Name))
				}
				return fmt.Errorf("multiple users match %q: %s. Be more specific", query, strings.Join(names, ", "))
			}

			user := matched[0]

			// Get this week's date range
			weekStart, weekEnd, isoWeek, isoYear, err := parseWeekFlag("")
			if err != nil {
				return err
			}
			startStr := formatDate(weekStart)
			endStr := formatDate(weekEnd)

			// Fetch assignments for this week
			assignQuery := url.Values{}
			assignQuery.Set("user_id", user.ID)
			assignQuery.Set("start_date", startStr)
			assignQuery.Set("end_date", endStr)
			assignQuery.Set("limit", fetchLimit)
			assignQuery.Set("expand", "project")
			assignQuery = app.ResolvedQuery("GET", "/assignments", assignQuery)

			var assignments []api.Assignment
			if err := client.Do(ctx, "GET", "/assignments", assignQuery, nil, &assignments); err != nil {
				return fmt.Errorf("fetch assignments: %w", err)
			}

			// Fetch upcoming time-off (from today onwards)
			today := formatDate(time.Now())
			toQuery := url.Values{}
			toQuery.Set("user_id", user.ID)
			toQuery.Set("start_date", today)
			toQuery.Set("limit", fetchLimit)
			toQuery.Set("expand", "timeOffType")
			toQuery = app.ResolvedQuery("GET", "/time-off", toQuery)

			var timeOff []api.TimeOffRequest
			if err := client.Do(ctx, "GET", "/time-off", toQuery, nil, &timeOff); err != nil {
				return fmt.Errorf("fetch time-off: %w", err)
			}

			// Build report
			report := buildWhoisReport(user, assignments, timeOff, weekStart, weekEnd)

			if app.Printer().IsJSON() {
				return app.PrintData(report)
			}

			return printWhoisProfile(app.Printer(), report, isoWeek, isoYear, weekStart, weekEnd)
		},
	}

	return cmd
}

func matchUser(users []api.User, query string) []api.User {
	queryLower := strings.ToLower(strings.TrimSpace(query))
	var matches []api.User

	if strings.Contains(queryLower, "@") {
		// Exact email match
		for _, u := range users {
			if strings.ToLower(u.Email) == queryLower {
				matches = append(matches, u)
			}
		}
	} else {
		// Substring match on name
		for _, u := range users {
			name := strings.ToLower(output.CoalesceString(u.Name))
			if strings.Contains(name, queryLower) {
				matches = append(matches, u)
			}
		}
	}

	return matches
}

func buildWhoisReport(user api.User, assignments []api.Assignment, timeOff []api.TimeOffRequest, weekStart, weekEnd time.Time) WhoisReport {
	report := WhoisReport{
		User: WhoisUser{
			ID:    user.ID,
			Name:  output.CoalesceString(user.Name),
			Email: user.Email,
			Image: user.Image,
		},
		Assignments: make([]WhoisAssignment, 0, len(assignments)),
		TimeOff:     make([]WhoisTimeOff, 0, len(timeOff)),
	}

	assignedHours := 0
	for _, a := range assignments {
		projName := a.ProjectID
		if a.Project != nil {
			projName = a.Project.Name
		}
		report.Assignments = append(report.Assignments, WhoisAssignment{
			Project: projName,
			Date:    a.Date,
			Hours:   a.Hours,
		})
		assignedHours += a.Hours
	}

	for _, to := range timeOff {
		typeName := to.TimeOffTypeID
		if to.TimeOffType != nil {
			typeName = to.TimeOffType.Name
		}
		report.TimeOff = append(report.TimeOff, WhoisTimeOff{
			Type:      typeName,
			StartDate: to.StartDate,
			EndDate:   to.EndDate,
			Status:    to.Status,
		})
	}

	// Calculate time-off hours for this week only (approved)
	timeOffHours := 0
	dailyDefault := user.DefaultAvailability / 5
	for _, to := range timeOff {
		if to.Status != "approved" {
			continue
		}
		toStart, err1 := time.Parse("2006-01-02", to.StartDate)
		toEnd, err2 := time.Parse("2006-01-02", to.EndDate)
		if err1 != nil || err2 != nil {
			continue
		}
		days := businessDaysOverlap(weekStart, weekEnd, toStart, toEnd)
		hoursPerDay := dailyDefault - to.Availability
		if hoursPerDay < 0 {
			hoursPerDay = 0
		}
		timeOffHours += days * hoursPerDay
	}

	available := user.DefaultAvailability - timeOffHours
	report.WeekSummary = WhoisWeekSummary{
		AssignedHours:  assignedHours,
		AvailableHours: available,
		FreeHours:      available - assignedHours,
	}

	return report
}

func printWhoisProfile(p *output.Printer, report WhoisReport, isoWeek, isoYear int, weekStart, weekEnd time.Time) error {
	p.PrintMessage("%s (%s)\n", report.User.Name, report.User.Email)

	header := formatWeekHeader(isoWeek, isoYear, weekStart, weekEnd)
	p.PrintMessage("This Week — %s", header)
	p.PrintMessage("  Assigned: %dh / %dh available (%dh free)",
		report.WeekSummary.AssignedHours,
		report.WeekSummary.AvailableHours,
		report.WeekSummary.FreeHours)

	if len(report.Assignments) > 0 {
		// Aggregate by project for display
		projectMap := make(map[string]int)
		for _, a := range report.Assignments {
			projectMap[a.Project] += a.Hours
		}
		parts := make([]string, 0, len(projectMap))
		for name, hours := range projectMap {
			parts = append(parts, fmt.Sprintf("%s (%dh)", name, hours))
		}
		p.PrintMessage("  Projects: %s", strings.Join(parts, ", "))
	}

	p.PrintMessage("")
	if len(report.TimeOff) > 0 {
		p.PrintMessage("Upcoming Time Off")
		for _, to := range report.TimeOff {
			p.PrintMessage("  %s: %s to %s (%s)", to.Type, to.StartDate, to.EndDate, to.Status)
		}
	} else {
		p.PrintMessage("Upcoming Time Off: None")
	}

	return nil
}

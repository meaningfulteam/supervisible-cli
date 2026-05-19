package cmd

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/output"
)

// --- Output types for capacity/bench commands ---

type ProjectHours struct {
	Name  string `json:"name"`
	Hours int    `json:"hours"`
}

type TimeOffEntry struct {
	Type      string `json:"type"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Status    string `json:"status"`
}

type UserCapacity struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Email               string         `json:"email"`
	DefaultAvailability int            `json:"defaultAvailability"`
	AssignedHours       int            `json:"assignedHours"`
	TimeOffHours        int            `json:"timeOffHours"`
	AvailableHours      int            `json:"availableHours"`
	FreeHours           int            `json:"freeHours"`
	Projects            []ProjectHours `json:"projects"`
	TimeOff             []TimeOffEntry `json:"timeOff"`
}

type CapacityReport struct {
	Week      string         `json:"week"`
	StartDate string         `json:"startDate"`
	EndDate   string         `json:"endDate"`
	Users     []UserCapacity `json:"users"`
}

// computeCapacity takes raw API data and produces a capacity report.
// Users should be pre-filtered to active only. Assignments should be
// pre-filtered to the target week. Time-off should be pre-filtered to
// approved + overlapping the target week.
func computeCapacity(
	users []api.User,
	assignments []api.Assignment,
	timeOff []api.TimeOffRequest,
	weekStart, weekEnd time.Time,
	isoWeek, isoYear int,
) CapacityReport {
	// Index assignments by user ID
	assignmentsByUser := make(map[string][]api.Assignment)
	for _, a := range assignments {
		assignmentsByUser[a.UserID] = append(assignmentsByUser[a.UserID], a)
	}

	// Index time-off by user ID
	timeOffByUser := make(map[string][]api.TimeOffRequest)
	for _, to := range timeOff {
		timeOffByUser[to.UserID] = append(timeOffByUser[to.UserID], to)
	}

	result := CapacityReport{
		Week:      formatISOWeek(isoYear, isoWeek),
		StartDate: formatDate(weekStart),
		EndDate:   formatDate(weekEnd),
		Users:     make([]UserCapacity, 0, len(users)),
	}

	for _, user := range users {
		uc := UserCapacity{
			ID:                  user.ID,
			Name:                output.CoalesceString(user.Name),
			Email:               user.Email,
			DefaultAvailability: user.DefaultAvailability,
			Projects:            make([]ProjectHours, 0),
			TimeOff:             make([]TimeOffEntry, 0),
		}

		// Sum assigned hours, group by project
		projectMap := make(map[string]*ProjectHours)
		for _, a := range assignmentsByUser[user.ID] {
			projName := a.ProjectID // fallback to ID
			if a.Project != nil {
				projName = a.Project.Name
			}
			if ph, ok := projectMap[projName]; ok {
				ph.Hours += a.Hours
			} else {
				projectMap[projName] = &ProjectHours{Name: projName, Hours: a.Hours}
			}
			uc.AssignedHours += a.Hours
		}
		for _, ph := range projectMap {
			uc.Projects = append(uc.Projects, *ph)
		}
		// Sort projects by hours descending for consistent output
		sort.Slice(uc.Projects, func(i, j int) bool {
			return uc.Projects[i].Hours > uc.Projects[j].Hours
		})

		// Calculate time-off hours
		dailyDefault := user.DefaultAvailability / 5 // hours per business day
		for _, to := range timeOffByUser[user.ID] {
			toStart, err1 := time.Parse("2006-01-02", to.StartDate)
			toEnd, err2 := time.Parse("2006-01-02", to.EndDate)
			if err1 != nil || err2 != nil {
				continue
			}

			days := businessDaysOverlap(weekStart, weekEnd, toStart, toEnd)
			if days > 0 {
				hoursPerDay := dailyDefault - to.Availability
				if hoursPerDay < 0 {
					hoursPerDay = 0
				}
				uc.TimeOffHours += days * hoursPerDay

				typeName := to.TimeOffTypeID
				if to.TimeOffType != nil {
					typeName = to.TimeOffType.Name
				}
				uc.TimeOff = append(uc.TimeOff, TimeOffEntry{
					Type:      typeName,
					StartDate: to.StartDate,
					EndDate:   to.EndDate,
					Status:    to.Status,
				})
			}
		}

		uc.AvailableHours = user.DefaultAvailability - uc.TimeOffHours
		uc.FreeHours = uc.AvailableHours - uc.AssignedHours

		result.Users = append(result.Users, uc)
	}

	return result
}

func formatISOWeek(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}

// --- Command wiring ---

const fetchLimit = "200"

func newCapacityCommand() *cobra.Command {
	var weekFlag string

	cmd := &cobra.Command{
		Use:   "capacity",
		Short: "Show team capacity for a week",
		Long: `Show assigned hours, available hours, and free capacity for all active team members.

Fetches users, assignments, and approved time-off for the target week, then
computes per-user capacity. Defaults to the current week.

Examples:
  supervisible capacity
  supervisible capacity --week 2026-W21
  supervisible capacity --week 2026-05-18 --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			report, header, err := fetchCapacityData(cmd, app, weekFlag)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(report)
			}

			return printCapacityTable(app.Printer(), report, header)
		},
	}

	cmd.Flags().StringVar(&weekFlag, "week", "", "Target week: YYYY-Www or YYYY-MM-DD (default: current week)")
	return cmd
}

// fetchCapacityData fetches users, assignments, and time-off from the API
// and computes a CapacityReport. Shared by capacity and bench commands.
func fetchCapacityData(cmd *cobra.Command, app *App, weekFlag string) (*CapacityReport, string, error) {
	weekStart, weekEnd, isoWeek, isoYear, err := parseWeekFlag(weekFlag)
	if err != nil {
		return nil, "", err
	}

	client, err := app.RequireClient()
	if err != nil {
		return nil, "", err
	}

	startStr := formatDate(weekStart)
	endStr := formatDate(weekEnd)
	ctx := cmd.Context()

	// Fetch users
	userQuery := url.Values{}
	userQuery.Set("limit", fetchLimit)
	userQuery = app.ResolvedQuery("GET", "/users", userQuery)

	var allUsers []api.User
	if err := client.Do(ctx, "GET", "/users", userQuery, nil, &allUsers); err != nil {
		return nil, "", fmt.Errorf("fetch users: %w", err)
	}

	// Filter to active users
	var users []api.User
	for _, u := range allUsers {
		if u.IsActive {
			users = append(users, u)
		}
	}

	// Fetch assignments for the week with project expand
	assignQuery := url.Values{}
	assignQuery.Set("start_date", startStr)
	assignQuery.Set("end_date", endStr)
	assignQuery.Set("limit", fetchLimit)
	assignQuery.Set("expand", "project")
	assignQuery = app.ResolvedQuery("GET", "/assignments", assignQuery)

	var assignments []api.Assignment
	if err := client.Do(ctx, "GET", "/assignments", assignQuery, nil, &assignments); err != nil {
		return nil, "", fmt.Errorf("fetch assignments: %w", err)
	}

	// Fetch approved time-off overlapping the week
	toQuery := url.Values{}
	toQuery.Set("start_date", startStr)
	toQuery.Set("end_date", endStr)
	toQuery.Set("status", "approved")
	toQuery.Set("limit", fetchLimit)
	toQuery.Set("expand", "timeOffType")
	toQuery = app.ResolvedQuery("GET", "/time-off", toQuery)

	var timeOff []api.TimeOffRequest
	if err := client.Do(ctx, "GET", "/time-off", toQuery, nil, &timeOff); err != nil {
		return nil, "", fmt.Errorf("fetch time-off: %w", err)
	}

	report := computeCapacity(users, assignments, timeOff, weekStart, weekEnd, isoWeek, isoYear)
	header := formatWeekHeader(isoWeek, isoYear, weekStart, weekEnd)
	return &report, header, nil
}

func printCapacityTable(p *output.Printer, report *CapacityReport, header string) error {
	p.PrintMessage("Capacity — %s\n", header)

	rows := make([][]string, 0, len(report.Users))
	for _, u := range report.Users {
		projects := formatProjectList(u.Projects)
		rows = append(rows, []string{
			u.Name,
			fmt.Sprintf("%dh", u.AssignedHours),
			fmt.Sprintf("%dh", u.AvailableHours),
			fmt.Sprintf("%dh", u.FreeHours),
			projects,
		})
	}
	return p.Table([]string{"NAME", "ASSIGNED", "AVAILABLE", "FREE", "PROJECTS"}, rows)
}

func formatProjectList(projects []ProjectHours) string {
	parts := make([]string, 0, len(projects))
	for _, p := range projects {
		parts = append(parts, fmt.Sprintf("%s (%dh)", p.Name, p.Hours))
	}
	return strings.Join(parts, ", ")
}

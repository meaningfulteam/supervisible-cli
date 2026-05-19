package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newBenchCommand() *cobra.Command {
	var (
		weekFlag string
		minHours int
	)

	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Show team members with free capacity",
		Long: `Show team members who have free hours above a threshold for a given week.

Same data as 'capacity', but filtered to users with freeHours >= --min-hours
and sorted by most free hours first.`,
		Example: `  # Current-week bench
  supervisible bench

  # Specific week, higher threshold
  supervisible bench --week 2026-W21 --min-hours 16

  # JSON
  supervisible bench --json`,
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

			// Filter to users with free hours >= minHours
			filtered := make([]UserCapacity, 0)
			for _, u := range report.Users {
				if u.FreeHours >= minHours {
					filtered = append(filtered, u)
				}
			}

			// Sort by most free hours first
			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].FreeHours > filtered[j].FreeHours
			})

			report.Users = filtered

			if app.Printer().IsJSON() {
				return app.PrintData(report)
			}

			if len(filtered) == 0 {
				w := app.Printer().Stdout()
				fmt.Fprintf(w, "Bench — %s\n\n", header)
				fmt.Fprintf(w, "No team members with >= %dh free capacity.\n", minHours)
				return nil
			}

			header = fmt.Sprintf("%s (>= %dh free)", header, minHours)
			return printCapacityTable(app.Printer(), report, header)
		},
	}

	cmd.Flags().StringVar(&weekFlag, "week", "", "Target week: YYYY-Www or YYYY-MM-DD (default: current week)")
	cmd.Flags().IntVar(&minHours, "min-hours", 8, "Minimum free hours to include (default: 8)")
	return cmd
}

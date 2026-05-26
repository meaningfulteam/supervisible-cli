package cmd

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/inputs"
)

func newActualHoursCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actual-hours",
		Short: "Manage actual hour entries",
	}

	cmd.AddCommand(
		newActualHoursListCommand(),
		newActualHoursUpsertCommand(),
		newActualHoursDeleteCommand(),
	)

	return cmd
}

func newActualHoursListCommand() *cobra.Command {
	var (
		userID    string
		projectID string
		startDate string
		endDate   string
		limit     int
		offset    int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List actual hours",
		Example: `  # Actuals for one user over a range
  supervisible actual-hours list --user-id <uuid> \
    --start-date 2026-05-01 --end-date 2026-05-31 --json

  # Actuals for one project
  supervisible actual-hours list --project-id <uuid> --limit 100`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			baseQuery := url.Values{}
			if userID != "" {
				baseQuery.Set("user_id", userID)
			}
			if projectID != "" {
				baseQuery.Set("project_id", projectID)
			}
			if startDate != "" {
				baseQuery.Set("start_date", startDate)
			}
			if endDate != "" {
				baseQuery.Set("end_date", endDate)
			}
			baseQuery.Set("limit", strconv.Itoa(limit))
			baseQuery.Set("offset", strconv.Itoa(offset))

			var items []api.ActualHour
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "actual-hours list",
				Method:      "GET",
				Endpoint:    "/actual-hours",
				Query:       baseQuery,
				Out:         &items,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}

			if app.Printer().IsJSON() {
				return app.PrintData(items)
			}

			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.ID,
					item.UserID,
					item.ProjectID,
					item.Date,
					fmt.Sprintf("%d", item.Hours),
				})
			}
			return app.Printer().Table([]string{"ID", "USER", "PROJECT", "DATE", "HOURS"}, rows)
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "Filter by user ID")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Filter by project ID")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Pagination limit")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	return cmd
}

func newActualHoursUpsertCommand() *cobra.Command {
	var (
		jsonBody     string
		payload      string
		filePath     string
		userID       string
		projectID    string
		date         string
		hours        int
		capabilityID string
	)

	cmd := &cobra.Command{
		Use:   "upsert",
		Short: "Upsert actual hours",
		Long:  `Upsert actual hours via individual flags or bulk JSON.`,
		Example: `  # Single item via flags
  supervisible actual-hours upsert --user-id <uuid> --project-id <uuid> \
    --date 2026-03-06 --hours 5

  # Bulk via inline JSON
  supervisible actual-hours upsert --body '{"items":[...]}'

  # Bulk from a file
  supervisible actual-hours upsert --file payload.json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			var rawBody map[string]any

			if userID != "" {
				// Flag mode — single item
				if payload != "" || jsonBody != "" || filePath != "" {
					return fmt.Errorf("--user-id flag mode cannot be combined with --payload, --body, or --file")
				}
				if err := requireUUIDArg("user-id", userID); err != nil {
					return err
				}
				if err := requireUUIDArg("project-id", projectID); err != nil {
					return err
				}
				if projectID == "" {
					return fmt.Errorf("--project-id is required when using --user-id")
				}
				if date == "" {
					return fmt.Errorf("--date is required when using --user-id")
				}
				if err := validateOptionalDate("date", date); err != nil {
					return err
				}
				if !cmd.Flags().Changed("hours") {
					return fmt.Errorf("--hours is required when using --user-id")
				}

				item := map[string]any{
					"userId":    userID,
					"projectId": projectID,
					"date":      date,
					"hours":     hours,
				}
				if capabilityID != "" {
					if err := requireUUIDArg("capability-id", capabilityID); err != nil {
						return err
					}
					item["capabilityId"] = capabilityID
				}
				rawBody = map[string]any{"items": []any{item}}
			} else {
				// Bulk mode — existing payload/file behavior
				payloadValue := payload
				if payloadValue == "" {
					payloadValue = jsonBody
				}
				rawBody, err = inputs.ParsePayload(payloadValue, filePath)
				if err != nil {
					return err
				}
				if len(rawBody) == 0 {
					return fmt.Errorf("payload cannot be empty")
				}
			}

			var items []api.ActualHour
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "actual-hours upsert",
				Method:      "POST",
				Endpoint:    "/actual-hours",
				Body:        rawBody,
				Out:         &items,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}

			if app.Printer().IsJSON() {
				return app.PrintData(items)
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, []string{
					item.ID,
					item.UserID,
					item.ProjectID,
					item.Date,
					fmt.Sprintf("%d", item.Hours),
				})
			}
			return app.Printer().Table([]string{"ID", "USER_ID", "PROJECT_ID", "DATE", "HOURS"}, rows)
		},
	}

	cmd.Flags().StringVar(&jsonBody, "body", "", "JSON payload (deprecated: use --payload)")
	cmd.Flags().StringVar(&payload, "payload", "", "JSON payload")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID (single-item mode)")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID (single-item mode)")
	cmd.Flags().StringVar(&date, "date", "", "Date YYYY-MM-DD (single-item mode)")
	cmd.Flags().IntVar(&hours, "hours", 0, "Hours (single-item mode)")
	cmd.Flags().StringVar(&capabilityID, "capability-id", "", "Capability ID (optional, single-item mode)")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	cmd.MarkFlagsMutuallyExclusive("body", "file")
	return cmd
}

func newActualHoursDeleteCommand() *cobra.Command {
	var continueOnError bool
	cmd := &cobra.Command{
		Use:   "delete <id> [<id>...]",
		Short: "Delete one or more actual-hour entries",
		Args:  argsWithUsage(cobra.MinimumNArgs(1)),
		Example: `  # Delete by ID
  supervisible actual-hours delete 019404f3-...

  # Batch-delete in a single call (one bash invocation, N HTTP DELETEs)
  supervisible actual-hours delete 019404f3-... 019404f4-... 019404f5-...

  # Keep going if one delete returns 404 mid-batch
  supervisible actual-hours delete --continue-on-error a b c`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			for i, id := range args {
				if err := requireUUIDArg(fmt.Sprintf("id[%d]", i), id); err != nil {
					return err
				}
			}

			if app.DryRun() {
				return previewBatchDelete(
					app,
					"actual-hours delete",
					"/actual-hours/{actual_hour_id}",
					"/actual-hours/",
					app.RequiredScope("DELETE", "/actual-hours/{actual_hour_id}"),
					args,
				)
			}

			client, err := app.RequireClient()
			if err != nil {
				return err
			}
			return runBatchDelete(cmd.Context(), app, "actual-hour", args, client.DeleteActualHour, continueOnError)
		},
	}
	cmd.Flags().BoolVar(&continueOnError, "continue-on-error", false, "Keep deleting subsequent IDs after a failure (default: stop at first error)")
	return cmd
}

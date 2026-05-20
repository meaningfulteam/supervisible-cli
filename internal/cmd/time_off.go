package cmd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/api"
)

func newTimeOffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time-off",
		Short: "Manage time off requests",
	}

	cmd.AddCommand(
		newTimeOffListCommand(),
		newTimeOffCreateCommand(),
		newTimeOffUpdateCommand(),
		newTimeOffDeleteCommand(),
		newTimeOffApproveCommand(),
		newTimeOffRejectCommand(),
	)

	return cmd
}

func newTimeOffListCommand() *cobra.Command {
	var (
		userID    string
		status    string
		startDate string
		endDate   string
		limit     int
		offset    int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List time off requests",
		Example: `  # Approved time-off for a user this quarter
  supervisible time-off list --user-id <uuid> --status approved \
    --start-date 2026-04-01 --end-date 2026-06-30 --json

  # Pending requests for review
  supervisible time-off list --status pending`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			baseQuery := url.Values{}
			if userID != "" {
				baseQuery.Set("user_id", userID)
			}
			if status != "" {
				baseQuery.Set("status", status)
			}
			if startDate != "" {
				baseQuery.Set("start_date", startDate)
			}
			if endDate != "" {
				baseQuery.Set("end_date", endDate)
			}
			baseQuery.Set("limit", strconv.Itoa(limit))
			baseQuery.Set("offset", strconv.Itoa(offset))

			var items []api.TimeOffRequest
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "time-off list",
				Method:      "GET",
				Endpoint:    "/time-off",
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
					item.StartDate,
					item.EndDate,
					item.Status,
					strconv.Itoa(item.Availability),
				})
			}
			return app.Printer().Table([]string{"ID", "USER", "START", "END", "STATUS", "AVAILABILITY"}, rows)
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "Filter by user ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status: pending|approved|rejected")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Pagination limit")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	return cmd
}

func newTimeOffCreateCommand() *cobra.Command {
	var (
		userID        string
		timeOffTypeID string
		startDate     string
		endDate       string
		availability  int
		reason        string
		status        string
		payload       string
		filePath      string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a time off request",
		Example: `  # Create vacation request
  supervisible time-off create \
    --user-id <uuid> --time-off-type-id <uuid> \
    --start-date 2026-07-15 --end-date 2026-07-19 \
    --reason "Family trip"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(userID) == "" || strings.TrimSpace(timeOffTypeID) == "" || strings.TrimSpace(startDate) == "" || strings.TrimSpace(endDate) == "" || strings.TrimSpace(reason) == "" {
				return fmt.Errorf("--user-id, --time-off-type-id, --start-date, --end-date and --reason are required")
			}
			if err := requireUUIDArg("user-id", userID); err != nil {
				return err
			}
			if err := requireUUIDArg("time-off-type-id", timeOffTypeID); err != nil {
				return err
			}
			if err := validateOptionalDate("start-date", startDate); err != nil {
				return err
			}
			if err := validateOptionalDate("end-date", endDate); err != nil {
				return err
			}

			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			input := api.CreateTimeOffInput{
				UserID:        userID,
				TimeOffTypeID: timeOffTypeID,
				StartDate:     startDate,
				EndDate:       endDate,
				Availability:  availability,
				Reason:        reason,
			}
			if cmd.Flags().Changed("status") {
				input.Status = ptr(status)
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}

			var created api.TimeOffRequest
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "time-off create",
				Method:      "POST",
				Endpoint:    "/time-off",
				Body:        body,
				Out:         &created,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}
			if app.Printer().IsJSON() {
				return app.PrintData(created)
			}
			app.Printer().Aux("Created time off request: %s", created.ID)
			app.Printer().Aux("Status: %s", created.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "User ID (required)")
	cmd.Flags().StringVar(&timeOffTypeID, "time-off-type-id", "", "Time off type ID (required)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, required)")
	cmd.Flags().IntVar(&availability, "availability", 0, "Daily available hours (0-24)")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason (required)")
	cmd.Flags().StringVar(&status, "status", "", "Optional status: pending|approved|rejected")
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	return cmd
}

func newTimeOffUpdateCommand() *cobra.Command {
	var (
		timeOffTypeID string
		startDate     string
		endDate       string
		availability  int
		reason        string
		payload       string
		filePath      string
	)

	cmd := &cobra.Command{
		Use:   "update <request-id>",
		Short: "Update a time off request",
		Example: `  # Change end date
  supervisible time-off update 019404f3-... --end-date 2026-07-22`,
		Args: argsWithUsage(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := requireUUIDArg("request-id", args[0]); err != nil {
				return err
			}

			input := api.UpdateTimeOffInput{}
			changed := false
			if cmd.Flags().Changed("time-off-type-id") {
				input.TimeOffTypeID = ptr(timeOffTypeID)
				changed = true
			}
			if cmd.Flags().Changed("start-date") {
				if err := validateOptionalDate("start-date", startDate); err != nil {
					return err
				}
				input.StartDate = ptr(startDate)
				changed = true
			}
			if cmd.Flags().Changed("end-date") {
				if err := validateOptionalDate("end-date", endDate); err != nil {
					return err
				}
				input.EndDate = ptr(endDate)
				changed = true
			}
			if cmd.Flags().Changed("availability") {
				input.Availability = ptr(availability)
				changed = true
			}
			if cmd.Flags().Changed("reason") {
				input.Reason = ptr(reason)
				changed = true
			}

			if !changed {
				return fmt.Errorf("no fields provided: pass at least one flag to update")
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}

			var updated api.TimeOffRequest
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "time-off update",
				Method:      "PATCH",
				Endpoint:    "/time-off/{request_id}",
				Path:        "/time-off/" + args[0],
				Body:        body,
				Out:         &updated,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}
			if app.Printer().IsJSON() {
				return app.PrintData(updated)
			}
			app.Printer().Aux("Updated time off request: %s", updated.ID)
			app.Printer().Aux("Status: %s", updated.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&timeOffTypeID, "time-off-type-id", "", "Time off type ID")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&availability, "availability", 0, "Daily available hours (0-24)")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason")
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	return cmd
}

func newTimeOffDeleteCommand() *cobra.Command {
	var payload, filePath string

	cmd := &cobra.Command{
		Use:   "delete <request-id>",
		Short: "Delete a time off request",
		Example: `  # Delete by request ID
  supervisible time-off delete 019404f3-...`,
		Args: argsWithUsage(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := requireUUIDArg("request-id", args[0]); err != nil {
				return err
			}
			if err := ensurePayloadUnsupported(payload, filePath); err != nil {
				return err
			}

			var deleted map[string]string
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "time-off delete",
				Method:      "DELETE",
				Endpoint:    "/time-off/{request_id}",
				Path:        "/time-off/" + args[0],
				Out:         &deleted,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}
			if app.Printer().IsJSON() {
				return app.PrintData(deleted)
			}
			app.Printer().Aux("Deleted request: %s", deleted["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object (unsupported for this command)")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file (unsupported for this command)")
	return cmd
}

func newTimeOffApproveCommand() *cobra.Command {
	var payload, filePath string

	cmd := &cobra.Command{
		Use:   "approve <request-id>",
		Short: "Approve a time off request",
		Args:  argsWithUsage(cobra.ExactArgs(1)),
		Example: `  # Approve a pending request
  supervisible time-off approve 019404f3-...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := requireUUIDArg("request-id", args[0]); err != nil {
				return err
			}
			if err := ensurePayloadUnsupported(payload, filePath); err != nil {
				return err
			}

			var approved api.TimeOffRequest
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "time-off approve",
				Method:      "POST",
				Endpoint:    "/time-off/{request_id}/approve",
				Path:        "/time-off/" + args[0] + "/approve",
				Out:         &approved,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}
			if app.Printer().IsJSON() {
				return app.PrintData(approved)
			}
			app.Printer().Aux("Approved request: %s", approved.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object (unsupported for this command)")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file (unsupported for this command)")
	return cmd
}

func newTimeOffRejectCommand() *cobra.Command {
	var reason, payload, filePath string

	cmd := &cobra.Command{
		Use:   "reject <request-id>",
		Short: "Reject a time off request",
		Args:  argsWithUsage(cobra.ExactArgs(1)),
		Example: `  # Reject with a reason
  supervisible time-off reject 019404f3-... --reason "Coverage conflict"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("--reason is required")
			}

			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := requireUUIDArg("request-id", args[0]); err != nil {
				return err
			}

			input := api.RejectTimeOffInput{RejectionReason: reason}
			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}

			var rejected api.TimeOffRequest
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "time-off reject",
				Method:      "POST",
				Endpoint:    "/time-off/{request_id}/reject",
				Path:        "/time-off/" + args[0] + "/reject",
				Body:        body,
				Out:         &rejected,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}
			if app.Printer().IsJSON() {
				return app.PrintData(rejected)
			}
			app.Printer().Aux("Rejected request: %s", rejected.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Rejection reason")
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	return cmd
}

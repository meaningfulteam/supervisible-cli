package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/inputs"
)

func newAssignmentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assignments",
		Short: "Manage user assignments",
	}

	cmd.AddCommand(
		newAssignmentsListCommand(),
		newAssignmentsUpsertCommand(),
		newAssignmentsAddCommand(),
		newAssignmentsMoveCommand(),
		newAssignmentsDeleteCommand(),
	)

	return cmd
}

func newAssignmentsListCommand() *cobra.Command {
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
		Short: "List assignments",
		Example: `  # Assignments for one user this month
  supervisible assignments list --user-id <user-uuid> \
    --start-date 2026-05-01 --end-date 2026-05-31 --json

  # Assignments for one project
  supervisible assignments list --project-id <project-uuid> --limit 100`,
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

			var items []api.Assignment
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "assignments list",
				Method:      "GET",
				Endpoint:    "/assignments",
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

func newAssignmentsUpsertCommand() *cobra.Command {
	var (
		jsonBody       string
		payload        string
		filePath       string
		userID         string
		projectID      string
		date           string
		hours          int
		capabilityID   string
		autoCapability bool
	)

	cmd := &cobra.Command{
		Use:   "upsert",
		Short: "Upsert assignments",
		Long:  `Upsert assignments via individual flags or bulk JSON.`,
		Example: `  # Single item via flags
  supervisible assignments upsert --user-id <uuid> --project-id <uuid> \
    --date 2026-03-06 --hours 8 --capability-id <uuid>

  # Bulk via inline JSON, with capabilityId resolved per item
  supervisible assignments upsert --body '{"items":[...]}' --auto-capability

  # Bulk from a file
  supervisible assignments upsert --file payload.json`,
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

			if autoCapability {
				if err := fillAutoCapability(cmd.Context(), app, rawBody); err != nil {
					return err
				}
			}

			preflightTimeOff(cmd.Context(), app, rawBody)

			var items []api.Assignment
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "assignments upsert",
				Method:      "POST",
				Endpoint:    "/assignments",
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
	cmd.Flags().BoolVar(&autoCapability, "auto-capability", false, "Fill missing capabilityId per item using most-recent (user, project) capability")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	cmd.MarkFlagsMutuallyExclusive("body", "file")
	return cmd
}

// fillAutoCapability scans rawBody["items"] (the upsert envelope) and fills any
// item missing "capabilityId" by resolving it from the (userId, projectId) pair.
// Mutates rawBody in place. Returns the combined error if any item fails so
// callers don't partial-write.
func fillAutoCapability(ctx context.Context, app *App, rawBody map[string]any) error {
	itemsRaw, ok := rawBody["items"].([]any)
	if !ok || len(itemsRaw) == 0 {
		return nil
	}

	client, err := app.RequireClient()
	if err != nil {
		return err
	}
	resolver := newCapabilityResolver(client)

	var failures []string
	for idx, raw := range itemsRaw {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if existing, _ := item["capabilityId"].(string); existing != "" {
			continue
		}
		userID, _ := item["userId"].(string)
		projectID, _ := item["projectId"].(string)
		if userID == "" || projectID == "" {
			failures = append(failures, fmt.Sprintf("item[%d]: cannot resolve capability without userId+projectId", idx))
			continue
		}
		resolved, err := resolver.Resolve(ctx, userID, projectID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("item[%d]: %v", idx, err))
			continue
		}
		item["capabilityId"] = resolved
		app.Printer().Aux("capability resolved for %s/%s: %s", userID, projectID, resolved)
	}
	if len(failures) > 0 {
		return fmt.Errorf("auto-capability resolution failed:\n  %s", strings.Join(failures, "\n  "))
	}
	return nil
}

func newAssignmentsAddCommand() *cobra.Command {
	var (
		userID         string
		projectID      string
		date           string
		delta          int
		capabilityID   string
		autoCapability bool
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add hours to an assignment (read-modify-write)",
		Long: `Add (or subtract) hours from an existing (user, project, capability, date)
assignment. Fetches the current row, computes new = existing + delta, and upserts.

If no row exists, creates one with hours=delta. If the resulting value would go
negative, the command fails before writing. If the resulting value is exactly 0
on an existing row, the row is deleted (DELETE /assignments/{id}) instead of
being upserted to hours:0.

Race condition: another writer can modify the row between read and write. For
a single-actor CLI this is acceptable; document the trade-off if scripting.`,
		Example: `  # Add 2h to today's web-dev assignment for Juan, capability auto-resolved
  supervisible assignments add \
    --user-id 019404f3-... --project-id 019e1cde-... \
    --date 2026-05-24 --hours 2 --auto-capability

  # Subtract 1h with explicit capability (no auto-resolve)
  supervisible assignments add \
    --user-id 019404f3-... --project-id 019e1cde-... \
    --capability-id 0194b2e1-... \
    --date 2026-05-24 --hours -1`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireUUIDArg("user-id", userID); err != nil {
				return err
			}
			if err := requireUUIDArg("project-id", projectID); err != nil {
				return err
			}
			if err := validateOptionalDate("date", date); err != nil {
				return err
			}
			if !cmd.Flags().Changed("hours") {
				return fmt.Errorf("--hours is required")
			}

			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			// Resolve capability if needed.
			if capabilityID == "" {
				if !autoCapability {
					return fmt.Errorf("either --capability-id or --auto-capability is required")
				}
				resolver := newCapabilityResolver(client)
				resolved, err := resolver.Resolve(cmd.Context(), userID, projectID)
				if err != nil {
					return err
				}
				capabilityID = resolved
				app.Printer().Aux("capability resolved for %s/%s: %s", userID, projectID, resolved)
			} else if err := requireUUIDArg("capability-id", capabilityID); err != nil {
				return err
			}

			// Read existing row.
			existingHours, existingID, err := findAssignmentHours(cmd.Context(), client, userID, projectID, capabilityID, date)
			if err != nil {
				return err
			}

			newHours := existingHours + delta
			if newHours < 0 {
				return fmt.Errorf("computed hours would be negative (%d + %d = %d); refusing to write", existingHours, delta, newHours)
			}

			// Reaching exactly 0 on an existing row → delete the row server-side
			// instead of upserting hours:0 (which would leave a zombie row that
			// list endpoints / aggregators have to filter out).
			if existingID != "" && newHours == 0 {
				app.Printer().Aux("assignments add: %s %s %s %dh + %dh = 0h → deleting assignment %s", userID, projectID, date, existingHours, delta, existingID)

				plan := RequestPlan{
					CommandPath:   "assignments add",
					Method:        "DELETE",
					Endpoint:      "/assignments/" + existingID,
					RequiredScope: app.RequiredScope("DELETE", "/assignments/{assignment_id}"),
				}
				if app.MaybeDryRun(plan) {
					return nil
				}

				if err := client.DeleteAssignment(cmd.Context(), existingID); err != nil {
					return err
				}
				if app.Printer().IsJSON() {
					return app.PrintData(map[string]string{"id": existingID, "deleted": "true"})
				}
				app.Printer().Aux("Deleted assignment: %s", existingID)
				return nil
			}

			app.Printer().Aux("assignments add: %s %s %s %dh + %dh = %dh", userID, projectID, date, existingHours, delta, newHours)

			item := map[string]any{
				"userId":       userID,
				"projectId":    projectID,
				"capabilityId": capabilityID,
				"date":         date,
				"hours":        newHours,
			}
			rawBody := map[string]any{"items": []any{item}}

			preflightTimeOff(cmd.Context(), app, rawBody)

			var items []api.Assignment
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "assignments add",
				Method:      "POST",
				Endpoint:    "/assignments",
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
			for _, it := range items {
				rows = append(rows, []string{it.ID, it.UserID, it.ProjectID, it.Date, fmt.Sprintf("%d", it.Hours)})
			}
			return app.Printer().Table([]string{"ID", "USER_ID", "PROJECT_ID", "DATE", "HOURS"}, rows)
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "User ID (required)")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID (required)")
	cmd.Flags().StringVar(&date, "date", "", "Date YYYY-MM-DD (required)")
	cmd.Flags().IntVar(&delta, "hours", 0, "Hours delta to add (can be negative)")
	cmd.Flags().StringVar(&capabilityID, "capability-id", "", "Capability ID (omit to use --auto-capability)")
	cmd.Flags().BoolVar(&autoCapability, "auto-capability", true, "Resolve capability from history when --capability-id is empty (default true)")
	return cmd
}

// findAssignmentHours returns the existing hours and assignment ID for
// (user, project, capability, date), or (0, "", nil) when no row exists.
func findAssignmentHours(ctx context.Context, client *api.Client, userID, projectID, capabilityID, date string) (int, string, error) {
	q := url.Values{}
	q.Set("user_id", userID)
	q.Set("project_id", projectID)
	q.Set("start_date", date)
	q.Set("end_date", date)
	q.Set("limit", "50")

	var rows []api.Assignment
	if err := client.Do(ctx, "GET", "/assignments", q, nil, &rows); err != nil {
		return 0, "", fmt.Errorf("fetch existing assignment: %w", err)
	}
	for _, r := range rows {
		if r.Date != date {
			continue
		}
		if r.CapabilityID == nil || *r.CapabilityID != capabilityID {
			continue
		}
		return r.Hours, r.ID, nil
	}
	return 0, "", nil
}

func newAssignmentsMoveCommand() *cobra.Command {
	var (
		toUserID     string
		moveHours    int
		capabilityID string
	)

	cmd := &cobra.Command{
		Use:   "move <assignment-id>",
		Short: "Move hours from one user to another on the same project",
		Long: `Move hours from an existing assignment to a different user on the same
project and date.

Atomic on the server: POST /assignments/{id}/move performs the read,
validation, target upsert, and source delete/decrement in a single
transaction. No TOCTOU window; no partial-failure path.

Returns { source, target } — source is null when all hours were moved
(the source row was deleted).`,
		Example: `  # Move all of Mariana's hours on row X to Herbert
  supervisible assignments move 019d27a4-... \
    --to-user 0197a204-... \
    --capability-id 0194b2e1-...

  # Move just 4 hours
  supervisible assignments move 019d27a4-... \
    --to-user 0197a204-... \
    --capability-id 0194b2e1-... --hours 4

  # Preview without writing
  supervisible assignments move 019d27a4-... \
    --to-user 0197a204-... --dry-run`,
		Args: argsWithUsage(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			assignmentID := args[0]
			if err := requireUUIDArg("assignment-id", assignmentID); err != nil {
				return err
			}
			if err := requireUUIDArg("to-user", toUserID); err != nil {
				return err
			}
			if cmd.Flags().Changed("hours") && moveHours <= 0 {
				return fmt.Errorf("--hours must be positive (got %d)", moveHours)
			}
			if cmd.Flags().Changed("capability-id") {
				if err := requireUUIDArg("capability-id", capabilityID); err != nil {
					return err
				}
			}

			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			body := map[string]any{"toUserId": toUserID}
			if cmd.Flags().Changed("hours") {
				body["hours"] = moveHours
			}
			if cmd.Flags().Changed("capability-id") {
				body["capabilityId"] = capabilityID
			}

			var result map[string]any
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "assignments move",
				Method:      "POST",
				Endpoint:    "/assignments/{assignment_id}/move",
				Path:        "/assignments/" + assignmentID + "/move",
				Body:        body,
				Out:         &result,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}

			if app.Printer().IsJSON() {
				return app.PrintData(result)
			}
			renderAssignmentsMoveResult(app, result)
			return nil
		},
	}

	cmd.Flags().StringVar(&toUserID, "to-user", "", "Target user ID (required)")
	cmd.Flags().IntVar(&moveHours, "hours", 0, "Hours to move (default: all of source's hours)")
	cmd.Flags().StringVar(&capabilityID, "capability-id", "", "Target user's capability on this project (omit to inherit no capability)")
	_ = cmd.MarkFlagRequired("to-user")
	return cmd
}

// renderAssignmentsMoveResult prints a one-line non-JSON summary of the
// server's { source, target } response.
func renderAssignmentsMoveResult(app *App, result map[string]any) {
	target, _ := result["target"].(map[string]any)
	source := result["source"]

	targetID, _ := target["id"].(string)
	targetUser, _ := target["userId"].(string)
	targetHours := numberField(target, "hours")

	switch s := source.(type) {
	case nil:
		app.Printer().Aux("moved: source deleted, target %s for user %s (%dh)", targetID, targetUser, targetHours)
	case map[string]any:
		srcID, _ := s["id"].(string)
		srcHours := numberField(s, "hours")
		app.Printer().Aux("moved: source %s now %dh, target %s for user %s (%dh)", srcID, srcHours, targetID, targetUser, targetHours)
	}
}

func numberField(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func newAssignmentsDeleteCommand() *cobra.Command {
	var continueOnError bool
	cmd := &cobra.Command{
		Use:   "delete <id> [<id>...]",
		Short: "Delete one or more assignments",
		Args:  argsWithUsage(cobra.MinimumNArgs(1)),
		Example: `  # Delete by ID
  supervisible assignments delete 019404f3-...

  # Batch-delete in a single call (one bash invocation, N HTTP DELETEs)
  supervisible assignments delete 019404f3-... 019404f4-... 019404f5-...

  # Keep going if one delete returns 404 mid-batch
  supervisible assignments delete --continue-on-error a b c`,
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
					"assignments delete",
					"/assignments/{assignment_id}",
					"/assignments/",
					app.RequiredScope("DELETE", "/assignments/{assignment_id}"),
					args,
				)
			}

			client, err := app.RequireClient()
			if err != nil {
				return err
			}
			return runBatchDelete(cmd.Context(), app, "assignment", args, client.DeleteAssignment, continueOnError)
		},
	}
	cmd.Flags().BoolVar(&continueOnError, "continue-on-error", false, "Keep deleting subsequent IDs after a failure (default: stop at first error)")
	return cmd
}

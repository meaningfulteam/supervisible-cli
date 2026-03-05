package cmd

import (
	"fmt"
	"net/url"
	"strconv"

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
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			client, err := app.RequireClient()
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
			query := app.ResolvedQuery("GET", "/assignments", baseQuery)

			var items []api.Assignment
			err = client.Do(cmd.Context(), "GET", "/assignments", query, nil, &items)
			if err != nil {
				return err
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
		jsonBody string
		payload  string
		filePath string
	)

	cmd := &cobra.Command{
		Use:   "upsert",
		Short: "Bulk upsert assignments",
		Long:  "Pass JSON via --body or --file. Accepts either {\"items\":[...]} or an array of items.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			payloadValue := payload
			if payloadValue == "" {
				payloadValue = jsonBody
			}

			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			rawBody, err := inputs.ParsePayload(payloadValue, filePath)
			if err != nil {
				return err
			}
			if len(rawBody) == 0 {
				return fmt.Errorf("payload cannot be empty")
			}
			query := app.ResolvedQuery("POST", "/assignments", nil)
			plan := RequestPlan{
				CommandPath:   "assignments upsert",
				Method:        "POST",
				Endpoint:      "/assignments",
				Query:         query,
				Body:          rawBody,
				RequiredScope: app.RequiredScope("POST", "/assignments"),
			}
			if app.MaybeDryRun(plan) {
				return nil
			}

			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			var items []api.Assignment
			err = client.Do(cmd.Context(), "POST", "/assignments", query, rawBody, &items)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(items)
			}
			app.Printer().PrintMessage("Upserted %d assignment(s)", len(items))
			return nil
		},
	}

	cmd.Flags().StringVar(&jsonBody, "body", "", "JSON payload (deprecated: use --payload)")
	cmd.Flags().StringVar(&payload, "payload", "", "JSON payload")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	cmd.MarkFlagsMutuallyExclusive("body", "file")
	return cmd
}

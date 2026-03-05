package cmd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/api"
)

func newProjectsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage projects",
	}

	cmd.AddCommand(
		newProjectsListCommand(),
		newProjectsCreateCommand(),
		newProjectsUpdateCommand(),
	)

	return cmd
}

func newProjectsListCommand() *cobra.Command {
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Args:  cobra.NoArgs,
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
			baseQuery.Set("limit", strconv.Itoa(limit))
			baseQuery.Set("offset", strconv.Itoa(offset))
			query := app.ResolvedQuery("GET", "/projects", baseQuery)

			var projects []api.Project
			err = client.Do(cmd.Context(), "GET", "/projects", query, nil, &projects)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(projects)
			}

			rows := make([][]string, 0, len(projects))
			for _, project := range projects {
				rows = append(rows, []string{
					project.ID,
					project.Name,
					project.ClientID,
					project.StartDate,
					project.EndDate,
					project.Status,
				})
			}
			return app.Printer().Table([]string{"ID", "NAME", "CLIENT", "START", "END", "STATUS"}, rows)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Pagination limit")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	return cmd
}

func newProjectsCreateCommand() *cobra.Command {
	var (
		name             string
		clientID         string
		startDate        string
		endDate          string
		objective        string
		projectManagerID string
		status           string
		billingType      string
		amount           float64
		hourlyRate       float64
		payload          string
		filePath         string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(clientID) == "" || strings.TrimSpace(startDate) == "" || strings.TrimSpace(endDate) == "" {
				return fmt.Errorf("--name, --client-id, --start-date and --end-date are required")
			}
			if err := requireUUIDArg("client-id", clientID); err != nil {
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

			input := api.CreateProjectInput{
				Name:      name,
				ClientID:  clientID,
				StartDate: startDate,
				EndDate:   endDate,
			}

			if cmd.Flags().Changed("objective") {
				input.Objective = stringPtr(objective)
			}
			if cmd.Flags().Changed("project-manager-id") {
				input.ProjectManagerID = stringPtr(projectManagerID)
			}
			if cmd.Flags().Changed("status") {
				input.Status = stringPtr(status)
			}
			if cmd.Flags().Changed("billing-type") {
				input.BillingType = stringPtr(billingType)
			}
			if cmd.Flags().Changed("amount") {
				input.Amount = float64Ptr(amount)
			}
			if cmd.Flags().Changed("hourly-rate") {
				input.HourlyRate = float64Ptr(hourlyRate)
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}
			query := app.ResolvedQuery("POST", "/projects", nil)
			plan := RequestPlan{
				CommandPath:   "projects create",
				Method:        "POST",
				Endpoint:      "/projects",
				Query:         query,
				Body:          body,
				RequiredScope: app.RequiredScope("POST", "/projects"),
			}
			if app.MaybeDryRun(plan) {
				return nil
			}

			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			var created api.Project
			err = client.Do(cmd.Context(), "POST", "/projects", query, body, &created)
			if err != nil {
				return err
			}
			if app.Printer().IsJSON() {
				return app.PrintData(created)
			}
			app.Printer().PrintMessage("Created project: %s", created.ID)
			app.Printer().PrintMessage("Name: %s", created.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name (required)")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Client ID (required)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&objective, "objective", "", "Project objective")
	cmd.Flags().StringVar(&projectManagerID, "project-manager-id", "", "Project manager user ID")
	cmd.Flags().StringVar(&status, "status", "", "Status: draft|planned|active|completed|cancelled")
	cmd.Flags().StringVar(&billingType, "billing-type", "", "Billing type: fixed_price|retainer|time_materials|non_billable")
	cmd.Flags().Float64Var(&amount, "amount", 0, "Amount for fixed_price/retainer")
	cmd.Flags().Float64Var(&hourlyRate, "hourly-rate", 0, "Hourly rate for time_materials")
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	return cmd
}

func newProjectsUpdateCommand() *cobra.Command {
	var (
		name             string
		objective        string
		startDate        string
		endDate          string
		projectManagerID string
		status           string
		payload          string
		filePath         string
	)

	cmd := &cobra.Command{
		Use:   "update <project-id>",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := requireUUIDArg("project-id", args[0]); err != nil {
				return err
			}

			input := api.UpdateProjectInput{}
			changed := false

			if cmd.Flags().Changed("name") {
				input.Name = stringPtr(name)
				changed = true
			}
			if cmd.Flags().Changed("objective") {
				input.Objective = stringPtr(objective)
				changed = true
			}
			if cmd.Flags().Changed("start-date") {
				if err := validateOptionalDate("start-date", startDate); err != nil {
					return err
				}
				input.StartDate = stringPtr(startDate)
				changed = true
			}
			if cmd.Flags().Changed("end-date") {
				if err := validateOptionalDate("end-date", endDate); err != nil {
					return err
				}
				input.EndDate = stringPtr(endDate)
				changed = true
			}
			if cmd.Flags().Changed("project-manager-id") {
				input.ProjectManagerID = stringPtr(projectManagerID)
				changed = true
			}
			if cmd.Flags().Changed("status") {
				input.Status = stringPtr(status)
				changed = true
			}

			if !changed {
				return fmt.Errorf("no fields provided: pass at least one flag to update")
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}
			query := app.ResolvedQuery("PATCH", "/projects/{project_id}", nil)
			plan := RequestPlan{
				CommandPath:   "projects update",
				Method:        "PATCH",
				Endpoint:      "/projects/" + args[0],
				Query:         query,
				Body:          body,
				RequiredScope: app.RequiredScope("PATCH", "/projects/{project_id}"),
			}
			if app.MaybeDryRun(plan) {
				return nil
			}

			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			var updated api.Project
			err = client.Do(cmd.Context(), "PATCH", "/projects/"+args[0], query, body, &updated)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(updated)
			}
			app.Printer().PrintMessage("Updated project: %s", updated.ID)
			app.Printer().PrintMessage("Name: %s", updated.Name)
			app.Printer().PrintMessage("Status: %s", updated.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&objective, "objective", "", "Project objective")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&projectManagerID, "project-manager-id", "", "Project manager user ID")
	cmd.Flags().StringVar(&status, "status", "", "Status: draft|planned|active|completed|cancelled")
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	return cmd
}

func boolToText(v bool) string {
	return strconv.FormatBool(v)
}

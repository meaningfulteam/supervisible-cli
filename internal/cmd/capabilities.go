package cmd

import (
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

// PublicApiCapability mirrors the server's GET /capabilities row shape.
type PublicApiCapability struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

func newCapabilitiesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Inspect the organization's capabilities",
	}

	cmd.AddCommand(newCapabilitiesListCommand())
	return cmd
}

func newCapabilitiesListCommand() *cobra.Command {
	var (
		projectID string
		limit     int
		offset    int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List capabilities — optionally filtered to a single project",
		Long: `List the organization's capabilities. With --for-project, the server filters
through project_capabilities so only the capabilities attached to that
project come back — the recovery path when --auto-capability fails on a
user with no prior history on the project.`,
		Example: `  # All capabilities in the caller's org
  supervisible capabilities list --json

  # Capabilities staffable on a specific project
  supervisible capabilities list --for-project 019c885e-... --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed("for-project") {
				if err := requireUUIDArg("for-project", projectID); err != nil {
					return err
				}
			}

			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			q := url.Values{}
			q.Set("limit", strconv.Itoa(limit))
			q.Set("offset", strconv.Itoa(offset))
			if projectID != "" {
				q.Set("for_project", projectID)
			}

			var rows []PublicApiCapability
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "capabilities list",
				Method:      "GET",
				Endpoint:    "/capabilities",
				Query:       q,
				Out:         &rows,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}

			if len(rows) == 0 {
				if projectID != "" {
					app.Printer().Aux("note: no capabilities attached to project %s.", projectID)
				} else {
					app.Printer().Aux("note: organization has no capabilities defined.")
				}
			}

			if app.Printer().IsJSON() {
				return app.PrintData(rows)
			}

			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				desc := ""
				if r.Description != nil {
					desc = *r.Description
				}
				tableRows = append(tableRows, []string{r.ID, r.Name, desc})
			}
			return app.Printer().Table([]string{"ID", "NAME", "DESCRIPTION"}, tableRows)
		},
	}

	cmd.Flags().StringVar(&projectID, "for-project", "", "Filter to capabilities attached to this project (optional)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Pagination limit")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	return cmd
}

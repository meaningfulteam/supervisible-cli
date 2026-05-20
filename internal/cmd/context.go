package cmd

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/output"
)

// --- Output types for context command ---

type ContextUser struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	IsActive bool   `json:"isActive"`
}

type ContextClient struct {
	ID          string `json:"id"`
	CompanyName string `json:"companyName"`
	IsActive    bool   `json:"isActive"`
}

type ContextProject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	ClientID string `json:"clientId"`
}

type ContextReport struct {
	Organization string           `json:"organization"`
	Users        []ContextUser    `json:"users"`
	Clients      []ContextClient  `json:"clients"`
	Projects     []ContextProject `json:"projects"`
}

func newContextCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Machine-readable org summary for agent bootstrap",
		Long: `Fetch users, clients, and projects to produce a full org context summary.

Designed for AI agents to orient themselves in a single call.`,
		Example: `  # Summary on stdout
  supervisible context

  # JSON for agents
  supervisible context --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			baseQuery := url.Values{}
			baseQuery.Set("limit", fetchLimit)

			// Fetch identity for organization
			var identity api.Identity
			identityQuery := app.ResolvedQuery("GET", "/me", nil)
			if err := client.Do(ctx, "GET", "/me", identityQuery, nil, &identity); err != nil {
				return fmt.Errorf("fetch identity: %w", err)
			}

			// Fetch users
			userQuery := app.ResolvedQuery("GET", "/users", cloneQuery(baseQuery))
			var users []api.User
			if err := client.Do(ctx, "GET", "/users", userQuery, nil, &users); err != nil {
				return fmt.Errorf("fetch users: %w", err)
			}

			// Fetch clients
			clientQuery := app.ResolvedQuery("GET", "/clients", cloneQuery(baseQuery))
			var clients []api.ClientResource
			if err := client.Do(ctx, "GET", "/clients", clientQuery, nil, &clients); err != nil {
				return fmt.Errorf("fetch clients: %w", err)
			}

			// Fetch projects
			projectQuery := app.ResolvedQuery("GET", "/projects", cloneQuery(baseQuery))
			var projects []api.Project
			if err := client.Do(ctx, "GET", "/projects", projectQuery, nil, &projects); err != nil {
				return fmt.Errorf("fetch projects: %w", err)
			}

			report := buildContextReport(identity, users, clients, projects)

			if app.Printer().IsJSON() {
				return app.PrintData(report)
			}

			w := app.Printer().Stdout()
			fmt.Fprintf(w, "Organization: %s\n", report.Organization)
			fmt.Fprintf(w, "%d users, %d clients, %d projects\n",
				len(report.Users), len(report.Clients), len(report.Projects))
			return nil
		},
	}

	return cmd
}

func buildContextReport(identity api.Identity, users []api.User, clients []api.ClientResource, projects []api.Project) ContextReport {
	report := ContextReport{
		Organization: identity.OrganizationID,
		Users:        make([]ContextUser, 0, len(users)),
		Clients:      make([]ContextClient, 0, len(clients)),
		Projects:     make([]ContextProject, 0, len(projects)),
	}

	for _, u := range users {
		report.Users = append(report.Users, ContextUser{
			ID:       u.ID,
			Name:     output.CoalesceString(u.Name),
			Email:    u.Email,
			IsActive: u.IsActive,
		})
	}

	for _, c := range clients {
		report.Clients = append(report.Clients, ContextClient{
			ID:          c.ID,
			CompanyName: c.CompanyName,
			IsActive:    c.IsActive,
		})
	}

	for _, p := range projects {
		report.Projects = append(report.Projects, ContextProject{
			ID:       p.ID,
			Name:     p.Name,
			Status:   p.Status,
			ClientID: p.ClientID,
		})
	}

	return report
}

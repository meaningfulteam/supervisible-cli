package cmd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/output"
)

func newClientsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clients",
		Short: "Manage clients",
	}

	cmd.AddCommand(
		newClientsListCommand(),
		newClientsCreateCommand(),
		newClientsUpdateCommand(),
	)

	return cmd
}

func newClientsListCommand() *cobra.Command {
	var (
		limit, offset int
		nameFilter    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List clients",
		Args:  cobra.NoArgs,
		Example: `  # List active clients
  supervisible clients list

  # Find by company name (case-insensitive substring)
  supervisible clients list --name "avask" --json

  # Paginate JSON for agents
  supervisible clients list --limit 50 --offset 0 --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			baseQuery := url.Values{}
			baseQuery.Set("limit", strconv.Itoa(limit))
			baseQuery.Set("offset", strconv.Itoa(offset))

			var clients []api.ClientResource
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "clients list",
				Method:      "GET",
				Endpoint:    "/clients",
				Query:       baseQuery,
				Out:         &clients,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}

			getName := func(c api.ClientResource) string { return c.CompanyName }
			filtered := filterByName(clients, nameFilter, getName)
			if nameFilter != "" && len(clients) >= limit {
				app.Printer().Aux("note: list was paginated at %d rows before filtering by --name; pass --limit if you expect more", limit)
			}
			if nameFilter != "" && len(filtered) == 0 {
				emitNameMissWarning(app.Printer().Aux, "clients", clients, nameFilter, getName)
			}

			if app.Printer().IsJSON() {
				return app.PrintData(filtered)
			}

			rows := make([][]string, 0, len(filtered))
			for _, c := range filtered {
				rows = append(rows, []string{
					c.ID,
					c.CompanyName,
					output.CoalesceString(c.Website),
					c.ClientPriority,
					strconv.FormatBool(c.IsActive),
				})
			}
			return app.Printer().Table([]string{"ID", "NAME", "WEBSITE", "PRIORITY", "ACTIVE"}, rows)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Pagination limit")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVar(&nameFilter, "name", "", "Case-insensitive substring filter on the company name (applied after fetch)")
	return cmd
}

func newClientsCreateCommand() *cobra.Command {
	var (
		companyName      string
		email            string
		image            string
		countryCode      string
		website          string
		isActive         bool
		clientPriority   string
		categories       string
		accountManagerID string
		payload          string
		filePath         string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a client",
		Args:  cobra.NoArgs,
		Example: `  # Quick create via flags
  supervisible clients create --company-name "Acme Co" --website https://acme.com

  # Create from a JSON file
  supervisible clients create --file client.json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(companyName) == "" {
				return fmt.Errorf("--company-name is required")
			}

			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			input := api.CreateClientInput{CompanyName: companyName}
			if cmd.Flags().Changed("email") {
				input.Email = ptr(email)
			}
			if cmd.Flags().Changed("image") {
				input.Image = ptr(image)
			}
			if cmd.Flags().Changed("country-code") {
				input.CountryCode = ptr(countryCode)
			}
			if cmd.Flags().Changed("website") {
				input.Website = ptr(website)
			}
			if cmd.Flags().Changed("is-active") {
				input.IsActive = ptr(isActive)
			}
			if cmd.Flags().Changed("priority") {
				input.ClientPriority = ptr(clientPriority)
			}
			if cmd.Flags().Changed("categories") {
				input.Categories = splitCSV(categories)
			}
			if cmd.Flags().Changed("account-manager-id") {
				input.AccountManagerID = ptr(accountManagerID)
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}

			var created api.ClientResource
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "clients create",
				Method:      "POST",
				Endpoint:    "/clients",
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

			app.Printer().Aux("Created client: %s", created.ID)
			app.Printer().Aux("Name: %s", created.CompanyName)
			return nil
		},
	}

	cmd.Flags().StringVar(&companyName, "company-name", "", "Client company name (required)")
	cmd.Flags().StringVar(&email, "email", "", "Contact email")
	cmd.Flags().StringVar(&image, "image", "", "Logo URL")
	cmd.Flags().StringVar(&countryCode, "country-code", "", "Country code")
	cmd.Flags().StringVar(&website, "website", "", "Company URL")
	cmd.Flags().BoolVar(&isActive, "is-active", true, "Whether the client is active")
	cmd.Flags().StringVar(&clientPriority, "priority", "", "Priority: high|medium|low")
	cmd.Flags().StringVar(&categories, "categories", "", "Comma-separated categories")
	cmd.Flags().StringVar(&accountManagerID, "account-manager-id", "", "Account manager user ID")
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	return cmd
}

func newClientsUpdateCommand() *cobra.Command {
	var (
		companyName      string
		email            string
		image            string
		countryCode      string
		website          string
		isActive         bool
		clientPriority   string
		categories       string
		accountManagerID string
		payload          string
		filePath         string
	)

	cmd := &cobra.Command{
		Use:   "update <client-id>",
		Short: "Update a client",
		Args:  argsWithUsage(cobra.ExactArgs(1)),
		Example: `  # Rename a client
  supervisible clients update 019404f3-... --company-name "Acme, Inc."

  # Update via JSON payload
  supervisible clients update 019404f3-... --body '{"isActive":false}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := requireUUIDArg("client-id", args[0]); err != nil {
				return err
			}

			input := api.UpdateClientInput{}
			changed := false

			if cmd.Flags().Changed("company-name") {
				input.CompanyName = ptr(companyName)
				changed = true
			}
			if cmd.Flags().Changed("email") {
				input.Email = ptr(email)
				changed = true
			}
			if cmd.Flags().Changed("image") {
				input.Image = ptr(image)
				changed = true
			}
			if cmd.Flags().Changed("country-code") {
				input.CountryCode = ptr(countryCode)
				changed = true
			}
			if cmd.Flags().Changed("website") {
				input.Website = ptr(website)
				changed = true
			}
			if cmd.Flags().Changed("is-active") {
				input.IsActive = ptr(isActive)
				changed = true
			}
			if cmd.Flags().Changed("priority") {
				input.ClientPriority = ptr(clientPriority)
				changed = true
			}
			if cmd.Flags().Changed("categories") {
				input.Categories = splitCSV(categories)
				changed = true
			}
			if cmd.Flags().Changed("account-manager-id") {
				input.AccountManagerID = ptr(accountManagerID)
				changed = true
			}

			if !changed {
				return fmt.Errorf("no fields provided: pass at least one flag to update")
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}

			var updated api.ClientResource
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "clients update",
				Method:      "PATCH",
				Endpoint:    "/clients/{client_id}",
				Path:        "/clients/" + args[0],
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

			app.Printer().Aux("Updated client: %s", updated.ID)
			app.Printer().Aux("Name: %s", updated.CompanyName)
			return nil
		},
	}

	cmd.Flags().StringVar(&companyName, "company-name", "", "Client company name")
	cmd.Flags().StringVar(&email, "email", "", "Contact email")
	cmd.Flags().StringVar(&image, "image", "", "Logo URL")
	cmd.Flags().StringVar(&countryCode, "country-code", "", "Country code")
	cmd.Flags().StringVar(&website, "website", "", "Company URL")
	cmd.Flags().BoolVar(&isActive, "is-active", true, "Whether the client is active")
	cmd.Flags().StringVar(&clientPriority, "priority", "", "Priority: high|medium|low")
	cmd.Flags().StringVar(&categories, "categories", "", "Comma-separated categories")
	cmd.Flags().StringVar(&accountManagerID, "account-manager-id", "", "Account manager user ID")
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")
	return cmd
}

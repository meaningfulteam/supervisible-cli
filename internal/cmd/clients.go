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
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List clients",
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
			query := app.ResolvedQuery("GET", "/clients", baseQuery)

			var clients []api.ClientResource
			err = client.Do(cmd.Context(), "GET", "/clients", query, nil, &clients)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(clients)
			}

			rows := make([][]string, 0, len(clients))
			for _, c := range clients {
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
				input.Email = stringPtr(email)
			}
			if cmd.Flags().Changed("image") {
				input.Image = stringPtr(image)
			}
			if cmd.Flags().Changed("country-code") {
				input.CountryCode = stringPtr(countryCode)
			}
			if cmd.Flags().Changed("website") {
				input.Website = stringPtr(website)
			}
			if cmd.Flags().Changed("is-active") {
				input.IsActive = boolPtr(isActive)
			}
			if cmd.Flags().Changed("priority") {
				input.ClientPriority = stringPtr(clientPriority)
			}
			if cmd.Flags().Changed("categories") {
				input.Categories = splitCSV(categories)
			}
			if cmd.Flags().Changed("account-manager-id") {
				input.AccountManagerID = stringPtr(accountManagerID)
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}
			query := app.ResolvedQuery("POST", "/clients", nil)
			plan := RequestPlan{
				CommandPath:   "clients create",
				Method:        "POST",
				Endpoint:      "/clients",
				Query:         query,
				Body:          body,
				RequiredScope: app.RequiredScope("POST", "/clients"),
			}
			if app.MaybeDryRun(plan) {
				return nil
			}

			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			var created api.ClientResource
			err = client.Do(cmd.Context(), "POST", "/clients", query, body, &created)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(created)
			}

			app.Printer().PrintMessage("Created client: %s", created.ID)
			app.Printer().PrintMessage("Name: %s", created.CompanyName)
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
		Args:  cobra.ExactArgs(1),
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
				input.CompanyName = stringPtr(companyName)
				changed = true
			}
			if cmd.Flags().Changed("email") {
				input.Email = stringPtr(email)
				changed = true
			}
			if cmd.Flags().Changed("image") {
				input.Image = stringPtr(image)
				changed = true
			}
			if cmd.Flags().Changed("country-code") {
				input.CountryCode = stringPtr(countryCode)
				changed = true
			}
			if cmd.Flags().Changed("website") {
				input.Website = stringPtr(website)
				changed = true
			}
			if cmd.Flags().Changed("is-active") {
				input.IsActive = boolPtr(isActive)
				changed = true
			}
			if cmd.Flags().Changed("priority") {
				input.ClientPriority = stringPtr(clientPriority)
				changed = true
			}
			if cmd.Flags().Changed("categories") {
				input.Categories = splitCSV(categories)
				changed = true
			}
			if cmd.Flags().Changed("account-manager-id") {
				input.AccountManagerID = stringPtr(accountManagerID)
				changed = true
			}

			if !changed {
				return fmt.Errorf("no fields provided: pass at least one flag to update")
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}
			query := app.ResolvedQuery("PATCH", "/clients/{client_id}", nil)
			plan := RequestPlan{
				CommandPath:   "clients update",
				Method:        "PATCH",
				Endpoint:      "/clients/" + args[0],
				Query:         query,
				Body:          body,
				RequiredScope: app.RequiredScope("PATCH", "/clients/{client_id}"),
			}
			if app.MaybeDryRun(plan) {
				return nil
			}

			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			var updated api.ClientResource
			err = client.Do(cmd.Context(), "PATCH", "/clients/"+args[0], query, body, &updated)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(updated)
			}

			app.Printer().PrintMessage("Updated client: %s", updated.ID)
			app.Printer().PrintMessage("Name: %s", updated.CompanyName)
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

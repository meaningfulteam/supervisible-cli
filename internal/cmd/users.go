package cmd

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/output"
)

func newUsersCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Manage users",
	}

	cmd.AddCommand(
		newUsersListCommand(),
		newUsersUpdateCommand(),
	)

	return cmd
}

func newUsersListCommand() *cobra.Command {
	var limit, offset int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
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
			query := app.ResolvedQuery("GET", "/users", baseQuery)

			var users []api.User
			err = client.Do(cmd.Context(), "GET", "/users", query, nil, &users)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(users)
			}

			rows := make([][]string, 0, len(users))
			for _, user := range users {
				rows = append(rows, []string{
					user.ID,
					output.CoalesceString(user.Name),
					user.Email,
					user.UserType,
					strconv.FormatBool(user.IsActive),
				})
			}
			return app.Printer().Table([]string{"ID", "NAME", "EMAIL", "TYPE", "ACTIVE"}, rows)
		},
		Args: cobra.NoArgs,
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Pagination limit")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	return cmd
}

func newUsersUpdateCommand() *cobra.Command {
	var (
		name                string
		image               string
		countryCode         string
		defaultAvailability int
		reportsToID         string
		payload             string
		filePath            string
	)

	cmd := &cobra.Command{
		Use:   "update <user-id>",
		Short: "Update a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := requireUUIDArg("user-id", args[0]); err != nil {
				return err
			}

			input := api.UpdateUserInput{}
			changed := false

			if cmd.Flags().Changed("name") {
				input.Name = stringPtr(name)
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
			if cmd.Flags().Changed("default-availability") {
				input.DefaultAvailability = intPtr(defaultAvailability)
				changed = true
			}
			if cmd.Flags().Changed("reports-to-id") {
				input.ReportsToID = stringPtr(reportsToID)
				changed = true
			}

			if !changed {
				return fmt.Errorf("no fields provided: pass at least one flag to update")
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}

			query := app.ResolvedQuery("PATCH", "/users/{user_id}", nil)
			plan := RequestPlan{
				CommandPath:   "users update",
				Method:        "PATCH",
				Endpoint:      "/users/" + args[0],
				Query:         query,
				Body:          body,
				RequiredScope: app.RequiredScope("PATCH", "/users/{user_id}"),
			}
			if app.MaybeDryRun(plan) {
				return nil
			}

			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			var user api.User
			err = client.Do(cmd.Context(), "PATCH", "/users/"+args[0], query, body, &user)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(user)
			}

			app.Printer().PrintMessage("Updated user: %s", user.ID)
			app.Printer().PrintMessage("Name: %s", output.CoalesceString(user.Name))
			app.Printer().PrintMessage("Email: %s", user.Email)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Display name")
	cmd.Flags().StringVar(&image, "image", "", "Avatar URL")
	cmd.Flags().StringVar(&countryCode, "country-code", "", "ISO country code")
	cmd.Flags().IntVar(&defaultAvailability, "default-availability", 0, "Default weekly availability (hours)")
	cmd.Flags().StringVar(&reportsToID, "reports-to-id", "", "Manager user ID")
	cmd.Flags().StringVar(&payload, "payload", "", "Raw JSON payload object")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to JSON payload file")
	cmd.MarkFlagsMutuallyExclusive("payload", "file")

	return cmd
}

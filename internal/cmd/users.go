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
	var (
		limit, offset int
		nameFilter    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		Example: `  # List active users
  supervisible users list

  # Find by name (case-insensitive substring)
  supervisible users list --name "miquela" --json

  # Page through results, JSON for agents
  supervisible users list --limit 50 --offset 0 --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			baseQuery := url.Values{}
			baseQuery.Set("limit", strconv.Itoa(limit))
			baseQuery.Set("offset", strconv.Itoa(offset))

			var users []api.User
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "users list",
				Method:      "GET",
				Endpoint:    "/users",
				Query:       baseQuery,
				Out:         &users,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}

			getName := func(u api.User) string { return output.CoalesceString(u.Name) }
			filtered := filterByName(users, nameFilter, getName)
			if nameFilter != "" && len(users) >= limit {
				app.Printer().Aux("note: list was paginated at %d rows before filtering by --name; pass --limit if you expect more", limit)
			}
			if nameFilter != "" && len(filtered) == 0 {
				emitNameMissWarning(app.Printer().Aux, "users", users, nameFilter, getName)
			}

			if app.Printer().IsJSON() {
				return app.PrintData(filtered)
			}

			rows := make([][]string, 0, len(filtered))
			for _, user := range filtered {
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
	cmd.Flags().StringVar(&nameFilter, "name", "", "Case-insensitive substring filter on the user's display name (applied after fetch)")
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
		Args:  argsWithUsage(cobra.ExactArgs(1)),
		Example: `  # Update individual fields via flags
  supervisible users update 019404f3-... --name "Jane Doe"

  # Update via JSON payload (advanced)
  supervisible users update 019404f3-... --body '{"defaultAvailability":32}'`,
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
				input.Name = ptr(name)
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
			if cmd.Flags().Changed("default-availability") {
				input.DefaultAvailability = ptr(defaultAvailability)
				changed = true
			}
			if cmd.Flags().Changed("reports-to-id") {
				input.ReportsToID = ptr(reportsToID)
				changed = true
			}

			if !changed {
				return fmt.Errorf("no fields provided: pass at least one flag to update")
			}

			body, err := mergePayloadWithStruct(payload, filePath, input)
			if err != nil {
				return err
			}

			var user api.User
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "users update",
				Method:      "PATCH",
				Endpoint:    "/users/{user_id}",
				Path:        "/users/" + args[0],
				Body:        body,
				Out:         &user,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}

			if app.Printer().IsJSON() {
				return app.PrintData(user)
			}

			app.Printer().Aux("Updated user: %s", user.ID)
			app.Printer().Aux("Name: %s", output.CoalesceString(user.Name))
			app.Printer().Aux("Email: %s", user.Email)
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

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show authenticated API identity",
		Example: `  # Show the identity associated with the current API key
  supervisible me

  # JSON for agents
  supervisible me --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			client, err := app.RequireClient()
			if err != nil {
				return err
			}

			query := app.ResolvedQuery("GET", "/me", nil)
			var identity map[string]any
			err = client.Do(cmd.Context(), "GET", "/me", query, nil, &identity)
			if err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.PrintData(identity)
			}
			fmt.Fprintf(app.Printer().Stdout(), "Identity: %v\n", identity)
			return nil
		},
	}
}

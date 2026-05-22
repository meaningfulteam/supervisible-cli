package cmd

import (
	"fmt"
	"io"
	"strings"

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
			printMeIdentity(app.Printer().Stdout(), identity)
			return nil
		},
	}
}

// printMeIdentity renders identity as stable key/value lines. Only known fields
// appear; unknown server additions are deliberately omitted (the JSON path is
// the agent escape hatch for novel shapes).
func printMeIdentity(w io.Writer, identity map[string]any) {
	if v, ok := identity["keyName"].(string); ok && v != "" {
		fmt.Fprintf(w, "Key: %s\n", v)
	}
	if v, ok := identity["organizationId"].(string); ok && v != "" {
		fmt.Fprintf(w, "Organization: %s\n", v)
	}
	if v, ok := identity["actorUserId"].(string); ok && v != "" {
		fmt.Fprintf(w, "Actor user: %s\n", v)
	}
	if scopes, ok := identity["scopes"].([]any); ok && len(scopes) > 0 {
		parts := make([]string, 0, len(scopes))
		for _, s := range scopes {
			if str, ok := s.(string); ok {
				parts = append(parts, str)
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(w, "Scopes: %s\n", strings.Join(parts, ", "))
		}
	}
}

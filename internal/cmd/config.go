package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/supervisible/supervisible-cli/internal/api"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}

	cmd.AddCommand(
		newConfigShowCommand(),
		newConfigSetBaseURLCommand(),
	)

	return cmd
}

func newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show effective config",
		Example: `  # Show resolved base URL, config file path, and token source
  supervisible config show`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			payload := map[string]any{
				"base_url":      app.BaseURL(),
				"config_file":   app.ConfigStore().Path(),
				"token_present": app.APIKey() != "",
				"token_source":  app.TokenSource(),
			}

			if app.Printer().IsJSON() {
				return app.Printer().Data(payload)
			}

			w := app.Printer().Stdout()
			fmt.Fprintf(w, "Config file: %s\n", payload["config_file"])
			fmt.Fprintf(w, "Base URL: %s\n", payload["base_url"])
			fmt.Fprintf(w, "Token present: %v\n", payload["token_present"])
			if source, ok := payload["token_source"].(string); ok && source != "" {
				fmt.Fprintf(w, "Token source: %s\n", source)
			}
			return nil
		},
	}
}

func newConfigSetBaseURLCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-base-url <url>",
		Short: "Set default base URL",
		Args:  argsWithUsage(cobra.ExactArgs(1)),
		Example: `  # Point the CLI at a different environment
  supervisible config set-base-url https://staging.supervisible.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			normalized, err := api.NormalizeBaseURL(args[0])
			if err != nil {
				return err
			}

			if err := app.ConfigStore().SaveBaseURL(normalized); err != nil {
				return fmt.Errorf("save base url: %w", err)
			}

			if app.Printer().IsJSON() {
				return app.Printer().Data(map[string]any{
					"base_url":  normalized,
					"persisted": true,
				})
			}
			app.Printer().Aux("Default base URL saved: %s", normalized)
			return nil
		},
	}
	return cmd
}

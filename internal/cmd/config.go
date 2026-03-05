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
				return app.Printer().PrintJSON(payload)
			}

			app.Printer().PrintMessage("Config file: %s", payload["config_file"])
			app.Printer().PrintMessage("Base URL: %s", payload["base_url"])
			app.Printer().PrintMessage("Token present: %v", payload["token_present"])
			if source, ok := payload["token_source"].(string); ok && source != "" {
				app.Printer().PrintMessage("Token source: %s", source)
			}
			return nil
		},
	}
}

func newConfigSetBaseURLCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-base-url <url>",
		Short: "Set default base URL",
		Args:  cobra.ExactArgs(1),
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
				return app.Printer().PrintJSON(map[string]any{
					"base_url":  normalized,
					"persisted": true,
				})
			}
			app.Printer().PrintMessage("Default base URL saved: %s", normalized)
			return nil
		},
	}
	return cmd
}

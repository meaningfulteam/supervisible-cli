package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/output"
)

func newAuthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	cmd.AddCommand(
		newAuthLoginCommand(),
		newAuthStatusCommand(),
		newAuthLogoutCommand(),
		newAuthTokenCommand(),
	)

	return cmd
}

func newAuthLoginCommand() *cobra.Command {
	var (
		apiKey     string
		fromStdin  bool
		skipVerify bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store an API key for future commands",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			token := strings.TrimSpace(apiKey)
			switch {
			case token != "":
				// Use explicit flag value.
			case fromStdin:
				stdinData, readErr := io.ReadAll(bufio.NewReader(cmd.InOrStdin()))
				if readErr != nil {
					return fmt.Errorf("read api key from stdin: %w", readErr)
				}
				token = strings.TrimSpace(string(stdinData))
			default:
				app.Printer().PrintMessage("Paste your Supervisible API key:")
				password, readErr := term.ReadPassword(int(os.Stdin.Fd()))
				if readErr != nil {
					return fmt.Errorf("read api key: %w", readErr)
				}
				app.Printer().PrintMessage("")
				token = strings.TrimSpace(string(password))
			}

			if token == "" {
				return fmt.Errorf("api key is required")
			}

			if !skipVerify {
				client, err := api.NewClient(app.BaseURL(), token, 15*time.Second)
				if err != nil {
					return err
				}
				if _, err := client.Me(cmd.Context()); err != nil {
					return fmt.Errorf("api key verification failed: %w", err)
				}
			}

			source, err := app.ConfigStore().SaveToken(app.BaseURL(), token)
			if err != nil {
				return err
			}
			if err := app.ConfigStore().SaveBaseURL(app.BaseURL()); err != nil {
				return err
			}

			if app.Printer().IsJSON() {
				return app.Printer().PrintJSON(map[string]any{
					"authenticated": true,
					"base_url":      app.BaseURL(),
					"storage":       source,
				})
			}

			app.Printer().PrintMessage("Authentication successful")
			app.Printer().PrintMessage("Base URL: %s", app.BaseURL())
			app.Printer().PrintMessage("Stored in: %s", source)
			app.Printer().PrintMessage("Token: %s", output.MaskToken(token))
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key to store")
	cmd.Flags().BoolVar(&fromStdin, "from-stdin", false, "Read API key from stdin")
	cmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "Skip verification request to /me")

	return cmd
}

func newAuthStatusCommand() *cobra.Command {
	var verify bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			status := map[string]any{
				"base_url":      app.BaseURL(),
				"config_file":   app.ConfigStore().Path(),
				"authenticated": strings.TrimSpace(app.APIKey()) != "",
				"token_source":  app.TokenSource(),
			}

			if verify && strings.TrimSpace(app.APIKey()) != "" {
				client, err := app.RequireClient()
				if err != nil {
					return err
				}
				me, meErr := client.Me(cmd.Context())
				if meErr != nil {
					status["verified"] = false
					status["verification_error"] = meErr.Error()
				} else {
					status["verified"] = true
					status["identity"] = me
				}
			}

			if app.Printer().IsJSON() {
				return app.Printer().PrintJSON(status)
			}

			app.Printer().PrintMessage("Base URL: %s", status["base_url"])
			app.Printer().PrintMessage("Config: %s", status["config_file"])
			app.Printer().PrintMessage("Authenticated: %v", status["authenticated"])
			if source, ok := status["token_source"].(string); ok && source != "" {
				app.Printer().PrintMessage("Token source: %s", source)
			}
			if verify {
				if verified, ok := status["verified"].(bool); ok && verified {
					app.Printer().PrintMessage("Verification: ok")
				} else if errMsg, ok := status["verification_error"].(string); ok && errMsg != "" {
					app.Printer().PrintMessage("Verification: failed (%s)", errMsg)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&verify, "verify", false, "Call /me to verify token")
	return cmd
}

func newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete stored API credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			if err := app.ConfigStore().DeleteToken(app.BaseURL()); err != nil {
				return err
			}
			if app.Printer().IsJSON() {
				return app.Printer().PrintJSON(map[string]any{"logged_out": true})
			}
			app.Printer().PrintMessage("Logged out")
			return nil
		},
	}
}

func newAuthTokenCommand() *cobra.Command {
	var masked bool

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Print current API key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			token := strings.TrimSpace(app.APIKey())
			if token == "" {
				return fmt.Errorf("no api key found")
			}

			if masked {
				token = output.MaskToken(token)
			}
			app.Printer().PrintMessage("%s", token)
			return nil
		},
	}

	cmd.Flags().BoolVar(&masked, "masked", false, "Mask token output")
	return cmd
}

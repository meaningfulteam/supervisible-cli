package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/supervisible/supervisible-cli/internal/version"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		Example: `  # Show CLI version
  supervisible version

  # JSON form
  supervisible version --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}
			payload := map[string]string{
				"version": version.Version,
				"commit":  version.Commit,
				"date":    version.Date,
			}
			if app.Printer().IsJSON() {
				return app.Printer().Data(payload)
			}
			w := app.Printer().Stdout()
			fmt.Fprintf(w, "version: %s\n", version.Version)
			fmt.Fprintf(w, "commit: %s\n", version.Commit)
			fmt.Fprintf(w, "date: %s\n", version.Date)
			return nil
		},
	}
}

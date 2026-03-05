package cmd

import (
	"github.com/spf13/cobra"
	"github.com/supervisible/supervisible-cli/internal/version"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
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
				return app.Printer().PrintJSON(payload)
			}
			app.Printer().PrintMessage("version: %s", version.Version)
			app.Printer().PrintMessage("commit: %s", version.Commit)
			app.Printer().PrintMessage("date: %s", version.Date)
			return nil
		},
	}
}

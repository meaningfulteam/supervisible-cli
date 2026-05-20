package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Inspect public API schema",
	}

	cmd.AddCommand(newSchemaEndpointsCommand())
	cmd.AddCommand(newSchemaDescribeCommand())
	return cmd
}

func newSchemaEndpointsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "endpoints",
		Short: "List available API endpoints",
		Example: `  # List every endpoint in the loaded schema
  supervisible schema endpoints

  # JSON for agents
  supervisible schema endpoints --json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			endpoints := app.schema.Endpoints()
			sort.Slice(endpoints, func(i, j int) bool {
				if endpoints[i].Path == endpoints[j].Path {
					return endpoints[i].Method < endpoints[j].Method
				}
				return endpoints[i].Path < endpoints[j].Path
			})

			if app.Printer().IsJSON() {
				return app.Printer().Data(endpoints)
			}

			rows := make([][]string, 0, len(endpoints))
			for _, endpoint := range endpoints {
				rows = append(rows, []string{endpoint.Operation, endpoint.Method, endpoint.Path, endpoint.RequiredScope, endpoint.Summary})
			}
			return app.Printer().Table([]string{"OPERATION", "METHOD", "PATH", "SCOPE", "SUMMARY"}, rows)
		},
	}
}

func newSchemaDescribeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <operation|\"METHOD /path\">",
		Short: "Describe a specific API operation",
		Args:  argsWithUsage(cobra.ExactArgs(1)),
		Example: `  # Describe by operation ID
  supervisible schema describe assignments.get

  # Describe by method + path
  supervisible schema describe "GET /assignments"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			endpoint, operation, ok := app.schema.Describe(args[0])
			if !ok {
				return fmt.Errorf("operation not found: %s", args[0])
			}

			payload := map[string]any{
				"endpoint": endpoint,
				"operation": map[string]any{
					"summary":        operation.Summary,
					"description":    operation.Description,
					"operation_id":   operation.OperationID,
					"required_scope": operation.RequiredScope,
					"parameters":     operation.Parameters,
					"request_body":   operation.RequestBody,
					"responses":      operation.Responses,
				},
			}

			if app.Printer().IsJSON() {
				return app.Printer().Data(payload)
			}

			w := app.Printer().Stdout()
			fmt.Fprintf(w, "%s %s\n", endpoint.Method, endpoint.Path)
			fmt.Fprintf(w, "Operation: %s\n", endpoint.Operation)
			if endpoint.RequiredScope != "" {
				fmt.Fprintf(w, "Required scope: %s\n", endpoint.RequiredScope)
			}
			if endpoint.Summary != "" {
				fmt.Fprintf(w, "Summary: %s\n", endpoint.Summary)
			}
			return nil
		},
	}
}

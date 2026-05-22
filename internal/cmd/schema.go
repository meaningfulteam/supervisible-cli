package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/schema"
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
		Example: `  # Describe a single operation
  supervisible schema describe assignments.get

  # Describe every operation under a noun
  supervisible schema describe assignments

  # Describe by method + path
  supervisible schema describe "GET /assignments"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			selector := args[0]

			if endpoint, operation, ok := app.schema.Describe(selector); ok {
				return renderDescribed(app, []describedOp{{endpoint, operation}})
			}

			// Bare noun form: "assignments" → "assignments.*"
			if nounMatches := nounLookup(app.schema, selector); len(nounMatches) > 0 {
				return renderDescribed(app, nounMatches)
			}

			suggestions := suggestOperations(app.schema, selector)
			if len(suggestions) == 0 {
				return fmt.Errorf("operation not found: %s", selector)
			}
			return fmt.Errorf("operation not found: %s. Did you mean: %s?", selector, strings.Join(suggestions, ", "))
		},
	}
}

type describedOp struct {
	endpoint  schema.Endpoint
	operation schema.Operation
}

// nounLookup returns every operation whose operation ID has the form "<noun>.<verb>",
// e.g. "assignments.get" / "assignments.post" for noun="assignments". The selector
// must not contain "." or whitespace (i.e. callers have already ruled out the exact-match
// path). Returns results sorted by operation ID for stable output.
func nounLookup(p *schema.Provider, noun string) []describedOp {
	noun = strings.TrimSpace(noun)
	if noun == "" || strings.ContainsAny(noun, ". ") {
		return nil
	}
	prefix := strings.ToLower(noun) + "."
	var out []describedOp
	for _, ep := range p.Endpoints() {
		if !strings.HasPrefix(strings.ToLower(ep.Operation), prefix) {
			continue
		}
		_, op, ok := p.Describe(ep.Operation)
		if !ok {
			continue
		}
		out = append(out, describedOp{ep, op})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].endpoint.Operation < out[j].endpoint.Operation })
	return out
}

// suggestOperations returns up to 5 operation IDs whose lowercase form contains the
// selector or shares the same prefix. Cheap "did you mean" without an edit-distance lib.
func suggestOperations(p *schema.Provider, selector string) []string {
	needle := strings.ToLower(strings.TrimSpace(selector))
	if needle == "" {
		return nil
	}
	// Treat the part before the dot as a noun candidate too.
	noun := needle
	if i := strings.Index(needle, "."); i > 0 {
		noun = needle[:i]
	}

	seen := map[string]struct{}{}
	var out []string
	for _, ep := range p.Endpoints() {
		op := strings.ToLower(ep.Operation)
		if !strings.Contains(op, needle) && !strings.HasPrefix(op, noun+".") {
			continue
		}
		if _, ok := seen[ep.Operation]; ok {
			continue
		}
		seen[ep.Operation] = struct{}{}
		out = append(out, ep.Operation)
	}
	sort.Strings(out)
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func renderDescribed(app *App, ops []describedOp) error {
	payloads := make([]map[string]any, 0, len(ops))
	for _, o := range ops {
		payloads = append(payloads, map[string]any{
			"endpoint": o.endpoint,
			"operation": map[string]any{
				"summary":        o.operation.Summary,
				"description":    o.operation.Description,
				"operation_id":   o.operation.OperationID,
				"required_scope": o.operation.RequiredScope,
				"parameters":     o.operation.Parameters,
				"request_body":   o.operation.RequestBody,
				"responses":      o.operation.Responses,
			},
		})
	}

	if app.Printer().IsJSON() {
		if len(payloads) == 1 {
			return app.Printer().Data(payloads[0])
		}
		return app.Printer().Data(payloads)
	}

	w := app.Printer().Stdout()
	for i, o := range ops {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "%s %s\n", o.endpoint.Method, o.endpoint.Path)
		fmt.Fprintf(w, "Operation: %s\n", o.endpoint.Operation)
		if o.endpoint.RequiredScope != "" {
			fmt.Fprintf(w, "Required scope: %s\n", o.endpoint.RequiredScope)
		}
		if o.endpoint.Summary != "" {
			fmt.Fprintf(w, "Summary: %s\n", o.endpoint.Summary)
		}
	}
	return nil
}

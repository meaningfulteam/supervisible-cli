package cmd

import (
	"net/url"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/api"
)

// capabilitySource marks the provenance of a row. When a canonical
// GET /capabilities endpoint lands, swap this to "canonical" and drop the
// derived-view warning in newCapabilitiesListCommand.
const capabilitySourceDerived = "derived-from-assignments"

// DerivedCapability is the per-row payload emitted by `capabilities list`. The
// shape is deliberately small so it doesn't lock us in before a real
// /capabilities endpoint exists.
type DerivedCapability struct {
	CapabilityID string `json:"capabilityId"`
	Name         string `json:"name"`
	UsageCount   int    `json:"usageCount"`
	Source       string `json:"source"`
}

func newCapabilitiesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Inspect capabilities (derived from assignment history)",
	}

	cmd.AddCommand(newCapabilitiesListCommand())
	return cmd
}

func newCapabilitiesListCommand() *cobra.Command {
	var (
		projectID string
		limit     int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List capabilities in use on a project",
		Long: `List the capabilities that currently appear on a project's assignments.

Because the public API does not yet expose GET /capabilities, this result is
derived from /assignments?project_id=<id>&expand=capability and reflects only
capabilities that have been used. New capabilities with no assignment history
will not appear. Output rows carry source="derived-from-assignments" so
consumers can detect the provenance.`,
		Example: `  # List capabilities used on a project
  supervisible capabilities list --for-project 019c885e-... --json

  # Cap how deep the scan goes
  supervisible capabilities list --for-project 019c885e-... --limit 50`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireUUIDArg("for-project", projectID); err != nil {
				return err
			}

			app, err := appFromCommand(cmd)
			if err != nil {
				return err
			}

			q := url.Values{}
			q.Set("project_id", projectID)
			q.Set("expand", "capability")
			q.Set("limit", strconv.Itoa(limit))

			var assignments []api.Assignment
			executed, err := app.Execute(cmd.Context(), ExecuteOpts{
				CommandPath: "capabilities list",
				Method:      "GET",
				Endpoint:    "/assignments",
				Query:       q,
				Out:         &assignments,
			})
			if err != nil {
				return err
			}
			if !executed {
				return nil
			}

			derived := aggregateDerivedCapabilities(assignments)

			if len(derived) == 0 {
				app.Printer().Aux("note: no capabilities found via assignment history. Project may be new or unstaffed.")
			} else {
				app.Printer().Aux("warning: capability list is derived from assignment history (no GET /capabilities endpoint). May be incomplete.")
			}

			if app.Printer().IsJSON() {
				return app.PrintData(derived)
			}

			rows := make([][]string, 0, len(derived))
			for _, d := range derived {
				rows = append(rows, []string{
					d.CapabilityID,
					d.Name,
					strconv.Itoa(d.UsageCount),
					d.Source,
				})
			}
			return app.Printer().Table([]string{"ID", "NAME", "USAGE", "SOURCE"}, rows)
		},
	}

	cmd.Flags().StringVar(&projectID, "for-project", "", "Project ID to derive capabilities from (required)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max assignments to scan when aggregating")
	_ = cmd.MarkFlagRequired("for-project")
	return cmd
}

// aggregateDerivedCapabilities collapses assignments into one row per
// capabilityId with a usage count. Skips zombie rows (hours <= 0) and
// assignments missing a capabilityId. Result is sorted by usageCount desc,
// then by name asc for stable output.
func aggregateDerivedCapabilities(assignments []api.Assignment) []DerivedCapability {
	type acc struct {
		name  string
		count int
	}
	buckets := map[string]*acc{}
	for _, a := range assignments {
		if a.Hours <= 0 {
			continue
		}
		if a.CapabilityID == nil || *a.CapabilityID == "" {
			continue
		}
		capID := *a.CapabilityID
		b, ok := buckets[capID]
		if !ok {
			b = &acc{}
			buckets[capID] = b
		}
		if b.name == "" && a.Capability != nil {
			b.name = a.Capability.Name
		}
		b.count++
	}

	out := make([]DerivedCapability, 0, len(buckets))
	for id, b := range buckets {
		out = append(out, DerivedCapability{
			CapabilityID: id,
			Name:         b.name,
			UsageCount:   b.count,
			Source:       capabilitySourceDerived,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UsageCount != out[j].UsageCount {
			return out[i].UsageCount > out[j].UsageCount
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].CapabilityID < out[j].CapabilityID
	})
	return out
}

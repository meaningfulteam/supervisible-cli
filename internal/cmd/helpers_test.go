package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestArgsWithUsage_PrintsUsageOnFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{
		Use:     "leaf <id>",
		Example: "  example call",
		Args:    argsWithUsage(cobra.ExactArgs(1)),
		RunE:    func(cmd *cobra.Command, args []string) error { return nil },
	}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "example call") {
		t.Fatalf("expected usage with example, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestArgsWithUsage_DoesNotPrintOnSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{
		Use:     "leaf <id>",
		Example: "  example call",
		Args:    argsWithUsage(cobra.ExactArgs(1)),
		RunE:    func(cmd *cobra.Command, args []string) error { return nil },
	}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"abc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	combined := stdout.String() + stderr.String()
	if strings.Contains(combined, "example call") {
		t.Fatalf("expected no usage on success, got: %q", combined)
	}
}

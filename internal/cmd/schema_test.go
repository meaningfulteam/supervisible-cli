package cmd

import (
	"strings"
	"testing"
)

func TestSchemaDescribe_ExactOperation(t *testing.T) {
	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"schema", "describe", "assignments.get",
	)
	if err != nil {
		t.Fatalf("describe assignments.get failed: %v", err)
	}
	if !strings.Contains(stdout, "GET /assignments") || !strings.Contains(stdout, "assignments.get") {
		t.Fatalf("expected GET /assignments + assignments.get in stdout, got: %q", stdout)
	}
	if strings.Contains(stdout, "POST") {
		t.Fatalf("expected only the .get operation, got: %q", stdout)
	}
}

func TestSchemaDescribe_NounListsAllOps(t *testing.T) {
	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"schema", "describe", "assignments",
	)
	if err != nil {
		t.Fatalf("describe assignments (noun) failed: %v", err)
	}
	if !strings.Contains(stdout, "assignments.get") {
		t.Fatalf("expected assignments.get in noun-form output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "assignments.post") {
		t.Fatalf("expected assignments.post in noun-form output, got: %q", stdout)
	}
}

func TestSchemaDescribe_TypoOffersSuggestions(t *testing.T) {
	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"schema", "describe", "assignment",
	)
	if err == nil {
		t.Fatalf("expected error for typo, got nil")
	}
	if !strings.Contains(err.Error(), "Did you mean") {
		t.Fatalf("expected 'Did you mean' hint, got: %v", err)
	}
	if !strings.Contains(err.Error(), "assignments.") {
		t.Fatalf("expected assignments.* suggestion, got: %v", err)
	}
}

func TestSchemaDescribe_UnknownReturnsError(t *testing.T) {
	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"schema", "describe", "totally-not-a-thing",
	)
	if err == nil {
		t.Fatalf("expected error for unknown selector, got nil")
	}
	if !strings.Contains(err.Error(), "operation not found") {
		t.Fatalf("expected 'operation not found', got: %v", err)
	}
}

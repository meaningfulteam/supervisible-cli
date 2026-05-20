package cmd

import (
	"strings"
	"testing"
)

func TestParamsWarning_UnknownKeyFires(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://example.invalid")

	_, stderr, _ := executeCLI(t,
		"--config", testConfigPath(t),
		"--dry-run",
		"--params", `{"startDate":"2026-05-18"}`,
		"assignments", "list",
	)
	if !strings.Contains(stderr, `warning: unknown query param "startDate"`) {
		t.Fatalf("expected warning for camelCase typo, got stderr=%q", stderr)
	}
	if !strings.Contains(stderr, "allowed:") {
		t.Fatalf("expected allowed-list in warning, got: %q", stderr)
	}
}

func TestParamsWarning_KnownKeysSilent(t *testing.T) {
	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", "http://example.invalid")

	_, stderr, _ := executeCLI(t,
		"--config", testConfigPath(t),
		"--dry-run",
		"--params", `{"start_date":"2026-05-18","limit":10}`,
		"assignments", "list",
	)
	if strings.Contains(stderr, "warning:") {
		t.Fatalf("expected no warning for known params, got: %q", stderr)
	}
}

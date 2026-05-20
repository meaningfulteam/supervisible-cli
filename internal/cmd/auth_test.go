package cmd

import (
	"strings"
	"testing"
)

// Under `go test`, stdin is not a TTY, so the non-interactive guard should fire.
func TestAuthLogin_NonInteractiveGuardReturnsFriendlyError(t *testing.T) {
	if isStdinInteractive() {
		t.Skip("stdin is a TTY in this environment; can't test non-interactive guard")
	}

	_, _, err := executeCLI(t, "--config", testConfigPath(t), "auth", "login")
	if err == nil {
		t.Fatalf("expected non-interactive guard error, got nil")
	}
	if !strings.Contains(err.Error(), "api key required") {
		t.Fatalf("expected friendly hint, got: %v", err)
	}
}

package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func executeCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	command := NewRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs(args)

	err := command.Execute()
	return stdout.String(), stderr.String(), err
}

func testConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.json")
}

func TestUsersListParamsOverrideAndLocalProjection(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"u1","email":"a@b.com","userType":"admin","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"--json",
		"--params", `{"limit":10,"offset":2}`,
		"--fields", "id",
		"users", "list",
		"--limit", "50",
		"--offset", "1",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if !strings.Contains(capturedQuery, "limit=10") || !strings.Contains(capturedQuery, "offset=2") {
		t.Fatalf("expected raw params to override limit/offset, query=%q", capturedQuery)
	}

	var got []map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if len(got) != 1 || len(got[0]) != 1 || got[0]["id"] != "u1" {
		t.Fatalf("unexpected projected output: %v", got)
	}
}

func TestUsersUpdatePayloadOverridesFlags(t *testing.T) {
	const userID = "550e8400-e29b-41d4-a716-446655440000"

	var capturedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/users/"+userID {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if err := json.Unmarshal(raw, &capturedBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"550e8400-e29b-41d4-a716-446655440000","email":"a@b.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, _, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"--json",
		"users", "update", userID,
		"--name", "FlagName",
		"--payload", `{"name":"RawName"}`,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if got, ok := capturedBody["name"].(string); !ok || got != "RawName" {
		t.Fatalf("expected payload to override flag, body=%v", capturedBody)
	}
}

func TestDryRunMutatingCommandSkipsClientRequirement(t *testing.T) {
	const clientID = "550e8400-e29b-41d4-a716-446655440001"

	t.Setenv("SUPERVISIBLE_API_KEY", "")
	t.Setenv("SUPERVISIBLE_BASE_URL", "")

	stdout, _, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"--json",
		"--dry-run",
		"projects", "create",
		"--name", "Proj",
		"--client-id", clientID,
		"--start-date", "2026-01-01",
		"--end-date", "2026-01-31",
	)
	if err != nil {
		t.Fatalf("dry-run should not require api key: %v", err)
	}
	if !strings.Contains(stdout, `"will_execute": false`) {
		t.Fatalf("expected dry-run plan output, got: %s", stdout)
	}
}

func TestDryRunMutatingCommandMakesNoHTTPRequest(t *testing.T) {
	const clientID = "550e8400-e29b-41d4-a716-446655440002"

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, _, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"--json",
		"--dry-run",
		"projects", "create",
		"--name", "Proj",
		"--client-id", clientID,
		"--start-date", "2026-01-01",
		"--end-date", "2026-01-31",
	)
	if err != nil {
		t.Fatalf("dry-run command failed: %v", err)
	}

	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("expected zero HTTP calls in dry-run, got %d", got)
	}
}

func TestPayloadAndFileAreMutuallyExclusive(t *testing.T) {
	const userID = "550e8400-e29b-41d4-a716-446655440003"

	payloadFile := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(payloadFile, []byte(`{"name":"from-file"}`), 0o600); err != nil {
		t.Fatalf("write payload file: %v", err)
	}

	_, stderr, err := executeCLI(
		t,
		"--config", testConfigPath(t),
		"--json",
		"users", "update", userID,
		"--payload", `{"name":"from-flag"}`,
		"--file", payloadFile,
	)
	if err == nil {
		t.Fatalf("expected mutual exclusivity error")
	}
	if !strings.Contains(err.Error(), "none of the others can be") && !strings.Contains(stderr, "none of the others can be") {
		t.Fatalf("expected mutually exclusive error, err=%v stderr=%q", err, stderr)
	}
}

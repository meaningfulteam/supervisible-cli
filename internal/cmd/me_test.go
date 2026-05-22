package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrintMeIdentity_AllFieldsInStableOrder(t *testing.T) {
	var buf bytes.Buffer
	printMeIdentity(&buf, map[string]any{
		"keyName":        "test-key",
		"organizationId": "org-uuid",
		"actorUserId":    "user-uuid",
		"scopes":         []any{"read:users", "write:assignments"},
	})
	want := "Key: test-key\nOrganization: org-uuid\nActor user: user-uuid\nScopes: read:users, write:assignments\n"
	if buf.String() != want {
		t.Fatalf("unexpected output:\nwant: %q\ngot:  %q", want, buf.String())
	}
}

func TestPrintMeIdentity_OmitsMissingFields(t *testing.T) {
	var buf bytes.Buffer
	printMeIdentity(&buf, map[string]any{
		"organizationId": "org-uuid",
		"scopes":         []any{"*"},
	})
	out := buf.String()
	if strings.Contains(out, "Key:") {
		t.Fatalf("expected no Key line, got: %q", out)
	}
	if strings.Contains(out, "Actor user:") {
		t.Fatalf("expected no Actor user line, got: %q", out)
	}
	if !strings.Contains(out, "Organization: org-uuid") {
		t.Fatalf("expected organization line, got: %q", out)
	}
	if !strings.Contains(out, "Scopes: *") {
		t.Fatalf("expected scopes line, got: %q", out)
	}
}

func TestPrintMeIdentity_NoMapSyntaxLeak(t *testing.T) {
	var buf bytes.Buffer
	printMeIdentity(&buf, map[string]any{
		"keyName":        "test-key",
		"organizationId": "org-uuid",
	})
	if strings.Contains(buf.String(), "map[") {
		t.Fatalf("expected no Go map syntax in output, got: %q", buf.String())
	}
}

func TestMeCommand_JSONUnchanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"keyName":"k","organizationId":"o","actorUserId":"u","scopes":["*"]}}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"me",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if got["keyName"] != "k" || got["organizationId"] != "o" {
		t.Fatalf("unexpected json: %v", got)
	}
}

func TestMeCommand_NonJSONRendersTypedLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"keyName":"k","organizationId":"o","actorUserId":"u","scopes":["*"]}}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"me",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, want := range []string{"Key: k", "Organization: o", "Actor user: u", "Scopes: *"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in stdout, got: %q", want, stdout)
		}
	}
	if strings.Contains(stdout, "map[") {
		t.Fatalf("expected no Go map syntax, got: %q", stdout)
	}
}

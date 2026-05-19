package output

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/supervisible/supervisible-cli/internal/api"
)

func TestFormatCLIError_StatusHints(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{401, "Authentication failed"},
		{403, "Forbidden"},
		{404, "Not found"},
		{409, "Conflict"},
		{422, "Validation failed"},
		{429, "Rate limited"},
		{500, "Server error (500)"},
		{503, "Server error (503)"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			err := &api.APIError{StatusCode: tc.status, Message: "msg"}
			got := FormatCLIError(err)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("expected %q in output, got: %q", tc.want, got)
			}
		})
	}
}

func TestFormatCLIError_RequestIDRendered(t *testing.T) {
	err := &api.APIError{StatusCode: 404, RequestID: "01HZ-abc"}
	got := FormatCLIError(err)
	if !strings.Contains(got, "Request ID: 01HZ-abc") {
		t.Fatalf("expected Request ID line, got: %q", got)
	}
}

func TestFormatCLIError_NoRequestIDLineWhenEmpty(t *testing.T) {
	err := &api.APIError{StatusCode: 404}
	got := FormatCLIError(err)
	if strings.Contains(got, "Request ID") {
		t.Fatalf("expected no Request ID line, got: %q", got)
	}
}

func TestFormatCLIError_401SkipsVerboseHint(t *testing.T) {
	err := &api.APIError{StatusCode: 401}
	got := FormatCLIError(err)
	if strings.Contains(got, "--verbose") {
		t.Fatalf("expected no --verbose hint for 401, got: %q", got)
	}
}

func TestFormatCLIError_NonAuthIncludesVerboseHint(t *testing.T) {
	err := &api.APIError{StatusCode: 404}
	got := FormatCLIError(err)
	if !strings.Contains(got, "--verbose") {
		t.Fatalf("expected --verbose hint, got: %q", got)
	}
}

func TestFormatCLIError_WrappedAPIError(t *testing.T) {
	inner := &api.APIError{StatusCode: 422, Message: "name required"}
	wrapped := fmt.Errorf("create project: %w", inner)
	got := FormatCLIError(wrapped)
	if !strings.Contains(got, "Validation failed") {
		t.Fatalf("expected hint for wrapped APIError, got: %q", got)
	}
}

func TestFormatCLIError_PlainErrorFallsThrough(t *testing.T) {
	got := FormatCLIError(errors.New("network down"))
	if got != "Error: network down" {
		t.Fatalf("expected fallthrough message, got: %q", got)
	}
}

func TestFormatCLIError_NilReturnsEmpty(t *testing.T) {
	if got := FormatCLIError(nil); got != "" {
		t.Fatalf("expected empty string for nil error, got: %q", got)
	}
}

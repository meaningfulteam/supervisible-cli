package output

import (
	"errors"
	"fmt"
	"strings"

	"github.com/supervisible/supervisible-cli/internal/api"
)

// FormatCLIError renders an error for the user. APIError values get a
// status-code-aware hint plus optional Request ID line; everything else
// falls through to "Error: <err>".
func FormatCLIError(err error) string {
	if err == nil {
		return ""
	}

	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return "Error: " + err.Error()
	}

	hint := hintForStatus(apiErr)
	var b strings.Builder
	fmt.Fprintf(&b, "Error: %s", hint)
	if apiErr.RequestID != "" {
		fmt.Fprintf(&b, "\nRequest ID: %s", apiErr.RequestID)
	}
	if apiErr.StatusCode != 401 {
		b.WriteString("\nRun with --verbose to see the full request.")
	}
	return b.String()
}

func hintForStatus(e *api.APIError) string {
	switch {
	case e.StatusCode == 401:
		return "Authentication failed. Run: supervisible auth login --api-key sv_live_..."
	case e.StatusCode == 403:
		return "Forbidden. Your API key doesn't have access to this resource. Check key scopes with 'supervisible auth status --verify'."
	case e.StatusCode == 404:
		return "Not found — verify the resource ID, or it may not be shared with this API key."
	case e.StatusCode == 409:
		return "Conflict. The resource state changed; refetch and retry."
	case e.StatusCode == 422:
		msg := strings.TrimSpace(e.Message)
		if msg == "" {
			msg = "request rejected"
		}
		return fmt.Sprintf("Validation failed: %s. Try --dry-run to inspect the payload.", msg)
	case e.StatusCode == 429:
		return "Rate limited. Retry after a few seconds."
	case e.StatusCode >= 500 && e.StatusCode < 600:
		return fmt.Sprintf("Server error (%d). Try again; contact support with the request ID if persistent.", e.StatusCode)
	}
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("API error (%d %s)", e.StatusCode, e.Code)
}

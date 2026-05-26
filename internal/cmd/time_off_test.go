package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const timeOffTestUserID = "550e8400-e29b-41d4-a716-446655440100"

type timeOffServer struct {
	server          *httptest.Server
	types           []map[string]any // returned by GET /time-off-types
	gotCreateBody   atomic.Value     // string
	createTypeID    atomic.Value     // string
	createResponses []map[string]any
}

func newTimeOffServer(t *testing.T, types []map[string]any) *timeOffServer {
	t.Helper()
	s := &timeOffServer{types: types}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/time-off-types"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": s.types})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/time-off"):
			raw, _ := io.ReadAll(r.Body)
			s.gotCreateBody.Store(string(raw))
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			if tid, ok := body["timeOffTypeId"].(string); ok {
				s.createTypeID.Store(tid)
			}
			created := map[string]any{
				"id":            "request-1",
				"userId":        body["userId"],
				"timeOffTypeId": body["timeOffTypeId"],
				"startDate":     body["startDate"],
				"endDate":       body["endDate"],
				"status":        "pending",
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": created})
		default:
			http.NotFound(w, r)
		}
	}))
	return s
}

func timeOffTypeRow(id, name string) map[string]any {
	return map[string]any{
		"id":          id,
		"name":        name,
		"description": nil,
		"createdAt":   "2026-01-01T00:00:00Z",
		"updatedAt":   "2026-01-01T00:00:00Z",
	}
}

func TestTimeOffCreate_TypeNameResolvesToID(t *testing.T) {
	const vacationID = "550e8400-e29b-41d4-a716-446655440011"
	s := newTimeOffServer(t, []map[string]any{
		timeOffTypeRow(vacationID, "Vacation"),
		timeOffTypeRow("550e8400-e29b-41d4-a716-446655440022", "Sick Leave"),
	})
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"time-off", "create",
		"--user-id", timeOffTestUserID,
		"--type-name", "Vacation",
		"--start-date", "2026-07-15",
		"--end-date", "2026-07-19",
		"--reason", "Family trip",
	)
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, stderr)
	}
	if got, _ := s.createTypeID.Load().(string); got != vacationID {
		t.Fatalf("expected POST body timeOffTypeId=%s, got %q", vacationID, got)
	}
	if !strings.Contains(stderr, "time-off type resolved") {
		t.Fatalf("expected resolution stderr, got: %q", stderr)
	}
}

func TestTimeOffCreate_TypeNameIsCaseInsensitive(t *testing.T) {
	const vacationID = "550e8400-e29b-41d4-a716-446655440011"
	s := newTimeOffServer(t, []map[string]any{timeOffTypeRow(vacationID, "Vacation")})
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"time-off", "create",
		"--user-id", timeOffTestUserID,
		"--type-name", "VACATION",
		"--start-date", "2026-07-15",
		"--end-date", "2026-07-19",
		"--reason", "x",
	)
	if err != nil {
		t.Fatalf("expected case-insensitive match to succeed: %v", err)
	}
}

func TestTimeOffCreate_TypeNameMissReportsAvailable(t *testing.T) {
	s := newTimeOffServer(t, []map[string]any{
		timeOffTypeRow("a", "Vacation"),
		timeOffTypeRow("b", "Sick Leave"),
	})
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"time-off", "create",
		"--user-id", timeOffTestUserID,
		"--type-name", "Holidays",
		"--start-date", "2026-07-15",
		"--end-date", "2026-07-19",
		"--reason", "x",
	)
	if err == nil {
		t.Fatalf("expected error for unknown type name")
	}
	msg := err.Error()
	if !strings.Contains(msg, "no time-off type named") || !strings.Contains(msg, "Vacation") {
		t.Fatalf("expected helpful error listing available types, got: %v", err)
	}
}

func TestTimeOffCreate_RejectsBothTypeIDAndTypeName(t *testing.T) {
	s := newTimeOffServer(t, []map[string]any{timeOffTypeRow("a", "Vacation")})
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"time-off", "create",
		"--user-id", timeOffTestUserID,
		"--time-off-type-id", "550e8400-e29b-41d4-a716-446655440011",
		"--type-name", "Vacation",
		"--start-date", "2026-07-15",
		"--end-date", "2026-07-19",
		"--reason", "x",
	)
	if err == nil {
		t.Fatalf("expected error for combining --time-off-type-id and --type-name")
	}
}

func TestTimeOffCreate_RequiresOneOfTypeIDOrName(t *testing.T) {
	s := newTimeOffServer(t, nil)
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"time-off", "create",
		"--user-id", timeOffTestUserID,
		"--start-date", "2026-07-15",
		"--end-date", "2026-07-19",
		"--reason", "x",
	)
	if err == nil {
		t.Fatalf("expected error when neither --time-off-type-id nor --type-name provided")
	}
}

func TestTimeOffCreate_ExplicitTypeIDSkipsLookup(t *testing.T) {
	s := newTimeOffServer(t, nil) // no types — would fail name resolution
	defer s.server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", s.server.URL)

	const typeID = "550e8400-e29b-41d4-a716-446655440099"
	_, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"time-off", "create",
		"--user-id", timeOffTestUserID,
		"--time-off-type-id", typeID,
		"--start-date", "2026-07-15",
		"--end-date", "2026-07-19",
		"--reason", "x",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if got, _ := s.createTypeID.Load().(string); got != typeID {
		t.Fatalf("expected POST to carry typeID=%s, got %q", typeID, got)
	}
}

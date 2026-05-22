package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeNamed struct {
	Name string
}

func nameOf(n fakeNamed) string { return n.Name }

func TestFilterByName_CaseInsensitiveSubstring(t *testing.T) {
	items := []fakeNamed{{"Mariana Miquelajauregui"}, {"Juan Méndez"}, {"Herbert Hertz"}}
	got := filterByName(items, "MIQUELA", nameOf)
	if len(got) != 1 || got[0].Name != "Mariana Miquelajauregui" {
		t.Fatalf("expected single Mariana row, got %+v", got)
	}
}

func TestFilterByName_EmptyNeedleReturnsAll(t *testing.T) {
	items := []fakeNamed{{"a"}, {"b"}}
	got := filterByName(items, "", nameOf)
	if len(got) != 2 {
		t.Fatalf("expected unchanged input on empty needle, got %+v", got)
	}
}

func TestFilterByName_NoMatchReturnsEmpty(t *testing.T) {
	items := []fakeNamed{{"a"}, {"b"}}
	got := filterByName(items, "zzz", nameOf)
	if len(got) != 0 {
		t.Fatalf("expected empty slice on no match, got %+v", got)
	}
}

func TestFilterByName_WhitespaceNeedleTreatedAsEmpty(t *testing.T) {
	items := []fakeNamed{{"a"}, {"b"}}
	got := filterByName(items, "   ", nameOf)
	if len(got) != 2 {
		t.Fatalf("expected whitespace-only needle to skip filtering, got %+v", got)
	}
}

func TestUsersList_NameFilterAppliedPostFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"u1","name":"Mariana Miquelajauregui","email":"m@m8l.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"},
			{"id":"u2","name":"Juan Méndez","email":"j@m8l.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}
		]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, _, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"users", "list",
		"--name", "miquela",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if len(got) != 1 || got[0]["id"] != "u1" {
		t.Fatalf("expected only Mariana; got %v", got)
	}
}

func TestUsersList_NameFilterPaginationNote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 2 rows returned and --limit=2 → pagination note should fire.
		_, _ = w.Write([]byte(`{"data":[
			{"id":"u1","name":"Mariana","email":"a@b.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"},
			{"id":"u2","name":"Juan","email":"c@d.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}
		]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"users", "list",
		"--name", "juan",
		"--limit", "2",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(stderr, "paginated at 2 rows") {
		t.Fatalf("expected pagination note on stderr, got: %q", stderr)
	}
}

func TestSuggestNames_SubstringMatchesRankFirst(t *testing.T) {
	items := []fakeNamed{{"Iniciativa Q4"}, {"F25 Migration"}, {"Iniciativa F40"}, {"Marketplace"}}
	hints := suggestNames(items, "F40", nameOf, 3)
	if len(hints) == 0 || hints[0] != "Iniciativa F40" {
		t.Fatalf("expected Iniciativa F40 ranked first, got %v", hints)
	}
}

func TestSuggestNames_EmptyNeedleReturnsNil(t *testing.T) {
	items := []fakeNamed{{"a"}}
	if hints := suggestNames(items, "", nameOf, 5); hints != nil {
		t.Fatalf("expected nil on empty needle, got %v", hints)
	}
}

func TestSuggestNames_LimitsToMax(t *testing.T) {
	items := []fakeNamed{{"Avila"}, {"Avalon"}, {"Avocado"}, {"Avocet"}, {"Aviary"}, {"Avenue"}}
	hints := suggestNames(items, "av", nameOf, 3)
	if len(hints) != 3 {
		t.Fatalf("expected max 3 hints, got %d: %v", len(hints), hints)
	}
}

func TestSuggestNames_NothingCloseReturnsNil(t *testing.T) {
	items := []fakeNamed{{"abc"}, {"def"}}
	if hints := suggestNames(items, "xyz", nameOf, 5); hints != nil {
		t.Fatalf("expected nil for nothing close, got %v", hints)
	}
}

func TestUsersList_NameFilterEmitsSuggestionOnZeroMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"u1","name":"Mariana Miquelajauregui","email":"a@b.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"},
			{"id":"u2","name":"Marina Diaz","email":"c@d.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}
		]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	stdout, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"users", "list",
		"--name", "marain",
	)
	if err != nil {
		t.Fatalf("command failed: %v\n%s", err, stderr)
	}
	if !strings.Contains(stderr, "Did you mean") {
		t.Fatalf("expected Did-you-mean on stderr, got: %q", stderr)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected stdout=[] on no match, got: %q", stdout)
	}
}

func TestUsersList_NameFilterNoPaginationNoteWhenBelowLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"u1","name":"Mariana","email":"a@b.com","userType":"member","isActive":true,"defaultAvailability":40,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}
		]}`))
	}))
	defer server.Close()

	t.Setenv("SUPERVISIBLE_API_KEY", "test-token")
	t.Setenv("SUPERVISIBLE_BASE_URL", server.URL)

	_, stderr, err := executeCLI(t,
		"--config", testConfigPath(t),
		"--json",
		"users", "list",
		"--name", "mariana",
		"--limit", "50",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.Contains(stderr, "paginated at") {
		t.Fatalf("did not expect pagination note (1 row < limit 50), got: %q", stderr)
	}
}

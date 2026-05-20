package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/supervisible/supervisible-cli/internal/api"
)

func newCapResolverWithFixture(t *testing.T, rows []api.Assignment) (*capabilityResolver, *int, func()) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
	}))
	client, err := api.NewClient(server.URL, "test-token", 5*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return newCapabilityResolver(client), &calls, server.Close
}

func TestCapabilityResolver_PicksMostRecentNonZeroRow(t *testing.T) {
	older := "cap-older"
	newer := "cap-newer"
	rows := []api.Assignment{
		{ID: "a-old", UserID: "u1", ProjectID: "p1", CapabilityID: &older, Date: "2026-05-01", Hours: 4},
		{ID: "a-new", UserID: "u1", ProjectID: "p1", CapabilityID: &newer, Date: "2026-05-15", Hours: 2},
	}
	r, _, cleanup := newCapResolverWithFixture(t, rows)
	defer cleanup()

	got, err := r.Resolve(context.Background(), "u1", "p1")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if got != "cap-newer" {
		t.Fatalf("expected cap-newer, got %q", got)
	}
}

func TestCapabilityResolver_SkipsZeroHourRows(t *testing.T) {
	zombie := "cap-zombie"
	live := "cap-live"
	rows := []api.Assignment{
		{ID: "a-zombie", UserID: "u1", ProjectID: "p1", CapabilityID: &zombie, Date: "2026-05-15", Hours: 0},
		{ID: "a-live", UserID: "u1", ProjectID: "p1", CapabilityID: &live, Date: "2026-05-01", Hours: 4},
	}
	r, _, cleanup := newCapResolverWithFixture(t, rows)
	defer cleanup()

	got, err := r.Resolve(context.Background(), "u1", "p1")
	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}
	if got != "cap-live" {
		t.Fatalf("expected cap-live (zombie skipped), got %q", got)
	}
}

func TestCapabilityResolver_NoHistoryReturnsExplicitError(t *testing.T) {
	r, _, cleanup := newCapResolverWithFixture(t, nil)
	defer cleanup()

	_, err := r.Resolve(context.Background(), "u1", "p1")
	if err == nil {
		t.Fatalf("expected error for no history, got nil")
	}
	if !strings.Contains(err.Error(), "no prior capability found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCapabilityResolver_OnlyZombieRowsReturnsError(t *testing.T) {
	zombie := "cap-zombie"
	rows := []api.Assignment{
		{ID: "a1", UserID: "u1", ProjectID: "p1", CapabilityID: &zombie, Date: "2026-05-15", Hours: 0},
	}
	r, _, cleanup := newCapResolverWithFixture(t, rows)
	defer cleanup()

	_, err := r.Resolve(context.Background(), "u1", "p1")
	if err == nil || !strings.Contains(err.Error(), "no prior capability found") {
		t.Fatalf("expected no-history error, got: %v", err)
	}
}

func TestCapabilityResolver_CachesPerInvocation(t *testing.T) {
	cap := "cap-x"
	rows := []api.Assignment{
		{ID: "a1", UserID: "u1", ProjectID: "p1", CapabilityID: &cap, Date: "2026-05-15", Hours: 4},
	}
	r, calls, cleanup := newCapResolverWithFixture(t, rows)
	defer cleanup()

	if _, err := r.Resolve(context.Background(), "u1", "p1"); err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	if _, err := r.Resolve(context.Background(), "u1", "p1"); err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected resolver to cache; saw %d HTTP calls", *calls)
	}
}

func TestCapabilityResolver_DeterministicWhenSameDate(t *testing.T) {
	a := "cap-a"
	b := "cap-b"
	rows := []api.Assignment{
		{ID: "id-1", UserID: "u1", ProjectID: "p1", CapabilityID: &a, Date: "2026-05-15", Hours: 4},
		{ID: "id-2", UserID: "u1", ProjectID: "p1", CapabilityID: &b, Date: "2026-05-15", Hours: 4},
	}
	r, _, cleanup := newCapResolverWithFixture(t, rows)
	defer cleanup()

	got, err := r.Resolve(context.Background(), "u1", "p1")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// Secondary sort: ID desc → id-2 wins → cap-b.
	if got != "cap-b" {
		t.Fatalf("expected deterministic cap-b on same date, got %q", got)
	}
}

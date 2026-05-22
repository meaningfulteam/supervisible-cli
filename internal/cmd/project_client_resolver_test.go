package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/supervisible/supervisible-cli/internal/api"
)

func newProjectClientResolverWithFixture(t *testing.T, projects []api.Project) (*projectClientResolver, *int, func()) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": projects})
	}))
	client, err := api.NewClient(server.URL, "test-token", 5*time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return newProjectClientResolver(client), &calls, server.Close
}

func TestProjectClientResolver_ResolvesKnownProject(t *testing.T) {
	projects := []api.Project{
		{ID: "p1", Name: "Marketplace", Client: &api.ExpandedClient{ID: "c1", CompanyName: "EdVisorly"}},
		{ID: "p2", Name: "Web Redesign", Client: &api.ExpandedClient{ID: "c2", CompanyName: "Avask"}},
	}
	r, _, cleanup := newProjectClientResolverWithFixture(t, projects)
	defer cleanup()

	got := r.Resolve(context.Background(), "p2", nil)
	if got == nil {
		t.Fatalf("expected resolved client, got nil")
	}
	if got.ID != "c2" || got.Name != "Avask" {
		t.Fatalf("unexpected client: %+v", got)
	}
}

func TestProjectClientResolver_UnknownProjectReturnsNil(t *testing.T) {
	projects := []api.Project{
		{ID: "p1", Name: "Marketplace", Client: &api.ExpandedClient{ID: "c1", CompanyName: "EdVisorly"}},
	}
	r, _, cleanup := newProjectClientResolverWithFixture(t, projects)
	defer cleanup()

	got := r.Resolve(context.Background(), "p-does-not-exist", nil)
	if got != nil {
		t.Fatalf("expected nil for unknown project, got %+v", got)
	}
}

func TestProjectClientResolver_ProjectWithoutClientExpansionIsSkipped(t *testing.T) {
	projects := []api.Project{
		{ID: "p1", Name: "Marketplace", Client: nil},
	}
	r, _, cleanup := newProjectClientResolverWithFixture(t, projects)
	defer cleanup()

	got := r.Resolve(context.Background(), "p1", nil)
	if got != nil {
		t.Fatalf("expected nil when expand=client returned no client, got %+v", got)
	}
}

func TestProjectClientResolver_CachesBulkFetch(t *testing.T) {
	projects := []api.Project{
		{ID: "p1", Name: "Marketplace", Client: &api.ExpandedClient{ID: "c1", CompanyName: "EdVisorly"}},
	}
	r, calls, cleanup := newProjectClientResolverWithFixture(t, projects)
	defer cleanup()

	_ = r.Resolve(context.Background(), "p1", nil)
	_ = r.Resolve(context.Background(), "p1", nil)
	_ = r.Resolve(context.Background(), "missing", nil)

	if *calls != 1 {
		t.Fatalf("expected exactly 1 HTTP call, got %d", *calls)
	}
}

func TestProjectClientResolver_FetchFailureInvokesCallbackOnce(t *testing.T) {
	r := newProjectClientResolver(nil)
	wantErr := errors.New("boom")
	r.loadFn = func(ctx context.Context) (map[string]*ProjectClient, error) {
		return nil, wantErr
	}

	calls := 0
	var seen error
	cb := func(err error) {
		calls++
		seen = err
	}

	if got := r.Resolve(context.Background(), "p1", cb); got != nil {
		t.Fatalf("expected nil on fetch failure, got %+v", got)
	}
	if got := r.Resolve(context.Background(), "p2", cb); got != nil {
		t.Fatalf("expected nil on second call too, got %+v", got)
	}
	if calls != 1 {
		t.Fatalf("expected onError to fire exactly once, got %d", calls)
	}
	if !errors.Is(seen, wantErr) {
		t.Fatalf("expected callback to see %v, got %v", wantErr, seen)
	}
}

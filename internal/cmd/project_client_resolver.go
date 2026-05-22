package cmd

import (
	"context"
	"fmt"
	"net/url"

	"github.com/supervisible/supervisible-cli/internal/api"
)

// ProjectClient is the minimal projection of a client used to enrich
// assignment output (whois, etc.).
type ProjectClient struct {
	ID   string
	Name string
}

// projectClientResolver maps projectID → its expanded client via a single bulk
// fetch from /projects?expand=client. The bulk fetch is lazy: it fires on the
// first Resolve call. Subsequent calls hit the cache. Fail-soft: callers get
// nil when the project is unknown or the bulk fetch errored — they should
// degrade gracefully (omit the client field) rather than fail the command.
//
// Construct one per command invocation via newProjectClientResolver. Not safe
// for concurrent use across goroutines.
type projectClientResolver struct {
	client *api.Client
	cache  map[string]*ProjectClient
	loaded bool
	loadFn func(ctx context.Context) (map[string]*ProjectClient, error)
}

func newProjectClientResolver(client *api.Client) *projectClientResolver {
	r := &projectClientResolver{client: client}
	r.loadFn = r.bulkFetch
	return r
}

// Resolve returns the client linked to projectID, or nil if the project is not
// in the bulk-fetch result (or the bulk fetch failed). On the first call,
// triggers the bulk fetch; subsequent calls hit the cache.
//
// The optional onError callback fires once if the bulk fetch fails so the
// caller can emit a soft warning. It is invoked at most once per resolver.
func (r *projectClientResolver) Resolve(ctx context.Context, projectID string, onError func(error)) *ProjectClient {
	if !r.loaded {
		r.loaded = true
		cache, err := r.loadFn(ctx)
		if err != nil {
			if onError != nil {
				onError(err)
			}
			r.cache = map[string]*ProjectClient{}
			return nil
		}
		r.cache = cache
	}
	return r.cache[projectID]
}

func (r *projectClientResolver) bulkFetch(ctx context.Context) (map[string]*ProjectClient, error) {
	q := url.Values{}
	q.Set("limit", fetchLimit)
	q.Set("expand", "client")

	var projects []api.Project
	if err := r.client.Do(ctx, "GET", "/projects", q, nil, &projects); err != nil {
		return nil, fmt.Errorf("fetch projects: %w", err)
	}

	out := make(map[string]*ProjectClient, len(projects))
	for _, p := range projects {
		if p.Client == nil {
			continue
		}
		out[p.ID] = &ProjectClient{
			ID:   p.Client.ID,
			Name: p.Client.CompanyName,
		}
	}
	return out, nil
}

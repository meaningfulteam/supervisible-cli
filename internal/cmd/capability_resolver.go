package cmd

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/supervisible/supervisible-cli/internal/api"
)

// capabilityResolver caches lookups per (user, project) for the lifetime of a single
// command invocation. Construct one per command via newCapabilityResolver.
type capabilityResolver struct {
	client *api.Client
	cache  map[string]string
}

func newCapabilityResolver(client *api.Client) *capabilityResolver {
	return &capabilityResolver{client: client, cache: map[string]string{}}
}

// Resolve returns the capabilityId most recently used by userID on projectID in any
// assignment with hours > 0. Returns ("", err) if no qualifying history exists.
// Results are cached for the resolver's lifetime.
func (r *capabilityResolver) Resolve(ctx context.Context, userID, projectID string) (string, error) {
	key := userID + "|" + projectID
	if cached, ok := r.cache[key]; ok {
		if cached == "" {
			return "", fmt.Errorf("no prior capability found for user %s on project %s — pass --capability-id explicitly", userID, projectID)
		}
		return cached, nil
	}

	q := url.Values{}
	q.Set("user_id", userID)
	q.Set("project_id", projectID)
	q.Set("limit", strconv.Itoa(50))

	var assignments []api.Assignment
	if err := r.client.Do(ctx, "GET", "/assignments", q, nil, &assignments); err != nil {
		return "", fmt.Errorf("fetch assignment history: %w", err)
	}

	// Filter zombie rows; sort by date desc, then assignment ID desc for determinism.
	live := assignments[:0]
	for _, a := range assignments {
		if a.Hours > 0 && a.CapabilityID != nil && *a.CapabilityID != "" {
			live = append(live, a)
		}
	}
	if len(live) == 0 {
		r.cache[key] = ""
		return "", fmt.Errorf("no prior capability found for user %s on project %s — pass --capability-id explicitly", userID, projectID)
	}

	sort.SliceStable(live, func(i, j int) bool {
		if live[i].Date != live[j].Date {
			return live[i].Date > live[j].Date
		}
		return live[i].ID > live[j].ID
	})

	resolved := *live[0].CapabilityID
	r.cache[key] = resolved
	return resolved, nil
}

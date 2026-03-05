package schema

import (
	"context"
	"testing"
)

func TestProviderLoadsEmbeddedSchema(t *testing.T) {
	provider, err := NewProvider(context.Background())
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if !provider.SupportsQueryParam("GET", "/users", "limit") {
		t.Fatalf("expected /users to support limit")
	}

	endpoint, _, ok := provider.Describe("GET /projects")
	if !ok {
		t.Fatalf("expected describe to find GET /projects")
	}
	if endpoint.Path != "/projects" {
		t.Fatalf("unexpected path %s", endpoint.Path)
	}
}

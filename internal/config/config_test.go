package config

import (
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	initial, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if initial.BaseURL != "" || initial.Token != "" {
		t.Fatalf("unexpected initial config: %+v", initial)
	}

	cfg := Config{BaseURL: "https://example.com/api/v1", Token: "test-token"}
	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after save error = %v", err)
	}
	if loaded != cfg {
		t.Fatalf("Load() = %+v, want %+v", loaded, cfg)
	}
}

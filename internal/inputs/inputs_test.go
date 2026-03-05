package inputs

import "testing"

func TestParseJSONObject(t *testing.T) {
	obj, err := ParseJSONObject(`{"limit":10,"enabled":true}`)
	if err != nil {
		t.Fatalf("ParseJSONObject() error = %v", err)
	}
	if got := obj["limit"]; got != float64(10) {
		t.Fatalf("limit = %v", got)
	}
}

func TestMergeMapsRawWins(t *testing.T) {
	base := map[string]any{"status": "planned", "limit": 50}
	override := map[string]any{"limit": 10}
	got := MergeMaps(base, override)
	if got["limit"] != 10 {
		t.Fatalf("expected override limit=10, got %v", got["limit"])
	}
}

func TestToQueryValuesRejectsNested(t *testing.T) {
	_, err := ToQueryValues(map[string]any{"filters": map[string]any{"a": 1}})
	if err == nil {
		t.Fatalf("expected error for nested query value")
	}
}

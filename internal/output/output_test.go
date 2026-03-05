package output

import "testing"

func TestProjectFields(t *testing.T) {
	input := map[string]any{
		"id":   "1",
		"user": map[string]any{"name": "Ada", "email": "a@example.com"},
	}
	projected, err := ProjectFields(input, []string{"id", "user.name"})
	if err != nil {
		t.Fatalf("ProjectFields() error = %v", err)
	}
	m := projected.(map[string]any)
	if m["id"] != "1" {
		t.Fatalf("missing id")
	}
	user := m["user"].(map[string]any)
	if user["name"] != "Ada" {
		t.Fatalf("missing nested field")
	}
}

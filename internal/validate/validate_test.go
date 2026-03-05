package validate

import "testing"

func TestResourceIDRejectsUnsafeChars(t *testing.T) {
	cases := []string{"abc?x=1", "abc#x", "abc%20"}
	for _, c := range cases {
		if err := ResourceID("id", c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}

func TestUUID(t *testing.T) {
	if err := UUID("user-id", "550e8400-e29b-41d4-a716-446655440000"); err != nil {
		t.Fatalf("unexpected uuid error: %v", err)
	}
}

func TestDateYYYYMMDD(t *testing.T) {
	if err := DateYYYYMMDD("start-date", "2026-03-05"); err != nil {
		t.Fatalf("unexpected date error: %v", err)
	}
	if err := DateYYYYMMDD("start-date", "2026/03/05"); err == nil {
		t.Fatalf("expected invalid date error")
	}
}

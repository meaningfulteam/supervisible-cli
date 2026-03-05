package api

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "default host",
			input: "https://app.supervisible.com",
			want:  "https://app.supervisible.com/api/v1",
		},
		{
			name:  "already normalized",
			input: "https://example.com/api/v1",
			want:  "https://example.com/api/v1",
		},
		{
			name:  "legacy public path migrates",
			input: "https://example.com/api/public/v1",
			want:  "https://example.com/api/v1",
		},
		{
			name:  "host only",
			input: "example.com",
			want:  "https://example.com/api/v1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeBaseURL(tc.input)
			if err != nil {
				t.Fatalf("NormalizeBaseURL() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

func TestDebugRoundTripper_DumpsRequestAndResponse_MaskingAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "01HZ-abc")
		_, _ = w.Write([]byte(`{"data":{"id":"u1"}}`))
	}))
	defer server.Close()

	var debugBuf bytes.Buffer
	client, err := NewClientWithOptions(server.URL, "sv_live_tokenABCDEF1234567890", ClientOptions{
		Timeout:  5 * time.Second,
		Verbose:  true,
		DebugOut: &debugBuf,
	})
	if err != nil {
		t.Fatalf("NewClientWithOptions error: %v", err)
	}

	var out map[string]any
	if err := client.Do(context.Background(), http.MethodGet, "/users", nil, nil, &out); err != nil {
		t.Fatalf("Do error: %v", err)
	}

	dump := debugBuf.String()
	if !strings.Contains(dump, "GET ") {
		t.Fatalf("expected request line in dump, got: %q", dump)
	}
	if strings.Contains(dump, "sv_live_tokenABCDEF1234567890") {
		t.Fatalf("expected Authorization to be masked, got: %q", dump)
	}
	if !strings.Contains(dump, "Bearer sv_liv...7890") {
		t.Fatalf("expected masked bearer token, got: %q", dump)
	}
	if !strings.Contains(dump, "X-Request-Id") || !strings.Contains(dump, "01HZ-abc") {
		t.Fatalf("expected X-Request-Id header in dump, got: %q", dump)
	}
	if !strings.Contains(dump, `"data":{"id":"u1"}`) {
		t.Fatalf("expected response body in dump, got: %q", dump)
	}

	// Downstream JSON decode must still succeed (body preserved).
	if out["id"] != "u1" {
		t.Fatalf("expected downstream decode to receive body, got: %#v", out)
	}
}

func TestDebugRoundTripper_DisabledByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()

	var debugBuf bytes.Buffer
	client, err := NewClientWithOptions(server.URL, "sv_live_token12345abcdef", ClientOptions{
		Timeout:  5 * time.Second,
		Verbose:  false,
		DebugOut: &debugBuf,
	})
	if err != nil {
		t.Fatalf("NewClientWithOptions error: %v", err)
	}
	var out map[string]any
	if err := client.Do(context.Background(), http.MethodGet, "/users", nil, nil, &out); err != nil {
		t.Fatalf("Do error: %v", err)
	}
	if debugBuf.Len() != 0 {
		t.Fatalf("expected no debug output, got: %q", debugBuf.String())
	}
}

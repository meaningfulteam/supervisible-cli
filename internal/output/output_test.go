package output

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestPrinterAuxWritesToStderr(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := NewPrinter(&out, &errBuf, false)
	p.Aux("hello %s", "world")

	if out.Len() != 0 {
		t.Fatalf("expected stdout empty, got %q", out.String())
	}
	if got := errBuf.String(); got != "hello world\n" {
		t.Fatalf("expected stderr %q, got %q", "hello world\n", got)
	}
}

func TestPrinterDataPlainModeWritesToStdout(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := NewPrinter(&out, &errBuf, false)
	if err := p.Data("plain"); err != nil {
		t.Fatalf("Data error: %v", err)
	}
	if got := out.String(); got != "plain\n" {
		t.Fatalf("expected stdout %q, got %q", "plain\n", got)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected stderr empty, got %q", errBuf.String())
	}
}

func TestPrinterDataJSONModeDoesNotEscapeAmpersand(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := NewPrinter(&out, &errBuf, true)
	if err := p.Data(map[string]string{"label": "A & B"}); err != nil {
		t.Fatalf("Data error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"A & B"`) {
		t.Fatalf("expected literal '&' in JSON output, got %q", got)
	}
	if strings.Contains(got, "&amp;") {
		t.Fatalf("expected no &amp; in output, got %q", got)
	}
}

func TestPrinterTableWritesToStdout(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := NewPrinter(&out, &errBuf, false)
	if err := p.Table([]string{"A", "B"}, [][]string{{"1", "2"}}); err != nil {
		t.Fatalf("Table error: %v", err)
	}
	if !strings.Contains(out.String(), "A") || !strings.Contains(out.String(), "1") {
		t.Fatalf("expected table on stdout, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected stderr empty, got %q", errBuf.String())
	}
}

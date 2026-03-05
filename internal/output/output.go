package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
)

// Printer handles CLI output formatting.
type Printer struct {
	out      io.Writer
	err      io.Writer
	jsonMode bool
}

func NewPrinter(out, err io.Writer, jsonMode bool) *Printer {
	return &Printer{out: out, err: err, jsonMode: jsonMode}
}

func (p *Printer) IsJSON() bool {
	return p.jsonMode
}

func (p *Printer) Print(value any) error {
	if p.jsonMode {
		enc := json.NewEncoder(p.out)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	_, err := fmt.Fprintf(p.out, "%v\n", value)
	return err
}

func (p *Printer) PrintJSON(value any) error {
	enc := json.NewEncoder(p.out)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func (p *Printer) PrintMessage(format string, args ...any) {
	_, _ = fmt.Fprintf(p.out, format+"\n", args...)
}

func (p *Printer) PrintError(format string, args ...any) {
	_, _ = fmt.Fprintf(p.err, format+"\n", args...)
}

func (p *Printer) Table(headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(p.out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func CoalesceString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func MaskToken(token string) string {
	if len(token) <= 10 {
		return "**********"
	}
	return token[:6] + "..." + token[len(token)-4:]
}

func SplitFieldMask(raw string) []string {
	return splitAndTrim(raw)
}

func ProjectFields(value any, fields []string) (any, error) {
	if len(fields) == 0 {
		return value, nil
	}
	data, err := toAny(value)
	if err != nil {
		return nil, err
	}
	switch typed := data.(type) {
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, projectOne(item, fields))
		}
		return out, nil
	default:
		return projectOne(data, fields), nil
	}
}

func projectOne(value any, fields []string) map[string]any {
	out := map[string]any{}
	for _, field := range fields {
		parts := strings.Split(field, ".")
		if v, ok := resolvePath(value, parts); ok {
			setPath(out, parts, v)
		}
	}
	return out
}

func resolvePath(value any, parts []string) (any, bool) {
	if len(parts) == 0 {
		return value, true
	}
	cur := value
	for _, part := range parts {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, exists := obj[part]
		if !exists {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func setPath(root map[string]any, parts []string, value any) {
	if len(parts) == 0 {
		return
	}
	if len(parts) == 1 {
		root[parts[0]] = value
		return
	}
	next, ok := root[parts[0]].(map[string]any)
	if !ok {
		next = map[string]any{}
		root[parts[0]] = next
	}
	setPath(next, parts[1:], value)
}

func toAny(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		value = rv.Elem().Interface()
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func splitAndTrim(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if _, err := strconv.Unquote(`"` + strings.ReplaceAll(p, `"`, `\"`) + `"`); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out
}

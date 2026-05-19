package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/supervisible/supervisible-cli/internal/inputs"
	"github.com/supervisible/supervisible-cli/internal/validate"
)

// argsWithUsage wraps a cobra arg validator so that on validation failure the
// command's usage block (which includes Example:) is printed to stderr before
// the error returns. Use this on every leaf command that takes positional args.
func argsWithUsage(validator cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validator(cmd, args); err != nil {
			_ = cmd.Usage()
			return err
		}
		return nil
	}
}

// ptr returns a pointer to v. Use for building optional input structs:
//
//	input.Foo = ptr(foo)
func ptr[T any](v T) *T { return &v }

func splitCSV(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v != "" {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func unmarshalInput(bodyArg, filePath string, target any) error {
	var data []byte
	if strings.TrimSpace(filePath) != "" {
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		data = fileData
	} else {
		data = []byte(bodyArg)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return fmt.Errorf("empty JSON input")
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse JSON input: %w", err)
	}

	return nil
}

func requireUUIDArg(field, value string) error {
	if err := validate.ResourceID(field, value); err != nil {
		return err
	}
	return validate.UUID(field, value)
}

func validateOptionalDate(field, value string) error {
	return validate.DateYYYYMMDD(field, value)
}

func mergePayloadWithStruct(rawPayload, filePath string, typed any) (map[string]any, error) {
	base, err := inputs.ObjectToJSONMap(typed)
	if err != nil {
		return nil, err
	}
	raw, err := inputs.ParsePayload(rawPayload, filePath)
	if err != nil {
		return nil, err
	}
	return inputs.MergeMaps(base, raw), nil
}

func ensurePayloadUnsupported(rawPayload, filePath string) error {
	if strings.TrimSpace(rawPayload) != "" || strings.TrimSpace(filePath) != "" {
		return fmt.Errorf("this command does not accept --payload/--file")
	}
	return nil
}

func valueFromQuery(q url.Values, key, fallback string) string {
	v := strings.TrimSpace(q.Get(key))
	if v == "" {
		return fallback
	}
	return v
}

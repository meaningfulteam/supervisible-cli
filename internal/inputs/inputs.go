package inputs

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/supervisible/supervisible-cli/internal/validate"
)

const DefaultMaxJSONFileBytes int64 = 2 * 1024 * 1024

func ParseJSONObject(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}, nil
	}

	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("parse JSON object: %w", err)
	}

	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON value must be an object")
	}

	if err := validateObject(obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func ParseJSONObjectFromFile(path string) (map[string]any, error) {
	if err := validate.JSONFile(path, DefaultMaxJSONFileBytes); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return ParseJSONObject(string(data))
}

func ParsePayload(raw, filePath string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	filePath = strings.TrimSpace(filePath)
	if raw != "" && filePath != "" {
		return nil, fmt.Errorf("provide exactly one of payload or file")
	}
	if raw == "" && filePath == "" {
		return map[string]any{}, nil
	}
	if filePath != "" {
		return ParseJSONObjectFromFile(filePath)
	}
	return ParseJSONObject(raw)
}

func MergeMaps(base, override map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

func ToQueryValues(obj map[string]any) (url.Values, error) {
	values := url.Values{}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := validate.QueryKey(key); err != nil {
			return nil, fmt.Errorf("invalid query key %q: %w", key, err)
		}
		stringVal, err := scalarToString(obj[key])
		if err != nil {
			return nil, fmt.Errorf("invalid query value for key %q: %w", key, err)
		}
		if err := validate.QueryValue(stringVal); err != nil {
			return nil, fmt.Errorf("invalid query value for key %q: %w", key, err)
		}
		values.Set(key, stringVal)
	}
	return values, nil
}

func ObjectToJSONMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal object: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal object map: %w", err)
	}
	if out == nil {
		return map[string]any{}, nil
	}
	if err := validateObject(out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateObject(obj map[string]any) error {
	for key, value := range obj {
		if err := validate.QueryKey(key); err != nil {
			return fmt.Errorf("invalid key %q: %w", key, err)
		}
		switch typed := value.(type) {
		case map[string]any:
			if err := validateObject(typed); err != nil {
				return err
			}
		case []any:
			if err := validateArray(typed); err != nil {
				return err
			}
		case string:
			if err := validate.NoControlChars(typed); err != nil {
				return fmt.Errorf("invalid value for %q: %w", key, err)
			}
		}
	}
	return nil
}

func validateArray(values []any) error {
	for _, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			if err := validateObject(typed); err != nil {
				return err
			}
		case []any:
			if err := validateArray(typed); err != nil {
				return err
			}
		case string:
			if err := validate.NoControlChars(typed); err != nil {
				return err
			}
		}
	}
	return nil
}

func scalarToString(v any) (string, error) {
	switch typed := v.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case int:
		return strconv.Itoa(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	default:
		return "", fmt.Errorf("unsupported type %T", v)
	}
}

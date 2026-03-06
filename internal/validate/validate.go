package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	dateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func NoControlChars(value string) error {
	for _, r := range value {
		if r < 0x20 {
			return fmt.Errorf("contains control characters")
		}
	}
	return nil
}

func UUID(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if !uuidRegex.MatchString(value) {
		return fmt.Errorf("invalid %s: must be UUID", field)
	}
	return nil
}

func DateYYYYMMDD(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !dateRegex.MatchString(value) {
		return fmt.Errorf("invalid %s: expected YYYY-MM-DD", field)
	}
	return nil
}

func ResourceID(field, value string) error {
	if err := NoControlChars(value); err != nil {
		return fmt.Errorf("invalid %s: %w", field, err)
	}
	if strings.Contains(value, "?") || strings.Contains(value, "#") || strings.Contains(value, "%") {
		return fmt.Errorf("invalid %s: query/hash/encoded characters are not allowed", field)
	}
	return nil
}

func QueryKey(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("query key cannot be empty")
	}
	if err := NoControlChars(value); err != nil {
		return err
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("query key is not valid UTF-8")
	}
	return nil
}

func QueryValue(value string) error {
	if err := NoControlChars(value); err != nil {
		return err
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("query value is not valid UTF-8")
	}
	return nil
}

func JSONFile(path string, maxBytes int64) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("file path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve file path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("file does not exist: %s", abs)
		}
		return fmt.Errorf("stat file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("file must be a regular file")
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return fmt.Errorf("file too large: %d bytes exceeds %d", info.Size(), maxBytes)
	}
	return nil
}

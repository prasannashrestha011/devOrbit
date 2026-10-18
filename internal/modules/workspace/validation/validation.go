package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type Error struct {
	Field   string
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

func Name(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return "", &Error{Field: "name", Message: "must contain 1 to 120 characters"}
	}
	return name, nil
}

func Description(description string) error {
	if utf8.RuneCountInString(description) > 1000 {
		return &Error{Field: "description", Message: "must contain at most 1000 characters"}
	}
	return nil
}

func Root(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", &Error{Field: "root_path", Message: "must be an existing directory"}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", &Error{Field: "root_path", Message: "invalid directory path"}
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", &Error{Field: "root_path", Message: "must be an accessible existing directory"}
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", &Error{Field: "root_path", Message: "must be an accessible existing directory"}
	}
	return filepath.Clean(canonical), nil
}

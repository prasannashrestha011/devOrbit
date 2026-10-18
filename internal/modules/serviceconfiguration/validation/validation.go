package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type Error struct{ Field, Message string }

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

func Name(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return "", &Error{Field: "name", Message: "must contain 1 to 120 characters"}
	}
	return name, nil
}

func Command(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" || strings.ContainsRune(command, 0) {
		return "", &Error{Field: "command", Message: "must be a nonempty executable without null bytes"}
	}
	return command, nil
}

func Args(args []string) error {
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return &Error{Field: "args", Message: "must not contain null bytes"}
		}
	}
	return nil
}

func Env(env map[string]string) error {
	for key, value := range env {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, 0) {
			return &Error{Field: "env", Message: "variable names must be nonempty and cannot contain '=' or null bytes; values cannot contain null bytes"}
		}
	}
	return nil
}

func Directory(root, directory string) (string, error) {
	if strings.ContainsRune(directory, 0) {
		return "", &Error{Field: "working_directory", Message: "invalid directory path"}
	}
	absolute := filepath.IsAbs(directory)
	path := directory
	if !absolute {
		path = filepath.Join(root, directory)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", &Error{Field: "working_directory", Message: "must be an accessible existing directory"}
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", &Error{Field: "working_directory", Message: "must be an accessible existing directory"}
	}
	canonical = filepath.Clean(canonical)
	relative, err := filepath.Rel(root, canonical)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		if relative == "." {
			return "", nil
		}
		return relative, nil
	}
	if !absolute {
		return "", &Error{Field: "working_directory", Message: "relative directory must remain inside the workspace"}
	}
	return canonical, nil
}

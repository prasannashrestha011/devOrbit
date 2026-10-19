package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/prasannashrestha011/devOrbit/internal/process"
)

func TestResolveDirectoryRejectsRelativeEscape(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolveDirectory(root, ".."); !errors.Is(err, process.ErrInvalid) {
		t.Fatalf("relative escape: %v", err)
	}
	if got, err := resolveDirectory(root, "."); err != nil || got != root {
		t.Fatalf("workspace root: %q, %v", got, err)
	}
}

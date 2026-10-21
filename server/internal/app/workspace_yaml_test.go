package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

func TestEnsureWorkspaceYAML(t *testing.T) {
	root := t.TempDir()
	if err := ensureWorkspaceYAML(root); err != nil {
		t.Fatal(err)
	}
	path := workspaceconfig.Path(root)
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "services: {}\n" {
		t.Fatalf("missing default configuration: %q, %v", data, err)
	}
	custom := []byte("services:\n  api:\n    command: go run ./cmd/api\n")
	if err := os.WriteFile(path, custom, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ensureWorkspaceYAML(root); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || string(data) != string(custom) {
		t.Fatalf("existing configuration was changed: %q, %v", data, err)
	}
	if err := ensureWorkspaceYAML(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing workspace root was created")
	}
}

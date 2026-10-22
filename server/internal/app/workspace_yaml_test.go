package app

import (
	"os"
	"path/filepath"
	"strings"
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

func TestEnsureWorkspaceYAMLDiscardsServicePorts(t *testing.T) {
	root := t.TempDir()
	if err := ensureWorkspaceYAML(root); err != nil {
		t.Fatal(err)
	}
	path := workspaceconfig.Path(root)
	source := "# workspace notes\nservices:\n  api:\n    command: go run ./cmd/api # keep command\n    cwd: .\n    ports:\n      http: 8080\n  worker:\n    command: go run ./cmd/worker\n    ports: {}\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ensureWorkspaceYAML(root); err != nil {
		t.Fatal(err)
	}
	snapshot, err := workspaceconfig.Read(root)
	if err != nil {
		t.Fatalf("cleaned configuration cannot load: %v", err)
	}
	if len(snapshot.Config.Services) != 2 || snapshot.Config.Services["api"].Command != "go run ./cmd/api" || snapshot.Config.Services["api"].CWD != "." || snapshot.Config.Services["worker"].Command != "go run ./cmd/worker" {
		t.Fatalf("service definitions changed: %+v", snapshot.Config)
	}
	if strings.Contains(snapshot.Content, "ports:") || !strings.Contains(snapshot.Content, "# workspace notes") || !strings.Contains(snapshot.Content, "# keep command") {
		t.Fatalf("ports retained or comments removed: %s", snapshot.Content)
	}
	if err := ensureWorkspaceYAML(root); err != nil {
		t.Fatal(err)
	}
	repeated, err := workspaceconfig.Read(root)
	if err != nil || repeated.Revision != snapshot.Revision {
		t.Fatalf("cleanup is not repeatable: %+v %v", repeated, err)
	}
}

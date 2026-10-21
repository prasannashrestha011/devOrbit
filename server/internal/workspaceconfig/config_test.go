package workspaceconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidationAndAtomicRevision(t *testing.T) {
	root := t.TempDir()
	initial, err := Write(root, []byte("services: {}\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		"services:\n  api:\n    command: go\n    command: npm\n",
		"services:\n  api:\n    command: go\n    unknown: true\n",
		"services:\n  api:\n    command: go && rm -rf .\n",
	} {
		if _, err := Write(root, []byte(source), initial.Revision); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid YAML accepted: %q: %v", source, err)
		}
	}
	next, err := Write(root, []byte("services:\n  API server:\n    command: go run ./cmd/api\n    ports:\n      http: 8080\n"), initial.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if next.Config.Services["API server"].Ports["http"] != 8080 {
		t.Fatal("valid service was not loaded")
	}
	if _, err := Write(root, []byte("services: {}\n"), initial.Revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	current, err := Read(root)
	if err != nil || current.Revision != next.Revision {
		t.Fatalf("file changed after rejected write: %v", err)
	}
}

func TestSymlinkConfigurationRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".devorbit"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "external.yaml")
	if err := os.WriteFile(target, []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, Path(root)); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(root); !errors.Is(err, ErrInvalid) {
		t.Fatalf("symlink read: %v", err)
	}
	if _, err := Write(root, []byte("services: {}\n"), ""); err == nil {
		t.Fatal("symlink write accepted")
	}
}

func TestResolveDirectoryAllowsAbsoluteProjectAndContainsRelativePaths(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	resolved, err := ResolveDirectory(root, other)
	if err != nil || resolved != other {
		t.Fatalf("absolute project directory: %q, %v", resolved, err)
	}
	if _, err := ResolveDirectory(root, filepath.Join("..", filepath.Base(other))); err == nil {
		t.Fatal("relative directory escaped the workspace")
	}
}

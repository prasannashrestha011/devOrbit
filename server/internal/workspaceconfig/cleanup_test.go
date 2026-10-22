package workspaceconfig

import (
	"errors"
	"os"
	"testing"
)

func TestRemoveServicePortsRejectsInvalidOrChangedFiles(t *testing.T) {
	for _, source := range []string{
		"services:\n  api:\n    command: go\n    ports: {}\n    unknown: true\n",
		"services:\n  api:\n    command: go\n    ports: {}\n    ports: {}\n",
		"services:\n  api:\n    command: go\n    ports: {}\n---\nservices: {}\n",
		"services:\n  api:\n    command: go\n    ports: &ports {}\n",
	} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			if _, err := Write(root, []byte("services: {}\n"), ""); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(Path(root), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			snapshot, err := ReadRaw(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := RemoveServicePorts(root, snapshot); !errors.Is(err, ErrInvalid) {
				t.Fatalf("invalid configuration accepted: %v", err)
			}
			current, err := ReadRaw(root)
			if err != nil || current.Content != source {
				t.Fatalf("invalid file changed: %+v %v", current, err)
			}
		})
	}
	root := t.TempDir()
	current, err := Write(root, []byte("services: {}\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	stale := Snapshot{Content: "services:\n  api:\n    command: go\n    ports: {}\n", Revision: "stale"}
	if err := RemoveServicePorts(root, stale); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	after, err := Read(root)
	if err != nil || after.Revision != current.Revision {
		t.Fatalf("concurrent edit overwritten: %+v %v", after, err)
	}
}

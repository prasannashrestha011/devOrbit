package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEnvironmentFilesStayInTheirService(t *testing.T) {
	svc, wid, root := fixture(t)
	ctx := context.Background()
	for _, directory := range []string{"backend", "frontend"} {
		if err := os.Mkdir(filepath.Join(root, directory), 0700); err != nil {
			t.Fatal(err)
		}
	}
	first, _ := svc.Snapshot(ctx, wid)
	backend, err := svc.Create(ctx, wid, CreateInput{Name: "backend", Command: "go", WorkingDirectory: "backend", Revision: first.Revision})
	if err != nil {
		t.Fatal(err)
	}
	second, _ := svc.Snapshot(ctx, wid)
	frontend, err := svc.Create(ctx, wid, CreateInput{Name: "frontend", Command: "npm", WorkingDirectory: "frontend", Revision: second.Revision})
	if err != nil {
		t.Fatal(err)
	}
	backendPath := filepath.Join(root, "backend", ".env")
	content := []byte("# keep this comment\r\nPORT='8080'\r\n\r\n")
	if err := os.WriteFile(backendPath, content, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend", ".env.development"), []byte("DEBUG=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if names, err := svc.ListEnvFiles(ctx, backend.ID); err != nil || !reflect.DeepEqual(names, []string{".env", ".env.development"}) {
		t.Fatalf("backend files: %v %v", names, err)
	}
	if names, err := svc.ListEnvFiles(ctx, frontend.ID); err != nil || len(names) != 0 {
		t.Fatalf("frontend files: %v %v", names, err)
	}
	file, err := svc.ReadEnvFile(ctx, backend.ID, ".env")
	if err != nil || file.Content != string(content) {
		t.Fatalf("read: %+v %v", file, err)
	}
	updated, err := svc.SaveEnvFile(ctx, backend.ID, ".env", file.Content+"NEXT=1\r\n", file.Revision)
	if err != nil || updated.Content != string(content)+"NEXT=1\r\n" {
		t.Fatalf("save: %+v %v", updated, err)
	}
	onDisk, err := os.ReadFile(backendPath)
	if err != nil || string(onDisk) != updated.Content {
		t.Fatalf("raw content changed: %q %v", onDisk, err)
	}
	info, err := os.Stat(backendPath)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("permissions: %v %v", info, err)
	}
	if other, err := svc.ReadEnvFile(ctx, backend.ID, ".env.development"); err != nil || other.Content != "DEBUG=1\n" {
		t.Fatalf("other file changed: %+v %v", other, err)
	}
	if _, err := svc.ReadEnvFile(ctx, frontend.ID, ".env"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cross-service read: %v", err)
	}
	if _, err := svc.SaveEnvFile(ctx, backend.ID, ".env", "BAD=1\n", file.Revision); !errors.Is(err, ErrEnvConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	if err := os.WriteFile(backendPath, []byte("EXTERNAL=1\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveEnvFile(ctx, backend.ID, ".env", "BAD=1\n", updated.Revision); !errors.Is(err, ErrEnvConflict) {
		t.Fatalf("external edit conflict: %v", err)
	}
	for _, name := range []string{"../.env", ".env/../../secret", ".env\\secret", "other"} {
		if _, err := svc.ReadEnvFile(ctx, backend.ID, name); !errors.Is(err, ErrEnvFile) {
			t.Fatalf("unsafe path %q: %v", name, err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "backend", ".env"), filepath.Join(root, "backend", ".env.local")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReadEnvFile(ctx, backend.ID, ".env.local"); !errors.Is(err, ErrEnvFile) {
		t.Fatalf("symlink read: %v", err)
	}
	if names, err := svc.ListEnvFiles(ctx, backend.ID); err != nil || len(names) != 2 {
		t.Fatalf("symlink appeared in list: %v %v", names, err)
	}
}

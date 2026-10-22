package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type memoryStore struct{ items map[uuid.UUID]*model.Identity }

func (s *memoryStore) Identity(_ context.Context, id uuid.UUID) (*model.Identity, error) {
	if item := s.items[id]; item != nil {
		return item, nil
	}
	return nil, repository.ErrNotFound
}
func (s *memoryStore) Resolve(_ context.Context, wid uuid.UUID, key string) (*model.Identity, error) {
	for _, item := range s.items {
		if item.WorkspaceID == wid && item.Key == key {
			return item, nil
		}
	}
	item := &model.Identity{ID: uuid.New(), WorkspaceID: wid, Key: key}
	s.items[item.ID] = item
	return item, nil
}

type memoryWorkspaces map[uuid.UUID]*workspaceModel.Workspace

func (w memoryWorkspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
	if item := w[id]; item != nil {
		return item, nil
	}
	return nil, workspaceRepository.ErrNotFound
}
func fixture(t *testing.T) (*Service, uuid.UUID, string) {
	t.Helper()
	root := t.TempDir()
	wid := uuid.New()
	if _, err := workspaceconfig.Write(root, []byte("services: {}\n"), ""); err != nil {
		t.Fatal(err)
	}
	return New(&memoryStore{items: map[uuid.UUID]*model.Identity{}}, memoryWorkspaces{wid: {ID: wid, RootPath: root}}), wid, root
}
func TestYAMLCRUDAndStableIdentity(t *testing.T) {
	svc, wid, root := fixture(t)
	ctx := context.Background()
	if err := os.Mkdir(filepath.Join(root, "backend"), 0700); err != nil {
		t.Fatal(err)
	}
	first, err := svc.Snapshot(ctx, wid)
	if err != nil {
		t.Fatal(err)
	}
	created, err := svc.Create(ctx, wid, CreateInput{Name: "api", Command: "go", Args: []string{"run", "./cmd/api"}, WorkingDirectory: "backend", Revision: first.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if created.Command != "go" || created.WorkingDirectory != "backend" {
		t.Fatalf("created: %+v", created)
	}
	if _, err := svc.Create(ctx, wid, CreateInput{Name: "api", Command: "go", Revision: first.Revision}); !errors.Is(err, workspaceconfig.ErrConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	snapshot, err := svc.Snapshot(ctx, wid)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Config.Services["api"].CWD != "backend" {
		t.Fatal("YAML not updated")
	}
	command := "python3"
	args := []string{"-m", "http.server"}
	updated, err := svc.Update(ctx, created.ID, PatchInput{Command: &command, Args: &args, Revision: snapshot.Revision})
	if err != nil || updated.ID != created.ID || updated.Command != command {
		t.Fatalf("update: %+v %v", updated, err)
	}
	snapshot, _ = svc.Snapshot(ctx, wid)
	if err := svc.Delete(ctx, created.ID, snapshot.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, created.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("deleted lookup: %v", err)
	}
	snapshot, _ = svc.Snapshot(ctx, wid)
	recreated, err := svc.Create(ctx, wid, CreateInput{Name: "api", Command: "go", Revision: snapshot.Revision})
	if err != nil || recreated.ID != created.ID {
		t.Fatalf("identity changed: %+v %v", recreated, err)
	}
}
func TestServiceBranchUsesEachWorkingDirectory(t *testing.T) {
	svc, wid, root := fixture(t)
	ctx := context.Background()
	for _, name := range []string{"api", "worker", "plain"} {
		directory := filepath.Join(root, name)
		if err := os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
		if name != "plain" {
			for _, args := range [][]string{{"-C", directory, "init", "-q"}, {"-C", directory, "symbolic-ref", "HEAD", "refs/heads/" + name + "-branch"}} {
				if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
					t.Fatalf("git %v: %s: %v", args, output, err)
				}
			}
		}
		snapshot, err := svc.Snapshot(ctx, wid)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Create(ctx, wid, CreateInput{Name: name, Command: "go", WorkingDirectory: name, Revision: snapshot.Revision}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := svc.List(ctx, wid, 10, 0)
	if err != nil || len(items) != 3 {
		t.Fatalf("list: %+v %v", items, err)
	}
	for _, item := range items {
		want := ""
		if item.Name != "plain" {
			want = item.Name + "-branch"
		}
		if item.GitBranch != want {
			t.Fatalf("%s branch: got %q, want %q", item.Name, item.GitBranch, want)
		}
	}
	api, err := svc.Get(ctx, items[0].ID)
	if err != nil || api.GitBranch != "api-branch" {
		t.Fatalf("get branch: %+v %v", api, err)
	}
}
func TestExternalEditAndInvalidSnapshot(t *testing.T) {
	svc, wid, root := fixture(t)
	ctx := context.Background()
	snapshot, _ := svc.Snapshot(ctx, wid)
	if err := os.WriteFile(workspaceconfig.Path(root), []byte("services:\n  worker:\n    command: go test ./...\n"), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := svc.List(ctx, wid, 10, 0)
	if err != nil || len(items) != 1 || items[0].Name != "worker" {
		t.Fatalf("external edit: %+v %v", items, err)
	}
	if _, err := svc.Save(ctx, wid, "services: {}\n", snapshot.Revision); !errors.Is(err, workspaceconfig.ErrConflict) {
		t.Fatalf("stale write: %v", err)
	}
	if err := os.WriteFile(workspaceconfig.Path(root), []byte("services:\n  worker:\n    command: go\n    unknown: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.List(ctx, wid, 10, 0); !errors.Is(err, workspaceconfig.ErrInvalid) {
		t.Fatalf("invalid YAML executed: %v", err)
	}
	if _, err := svc.RawSnapshot(ctx, wid); err != nil {
		t.Fatalf("cannot repair invalid YAML: %v", err)
	}
}
func TestDeleteRunningServiceRejected(t *testing.T) {
	svc, wid, _ := fixture(t)
	ctx := context.Background()
	snapshot, _ := svc.Snapshot(ctx, wid)
	item, err := svc.Create(ctx, wid, CreateInput{Name: "api", Command: "go", Revision: snapshot.Revision})
	if err != nil {
		t.Fatal(err)
	}
	runtime := &runningRuntime{id: item.ID}
	svc.runtime = runtime
	snapshot, _ = svc.Snapshot(ctx, wid)
	if err := svc.Delete(ctx, item.ID, snapshot.Revision); !errors.Is(err, process.ErrActive) {
		t.Fatalf("delete running: %v", err)
	}
	if _, err := svc.Save(ctx, wid, "services: {}\n", snapshot.Revision); !errors.Is(err, process.ErrActive) {
		t.Fatalf("YAML save removed running service: %v", err)
	}
}

type runningRuntime struct{ id uuid.UUID }

func (r *runningRuntime) Status(id uuid.UUID) process.Status {
	if id == r.id {
		return process.Status{State: process.Running}
	}
	return process.Status{State: process.Stopped}
}

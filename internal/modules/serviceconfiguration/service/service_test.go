package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
)

type memoryStore struct {
	items   map[uuid.UUID]*model.Service
	changes map[string]any
}

func (s *memoryStore) Create(_ context.Context, item *model.Service) error {
	for _, existing := range s.items {
		if existing.WorkspaceID == item.WorkspaceID && existing.Name == item.Name {
			return repository.ErrDuplicate
		}
	}
	s.items[item.ID] = item
	return nil
}
func (s *memoryStore) Get(_ context.Context, id uuid.UUID) (*model.Service, error) {
	if item := s.items[id]; item != nil {
		return item, nil
	}
	return nil, repository.ErrNotFound
}
func (s *memoryStore) List(_ context.Context, workspaceID uuid.UUID, _, _ int) ([]*model.Service, error) {
	var items []*model.Service
	for _, item := range s.items {
		if item.WorkspaceID == workspaceID {
			items = append(items, item)
		}
	}
	return items, nil
}
func (s *memoryStore) Update(ctx context.Context, id uuid.UUID, changes map[string]any) (*model.Service, error) {
	s.changes = changes
	return s.Get(ctx, id)
}
func (s *memoryStore) Delete(_ context.Context, id uuid.UUID) error {
	if s.items[id] == nil {
		return repository.ErrNotFound
	}
	delete(s.items, id)
	return nil
}
func (s *memoryStore) NameExists(_ context.Context, workspaceID uuid.UUID, name string, except uuid.UUID) (bool, error) {
	for _, item := range s.items {
		if item.WorkspaceID == workspaceID && item.ID != except && item.Name == name {
			return true, nil
		}
	}
	return false, nil
}

type memoryWorkspaces map[uuid.UUID]*workspaceModel.Workspace

func (w memoryWorkspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
	if item := w[id]; item != nil {
		return item, nil
	}
	return nil, workspaceRepository.ErrNotFound
}

func TestCreateValidationAndDefaults(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "backend")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	wid := uuid.New()
	other := uuid.New()
	store := &memoryStore{items: map[uuid.UUID]*model.Service{}}
	svc := New(store, memoryWorkspaces{wid: {ID: wid, RootPath: root}, other: {ID: other, RootPath: root}})
	ctx := context.Background()
	base := CreateInput{Name: " Backend ", Command: " go ", WorkingDirectory: "backend"}
	item, err := svc.Create(ctx, wid, base)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == uuid.Nil || item.Name != "Backend" || item.Command != "go" || item.WorkingDirectory != "backend" || !*item.Enabled || item.AutoRestart || len(item.Args) != 0 || len(item.Env) != 0 {
		t.Fatalf("bad defaults: %+v", item)
	}
	if _, err := svc.Create(ctx, wid, base); !errors.Is(err, repository.ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := svc.Create(ctx, other, base); err != nil {
		t.Fatalf("same name in another workspace: %v", err)
	}
	if _, err := svc.Create(ctx, uuid.New(), base); !errors.Is(err, workspaceRepository.ErrNotFound) {
		t.Fatalf("missing workspace: %v", err)
	}
	falseValue, trueValue := false, true
	configured, err := svc.Create(ctx, wid, CreateInput{Name: "worker", Command: "python3", Args: []string{"", "-m"}, Env: map[string]string{"APP_ENV": "dev"}, WorkingDirectory: sub, Enabled: &falseValue, AutoRestart: &trueValue})
	if err != nil || *configured.Enabled || !configured.AutoRestart || configured.WorkingDirectory != "backend" || !reflect.DeepEqual(configured.Args, []string{"", "-m"}) {
		t.Fatalf("explicit values: %+v %v", configured, err)
	}
	for _, invalid := range []CreateInput{
		{Name: " ", Command: "go"}, {Name: strings.Repeat("x", 121), Command: "go"},
		{Name: "x", Command: " "}, {Name: "x", Command: "go\x00"},
		{Name: "x", Command: "go", Args: []string{"bad\x00"}},
		{Name: "x", Command: "go", WorkingDirectory: file},
		{Name: "x", Command: "go", WorkingDirectory: "missing"},
		{Name: "x", Command: "go", Env: map[string]string{"A=B": "x"}},
		{Name: "x", Command: "go", Env: map[string]string{"A": "x\x00"}},
	} {
		if _, err := svc.Create(ctx, wid, invalid); err == nil {
			t.Fatalf("accepted invalid input: %+v", invalid)
		}
	}
}

func TestPatchZeroValues(t *testing.T) {
	root := t.TempDir()
	wid := uuid.New()
	id := uuid.New()
	enabled := true
	store := &memoryStore{items: map[uuid.UUID]*model.Service{id: {ID: id, WorkspaceID: wid, Enabled: &enabled}}}
	svc := New(store, memoryWorkspaces{wid: {ID: wid, RootPath: root}})
	falseValue := false
	emptyArgs := []string{}
	emptyEnv := map[string]string{}
	emptyDir := ""
	_, err := svc.Update(context.Background(), id, PatchInput{Args: &emptyArgs, Env: &emptyEnv, Enabled: &falseValue, AutoRestart: &falseValue, WorkingDirectory: &emptyDir})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{"args": "[]", "env": "{}", "enabled": false, "auto_restart": false, "working_directory": ""} {
		if store.changes[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, store.changes[key], want)
		}
	}
	if _, err := svc.Update(context.Background(), id, PatchInput{}); err == nil {
		t.Fatal("empty patch accepted")
	}
}

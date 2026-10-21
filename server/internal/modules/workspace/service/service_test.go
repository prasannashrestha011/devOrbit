package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type captureStore struct {
	workspace *model.Workspace
	createErr error
	updateErr error
	deleteErr error
	others    []*model.Workspace
}

func (s *captureStore) Create(_ context.Context, workspace *model.Workspace) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.workspace = workspace
	return nil
}
func (s *captureStore) Get(_ context.Context, _ uuid.UUID) (*model.Workspace, error) {
	return s.workspace, nil
}
func (s *captureStore) List(_ context.Context, _, _ int) ([]*model.Workspace, error) {
	if s.workspace == nil {
		return s.others, nil
	}
	return append([]*model.Workspace{s.workspace}, s.others...), nil
}
func (s *captureStore) Update(_ context.Context, _ uuid.UUID, changes map[string]any) (*model.Workspace, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	if name, ok := changes["name"]; ok {
		s.workspace.Name = name.(string)
	}
	if description, ok := changes["description"]; ok {
		s.workspace.Description = description.(string)
	}
	return s.workspace, nil
}

func (s *captureStore) Delete(_ context.Context, _ uuid.UUID) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.workspace = nil
	return nil
}

func TestDeleteRemovesServicesAndRestoresConfigOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sharedRoot bool
		deleteErr  error
		wantFile   bool
	}{
		{name: "last workspace removes YAML"},
		{name: "shared root keeps YAML", sharedRoot: true, wantFile: true},
		{name: "database failure restores YAML", deleteErr: errors.New("database unavailable"), wantFile: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			store := &captureStore{deleteErr: tc.deleteErr}
			svc := New(store, dataDir)
			workspace, err := svc.Create(context.Background(), CreateInput{Name: "project"})
			if err != nil {
				t.Fatal(err)
			}
			root := workspace.RootPath
			content := []byte("services:\n  api:\n    command: go run .\n")
			current, err := workspaceconfig.Read(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := workspaceconfig.Write(root, content, current.Revision); err != nil {
				t.Fatal(err)
			}
			if tc.sharedRoot {
				store.others = []*model.Workspace{{ID: uuid.New(), RootPath: root}}
			}
			err = svc.Delete(context.Background(), workspace.ID)
			if !errors.Is(err, tc.deleteErr) {
				t.Fatalf("delete error = %v, want %v", err, tc.deleteErr)
			}
			actual, readErr := os.ReadFile(workspaceconfig.Path(root))
			if tc.wantFile && (readErr != nil || string(actual) != string(content)) {
				t.Fatalf("YAML was not preserved: %q, %v", actual, readErr)
			}
			if !tc.wantFile && !errors.Is(readErr, os.ErrNotExist) {
				t.Fatalf("YAML remained after deletion: %v", readErr)
			}
			if !tc.wantFile {
				if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("managed workspace directory remained: %v", err)
				}
			}
			if tc.deleteErr != nil && store.workspace == nil {
				t.Fatal("workspace metadata was deleted on failure")
			}
		})
	}
}

func TestDeleteKeepsLegacyProjectDirectory(t *testing.T) {
	root := t.TempDir()
	id := uuid.New()
	store := &captureStore{workspace: &model.Workspace{ID: id, RootPath: root}}
	if _, err := workspaceconfig.Write(root, []byte("services: {}\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := New(store, t.TempDir()).Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("legacy project directory was removed: %v", err)
	}
}

func TestCreateValidationAndNormalization(t *testing.T) {
	dataDir := t.TempDir()
	store := &captureStore{}
	svc := New(store, dataDir)
	for _, tc := range []struct {
		name  string
		input CreateInput
	}{
		{"empty name", CreateInput{}},
		{"whitespace name", CreateInput{Name: "   "}},
		{"long name", CreateInput{Name: strings.Repeat("x", 121)}},
		{"long description", CreateInput{Name: "ok", Description: strings.Repeat("x", 1001)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), tc.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	workspace, err := svc.Create(context.Background(), CreateInput{Name: "  good  "})
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Name != "good" || workspace.RootPath != filepath.Join(dataDir, "workspaces", workspace.ID.String()) || workspace.ID == uuid.Nil || workspace.Description != "" {
		t.Fatalf("unexpected normalized workspace: %+v", workspace)
	}
	snapshot, err := workspaceconfig.Read(workspace.RootPath)
	if err != nil {
		t.Fatalf("workspace YAML was not created: %v", err)
	}
	external := t.TempDir()
	if _, err := workspaceconfig.Update(workspace.RootPath, snapshot.Revision, func(config workspaceconfig.Config) error {
		config.Services["api"] = workspaceconfig.Service{Command: "go run .", CWD: external}
		return nil
	}); err != nil {
		t.Fatalf("external service folder was rejected: %v", err)
	}
	if _, err := svc.Update(context.Background(), workspace.ID, PatchInput{}); err == nil {
		t.Fatal("empty update accepted")
	}
	empty := ""
	if _, err := svc.Update(context.Background(), workspace.ID, PatchInput{Description: &empty}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateFailureRemovesManagedDirectory(t *testing.T) {
	dataDir := t.TempDir()
	store := &captureStore{createErr: errors.New("database unavailable")}
	if _, err := New(store, dataDir).Create(context.Background(), CreateInput{Name: "demo"}); !errors.Is(err, store.createErr) {
		t.Fatalf("create error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, "workspaces"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed workspace left files behind: %v, %v", entries, err)
	}
}

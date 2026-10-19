package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/validation"
)

type captureStore struct{ workspace *model.Workspace }

func (s *captureStore) Create(_ context.Context, workspace *model.Workspace) error {
	s.workspace = workspace
	return nil
}
func (s *captureStore) Get(_ context.Context, _ uuid.UUID) (*model.Workspace, error) {
	return s.workspace, nil
}
func (s *captureStore) List(_ context.Context, _, _ int) ([]*model.Workspace, error) {
	return nil, nil
}
func (s *captureStore) Update(_ context.Context, _ uuid.UUID, changes map[string]any) (*model.Workspace, error) {
	if name, ok := changes["name"]; ok {
		s.workspace.Name = name.(string)
	}
	if description, ok := changes["description"]; ok {
		s.workspace.Description = description.(string)
	}
	return s.workspace, nil
}
func (s *captureStore) Delete(context.Context, uuid.UUID) error { return nil }

func TestCreateValidationAndNormalization(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	store := &captureStore{}
	svc := New(store)
	for _, tc := range []struct {
		name  string
		input CreateInput
	}{
		{"empty name", CreateInput{RootPath: root}},
		{"whitespace name", CreateInput{Name: "   ", RootPath: root}},
		{"long name", CreateInput{Name: strings.Repeat("x", 121), RootPath: root}},
		{"long description", CreateInput{Name: "ok", Description: strings.Repeat("x", 1001), RootPath: root}},
		{"missing directory", CreateInput{Name: "ok", RootPath: filepath.Join(root, "missing")}},
		{"regular file", CreateInput{Name: "ok", RootPath: file}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), tc.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	workspace, err := svc.Create(context.Background(), CreateInput{Name: "  good  ", RootPath: link})
	if err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Name != "good" || workspace.RootPath != canonicalRoot || workspace.ID == uuid.Nil || workspace.Description != "" {
		t.Fatalf("unexpected normalized workspace: %+v", workspace)
	}
	if _, err := svc.Update(context.Background(), workspace.ID, PatchInput{}); err == nil {
		t.Fatal("empty update accepted")
	}
	empty := ""
	if _, err := svc.Update(context.Background(), workspace.ID, PatchInput{Description: &empty}); err != nil {
		t.Fatal(err)
	}
}

func TestRelativeRootPath(t *testing.T) {
	root := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(wd, root)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := validation.Root(relative)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.EvalSymlinks(root)
	if err != nil || actual != expected {
		t.Fatalf("got %q, want %q (%v)", actual, expected, err)
	}
}

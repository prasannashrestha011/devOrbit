package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
)

type Store interface {
	Create(context.Context, *model.Workspace) error
	Get(context.Context, uuid.UUID) (*model.Workspace, error)
	List(context.Context, int, int) ([]*model.Workspace, error)
	Update(context.Context, uuid.UUID, map[string]any) (*model.Workspace, error)
	Delete(context.Context, uuid.UUID) error
}

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

type CreateInput struct {
	Name        string
	Description string
	RootPath    string
}

type PatchInput struct {
	Name        *string
	Description *string
	RootPath    *string
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*model.Workspace, error) {
	name, err := validateName(input.Name)
	if err != nil {
		return nil, err
	}
	if err := validateDescription(input.Description); err != nil {
		return nil, err
	}
	root, err := normalizeRoot(input.RootPath)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	workspace := &model.Workspace{ID: id, Name: name, Description: input.Description, RootPath: root}
	return workspace, s.store.Create(ctx, workspace)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Workspace, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]*model.Workspace, error) {
	workspaces, err := s.store.List(ctx, limit, offset)
	if workspaces == nil && err == nil {
		workspaces = []*model.Workspace{}
	}
	return workspaces, err
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*model.Workspace, error) {
	changes := make(map[string]any)
	if input.Name != nil {
		name, err := validateName(*input.Name)
		if err != nil {
			return nil, err
		}
		changes["name"] = name
	}
	if input.Description != nil {
		if err := validateDescription(*input.Description); err != nil {
			return nil, err
		}
		changes["description"] = *input.Description
	}
	if input.RootPath != nil {
		root, err := normalizeRoot(*input.RootPath)
		if err != nil {
			return nil, err
		}
		changes["root_path"] = root
	}
	if len(changes) == 0 {
		return nil, &ValidationError{Field: "body", Message: "at least one field is required"}
	}
	return s.store.Update(ctx, id, changes)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Delete(ctx, id)
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return "", &ValidationError{Field: "name", Message: "must contain 1 to 120 characters"}
	}
	return name, nil
}

func validateDescription(description string) error {
	if utf8.RuneCountInString(description) > 1000 {
		return &ValidationError{Field: "description", Message: "must contain at most 1000 characters"}
	}
	return nil
}

func normalizeRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", &ValidationError{Field: "root_path", Message: "must be an existing directory"}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", &ValidationError{Field: "root_path", Message: "invalid directory path"}
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", &ValidationError{Field: "root_path", Message: "must be an accessible existing directory"}
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", &ValidationError{Field: "root_path", Message: "must be an accessible existing directory"}
	}
	return filepath.Clean(canonical), nil
}

func IsNotFound(err error) bool { return errors.Is(err, repository.ErrNotFound) }

package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/dto"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/validation"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type Store interface {
	Create(context.Context, *model.Workspace) error
	Get(context.Context, uuid.UUID) (*model.Workspace, error)
	List(context.Context, int, int) ([]*model.Workspace, error)
	Update(context.Context, uuid.UUID, map[string]any) (*model.Workspace, error)
	Delete(context.Context, uuid.UUID) error
}

type Service struct {
	store   Store
	dataDir string
}

func New(store Store, dataDir string) *Service {
	return &Service{store: store, dataDir: dataDir}
}

type ValidationError = validation.Error
type CreateInput = dto.CreateInput
type PatchInput = dto.PatchInput

func (s *Service) Create(ctx context.Context, input CreateInput) (*model.Workspace, error) {
	name, err := validation.Name(input.Name)
	if err != nil {
		return nil, err
	}
	if err := validation.Description(input.Description); err != nil {
		return nil, err
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(s.dataDir) {
		return nil, errors.New("workspace data directory is unavailable")
	}
	root := filepath.Join(s.dataDir, "workspaces", id.String())
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return nil, err
	}
	if err := os.Mkdir(root, 0700); err != nil {
		return nil, err
	}
	workspace := &model.Workspace{ID: id, Name: name, Description: input.Description, RootPath: root}
	if _, err := workspaceconfig.Write(root, []byte("services: {}\n"), ""); err != nil {
		_ = os.Remove(filepath.Join(root, ".devorbit"))
		_ = os.Remove(root)
		return nil, err
	}
	if err := s.store.Create(ctx, workspace); err != nil {
		_ = os.Remove(workspaceconfig.Path(root))
		_ = os.Remove(filepath.Join(root, ".devorbit"))
		_ = os.Remove(root)
		return nil, err
	}
	return workspace, nil
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
		name, err := validation.Name(*input.Name)
		if err != nil {
			return nil, err
		}
		changes["name"] = name
	}
	if input.Description != nil {
		if err := validation.Description(*input.Description); err != nil {
			return nil, err
		}
		changes["description"] = *input.Description
	}
	if len(changes) == 0 {
		return nil, &ValidationError{Field: "body", Message: "at least one field is required"}
	}
	return s.store.Update(ctx, id, changes)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	workspace, err := s.store.Get(ctx, id)
	if err != nil {
		return err
	}
	for offset := 0; ; offset += 100 {
		items, err := s.store.List(ctx, 100, offset)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.ID != id && item.RootPath == workspace.RootPath {
				return s.store.Delete(ctx, id)
			}
		}
		if len(items) < 100 {
			break
		}
	}
	snapshot, err := workspaceconfig.Read(workspace.RootPath)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return err
	}
	if !missing {
		if err := workspaceconfig.Remove(workspace.RootPath, snapshot.Revision); err != nil {
			return err
		}
	}
	if err := s.store.Delete(ctx, id); err != nil {
		if !missing {
			_, restoreErr := workspaceconfig.Write(workspace.RootPath, []byte(snapshot.Content), "")
			return errors.Join(err, restoreErr)
		}
		return err
	}
	if workspace.RootPath == filepath.Join(s.dataDir, "workspaces", id.String()) {
		_ = os.Remove(filepath.Join(workspace.RootPath, ".devorbit"))
		_ = os.Remove(workspace.RootPath)
	}
	return nil
}

func IsNotFound(err error) bool { return errors.Is(err, repository.ErrNotFound) }

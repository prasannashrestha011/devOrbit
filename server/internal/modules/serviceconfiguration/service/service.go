package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/dto"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/validation"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
)

type Store interface {
	Create(context.Context, *model.Service) error
	Get(context.Context, uuid.UUID) (*model.Service, error)
	List(context.Context, uuid.UUID, int, int) ([]*model.Service, error)
	Update(context.Context, uuid.UUID, map[string]any) (*model.Service, error)
	Delete(context.Context, uuid.UUID) error
	NameExists(context.Context, uuid.UUID, string, uuid.UUID) (bool, error)
}

type WorkspaceStore interface {
	Get(context.Context, uuid.UUID) (*workspaceModel.Workspace, error)
}

type Service struct {
	store      Store
	workspaces WorkspaceStore
}

func New(store Store, workspaces WorkspaceStore) *Service {
	return &Service{store: store, workspaces: workspaces}
}

type ValidationError = validation.Error
type CreateInput = dto.CreateInput
type PatchInput = dto.PatchInput

func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, input CreateInput) (*model.Service, error) {
	workspace, err := s.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	name, err := validation.Name(input.Name)
	if err != nil {
		return nil, err
	}
	command, err := validation.Command(input.Command)
	if err != nil {
		return nil, err
	}
	workingDir, err := validation.Directory(workspace.RootPath, input.WorkingDirectory)
	if err != nil {
		return nil, err
	}
	if err := validation.Args(input.Args); err != nil {
		return nil, err
	}
	if err := validation.Env(input.Env); err != nil {
		return nil, err
	}
	exists, err := s.store.NameExists(ctx, workspaceID, name, uuid.Nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, repository.ErrDuplicate
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	autoRestart := false
	if input.AutoRestart != nil {
		autoRestart = *input.AutoRestart
	}
	args := input.Args
	if args == nil {
		args = []string{}
	}
	env := input.Env
	if env == nil {
		env = map[string]string{}
	}
	item := &model.Service{ID: id, WorkspaceID: workspaceID, Name: name, Command: command, Args: args, WorkingDirectory: workingDir, Env: env, Enabled: &enabled, AutoRestart: autoRestart}
	return item, s.store.Create(ctx, item)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Service, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*model.Service, error) {
	if _, err := s.workspaces.Get(ctx, workspaceID); err != nil {
		return nil, err
	}
	items, err := s.store.List(ctx, workspaceID, limit, offset)
	if items == nil && err == nil {
		items = []*model.Service{}
	}
	return items, err
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*model.Service, error) {
	if input.Name == nil && input.Command == nil && input.Args == nil && input.WorkingDirectory == nil && input.Env == nil && input.Enabled == nil && input.AutoRestart == nil {
		return nil, &ValidationError{Field: "body", Message: "at least one field is required"}
	}
	item, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	changes := make(map[string]any)
	if input.Name != nil {
		name, err := validation.Name(*input.Name)
		if err != nil {
			return nil, err
		}
		exists, err := s.store.NameExists(ctx, item.WorkspaceID, name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, repository.ErrDuplicate
		}
		changes["name"] = name
	}
	if input.Command != nil {
		command, err := validation.Command(*input.Command)
		if err != nil {
			return nil, err
		}
		changes["command"] = command
	}
	if input.WorkingDirectory != nil {
		workspace, err := s.workspaces.Get(ctx, item.WorkspaceID)
		if err != nil {
			return nil, err
		}
		dir, err := validation.Directory(workspace.RootPath, *input.WorkingDirectory)
		if err != nil {
			return nil, err
		}
		changes["working_directory"] = dir
	}
	if input.Args != nil {
		if err := validation.Args(*input.Args); err != nil {
			return nil, err
		}
		args := *input.Args
		if args == nil {
			args = []string{}
		}
		encoded, _ := json.Marshal(args)
		changes["args"] = string(encoded)
	}
	if input.Env != nil {
		if err := validation.Env(*input.Env); err != nil {
			return nil, err
		}
		env := *input.Env
		if env == nil {
			env = map[string]string{}
		}
		encoded, _ := json.Marshal(env)
		changes["env"] = string(encoded)
	}
	if input.Enabled != nil {
		changes["enabled"] = *input.Enabled
	}
	if input.AutoRestart != nil {
		changes["auto_restart"] = *input.AutoRestart
	}
	if len(changes) == 0 {
		return nil, &ValidationError{Field: "body", Message: "at least one field is required"}
	}
	return s.store.Update(ctx, id, changes)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error { return s.store.Delete(ctx, id) }

func IsNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound) || errors.Is(err, workspaceRepository.ErrNotFound) || errors.Is(err, repository.ErrWorkspaceNotFound)
}
func IsDuplicate(err error) bool { return errors.Is(err, repository.ErrDuplicate) }

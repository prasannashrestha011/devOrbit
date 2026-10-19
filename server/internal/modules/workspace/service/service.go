package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/dto"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/validation"
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
	root, err := validation.Root(input.RootPath)
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
	if input.RootPath != nil {
		root, err := validation.Root(*input.RootPath)
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

func IsNotFound(err error) bool { return errors.Is(err, repository.ErrNotFound) }

package service

import (
	"context"
	"errors"
	"runtime"

	"github.com/google/uuid"
	serviceconfig "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

var ErrDisabled = errors.New("service is disabled")

type Service struct {
	configs    *serviceconfig.Service
	workspaces *workspace.Service
	supervisor *process.Supervisor
}

func New(configs *serviceconfig.Service, workspaces *workspace.Service, supervisor *process.Supervisor) *Service {
	return &Service{configs: configs, workspaces: workspaces, supervisor: supervisor}
}

func (s *Service) Start(ctx context.Context, id uuid.UUID) (process.Status, error) {
	cfg, err := s.configuration(ctx, id)
	if err != nil {
		return process.Status{}, err
	}
	return s.supervisor.Start(id, cfg)
}

func (s *Service) Stop(ctx context.Context, id uuid.UUID) (process.Status, error) {
	if _, err := s.configs.Get(ctx, id); err != nil {
		return process.Status{}, err
	}
	return s.supervisor.Stop(ctx, id)
}

func (s *Service) Restart(ctx context.Context, id uuid.UUID) (process.Status, error) {
	if _, err := s.configuration(ctx, id); err != nil {
		return process.Status{}, err
	}
	return s.supervisor.Restart(ctx, id, func(ctx context.Context) (process.Config, error) {
		return s.configuration(ctx, id)
	})
}

func (s *Service) configuration(ctx context.Context, id uuid.UUID) (process.Config, error) {
	config, err := s.configs.Get(ctx, id)
	if err != nil {
		return process.Config{}, err
	}
	if config.Enabled != nil && !*config.Enabled {
		return process.Config{}, ErrDisabled
	}
	parent, err := s.workspaces.Get(ctx, config.WorkspaceID)
	if err != nil {
		return process.Config{}, err
	}
	dir, err := resolveDirectory(parent.RootPath, config.WorkingDirectory)
	if err != nil {
		return process.Config{}, err
	}
	return process.Config{WorkspaceID: config.WorkspaceID, Name: config.Name, Command: config.Command, Args: config.Args, Dir: dir, Env: config.Env, Interactive: true, Tmux: runtime.GOOS != "windows"}, nil
}

func (s *Service) Status(ctx context.Context, id uuid.UUID) (process.Status, error) {
	if _, err := s.configs.Get(ctx, id); err != nil {
		return process.Status{}, err
	}
	return s.supervisor.Status(id), nil
}

func (s *Service) Terminal(ctx context.Context, id, executionID uuid.UUID) (*process.Terminal, uuid.UUID, error) {
	config, err := s.configs.Get(ctx, id)
	if err != nil {
		return nil, uuid.Nil, err
	}
	terminal, err := s.supervisor.Terminal(id, config.WorkspaceID, executionID)
	return terminal, config.WorkspaceID, err
}

func (s *Service) TerminalInput(id, workspaceID, executionID uuid.UUID, data []byte) error {
	return s.supervisor.TerminalInput(id, workspaceID, executionID, data)
}

func (s *Service) TerminalResize(id, workspaceID, executionID uuid.UUID, cols, rows uint16) error {
	return s.supervisor.TerminalResize(id, workspaceID, executionID, cols, rows)
}

func IsNotFound(err error) bool { return serviceconfig.IsNotFound(err) || workspace.IsNotFound(err) }

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
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

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }

type CreateInput struct {
	Name             string
	Command          string
	Args             []string
	WorkingDirectory string
	Env              map[string]string
	Enabled          *bool
	AutoRestart      *bool
}

type PatchInput struct {
	Name             *string
	Command          *string
	Args             *[]string
	WorkingDirectory *string
	Env              *map[string]string
	Enabled          *bool
	AutoRestart      *bool
}

func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, input CreateInput) (*model.Service, error) {
	workspace, err := s.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	name, err := validateName(input.Name)
	if err != nil {
		return nil, err
	}
	command, err := validateCommand(input.Command)
	if err != nil {
		return nil, err
	}
	workingDir, err := normalizeDirectory(workspace.RootPath, input.WorkingDirectory)
	if err != nil {
		return nil, err
	}
	if err := validateArgs(input.Args); err != nil {
		return nil, err
	}
	if err := validateEnv(input.Env); err != nil {
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
		name, err := validateName(*input.Name)
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
		command, err := validateCommand(*input.Command)
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
		dir, err := normalizeDirectory(workspace.RootPath, *input.WorkingDirectory)
		if err != nil {
			return nil, err
		}
		changes["working_directory"] = dir
	}
	if input.Args != nil {
		if err := validateArgs(*input.Args); err != nil {
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
		if err := validateEnv(*input.Env); err != nil {
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

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return "", &ValidationError{Field: "name", Message: "must contain 1 to 120 characters"}
	}
	return name, nil
}

func validateCommand(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" || strings.ContainsRune(command, 0) {
		return "", &ValidationError{Field: "command", Message: "must be a nonempty executable without null bytes"}
	}
	return command, nil
}

func validateArgs(args []string) error {
	for _, arg := range args {
		if strings.ContainsRune(arg, 0) {
			return &ValidationError{Field: "args", Message: "must not contain null bytes"}
		}
	}
	return nil
}

func validateEnv(env map[string]string) error {
	for key, value := range env {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, 0) {
			return &ValidationError{Field: "env", Message: "variable names must be nonempty and cannot contain '=' or null bytes; values cannot contain null bytes"}
		}
	}
	return nil
}

func normalizeDirectory(root, directory string) (string, error) {
	if strings.ContainsRune(directory, 0) {
		return "", &ValidationError{Field: "working_directory", Message: "invalid directory path"}
	}
	absolute := filepath.IsAbs(directory)
	path := directory
	if !absolute {
		path = filepath.Join(root, directory)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", &ValidationError{Field: "working_directory", Message: "must be an accessible existing directory"}
	}
	info, err := os.Stat(canonical)
	if err != nil || !info.IsDir() {
		return "", &ValidationError{Field: "working_directory", Message: "must be an accessible existing directory"}
	}
	canonical = filepath.Clean(canonical)
	relative, err := filepath.Rel(root, canonical)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		if relative == "." {
			return "", nil
		}
		return relative, nil
	}
	if !absolute {
		return "", &ValidationError{Field: "working_directory", Message: "relative directory must remain inside the workspace"}
	}
	return canonical, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound) || errors.Is(err, workspaceRepository.ErrNotFound) || errors.Is(err, repository.ErrWorkspaceNotFound)
}
func IsDuplicate(err error) bool { return errors.Is(err, repository.ErrDuplicate) }

package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/dto"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type Store interface {
	Identity(context.Context, uuid.UUID) (*model.Identity, error)
	Resolve(context.Context, uuid.UUID, string) (*model.Identity, error)
}

type WorkspaceStore interface {
	Get(context.Context, uuid.UUID) (*workspaceModel.Workspace, error)
}

type Runtime interface {
	Status(uuid.UUID) process.Status
}

type Service struct {
	store      Store
	workspaces WorkspaceStore
	runtime    Runtime
}

func New(store Store, workspaces WorkspaceStore, runtime ...Runtime) *Service {
	s := &Service{store: store, workspaces: workspaces}
	if len(runtime) != 0 {
		s.runtime = runtime[0]
	}
	return s
}

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

type CreateInput = dto.CreateInput
type PatchInput = dto.PatchInput

func (s *Service) Snapshot(ctx context.Context, workspaceID uuid.UUID) (workspaceconfig.Snapshot, error) {
	workspace, err := s.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return workspaceconfig.Snapshot{}, err
	}
	return workspaceconfig.Read(workspace.RootPath)
}

func (s *Service) RawSnapshot(ctx context.Context, workspaceID uuid.UUID) (workspaceconfig.Snapshot, error) {
	workspace, err := s.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return workspaceconfig.Snapshot{}, err
	}
	return workspaceconfig.ReadRaw(workspace.RootPath)
}

func (s *Service) Save(ctx context.Context, workspaceID uuid.UUID, content, revision string) (workspaceconfig.Snapshot, error) {
	workspace, err := s.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return workspaceconfig.Snapshot{}, err
	}
	if revision == "" {
		return workspaceconfig.Snapshot{}, workspaceconfig.ErrConflict
	}
	next, err := workspaceconfig.Parse(workspace.RootPath, []byte(content))
	if err != nil {
		return workspaceconfig.Snapshot{}, err
	}
	if s.runtime != nil {
		current, err := workspaceconfig.Read(workspace.RootPath)
		if err != nil {
			return workspaceconfig.Snapshot{}, err
		}
		if current.Revision != revision {
			return workspaceconfig.Snapshot{}, workspaceconfig.ErrConflict
		}
		for key := range current.Config.Services {
			if _, remains := next.Services[key]; remains {
				continue
			}
			identity, err := s.store.Resolve(ctx, workspaceID, key)
			if err != nil {
				return workspaceconfig.Snapshot{}, err
			}
			state := s.runtime.Status(identity.ID).State
			if state == process.Running || state == process.Starting || state == process.Stopping {
				return workspaceconfig.Snapshot{}, process.ErrActive
			}
		}
	}
	return workspaceconfig.Write(workspace.RootPath, []byte(content), revision)
}

func (s *Service) Validate(ctx context.Context, workspaceID uuid.UUID, content string) (workspaceconfig.Config, error) {
	workspace, err := s.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return workspaceconfig.Config{}, err
	}
	return workspaceconfig.Parse(workspace.RootPath, []byte(content))
}

func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, input CreateInput) (*model.Service, error) {
	workspace, err := s.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if input.Revision == "" {
		return nil, workspaceconfig.ErrConflict
	}
	name := strings.TrimSpace(input.Name)
	definition := workspaceconfig.Service{Command: commandLine(input.Command, input.Args), CWD: input.WorkingDirectory}
	var identity *model.Identity
	_, err = workspaceconfig.Update(workspace.RootPath, input.Revision, func(config workspaceconfig.Config) error {
		if _, exists := config.Services[name]; exists {
			return repository.ErrDuplicate
		}
		config.Services[name] = definition
		encoded, err := workspaceconfig.Encode(config)
		if err != nil {
			return err
		}
		if _, err := workspaceconfig.Parse(workspace.RootPath, encoded); err != nil {
			return err
		}
		identity, err = s.store.Resolve(ctx, workspaceID, name)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, identity.ID)
}

func (s *Service) Identity(ctx context.Context, id uuid.UUID) (*model.Identity, error) {
	return s.store.Identity(ctx, id)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Service, error) {
	identity, err := s.store.Identity(ctx, id)
	if err != nil {
		return nil, err
	}
	workspace, err := s.workspaces.Get(ctx, identity.WorkspaceID)
	if err != nil {
		return nil, err
	}
	snapshot, err := workspaceconfig.Read(workspace.RootPath)
	if err != nil {
		return nil, err
	}
	definition, exists := snapshot.Config.Services[identity.Key]
	if !exists {
		return nil, repository.ErrNotFound
	}
	return project(ctx, identity, definition, workspace.RootPath), nil
}

func (s *Service) List(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*model.Service, error) {
	workspace, err := s.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	snapshot, err := workspaceconfig.Read(workspace.RootPath)
	if err != nil {
		return nil, err
	}
	keys := workspaceconfig.Keys(snapshot.Config)
	if offset >= len(keys) {
		return []*model.Service{}, nil
	}
	end := offset + limit
	if end > len(keys) {
		end = len(keys)
	}
	items := make([]*model.Service, 0, end-offset)
	for _, key := range keys[offset:end] {
		identity, err := s.store.Resolve(ctx, workspaceID, key)
		if err != nil {
			return nil, err
		}
		items = append(items, project(ctx, identity, snapshot.Config.Services[key], workspace.RootPath))
	}
	return items, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input PatchInput) (*model.Service, error) {
	identity, err := s.store.Identity(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil && *input.Name != identity.Key {
		return nil, &ValidationError{Field: "name", Message: "rename the YAML key in the file to create a new service identity"}
	}
	if input.Command == nil && input.Args == nil && input.WorkingDirectory == nil {
		return nil, &ValidationError{Field: "body", Message: "at least one field is required"}
	}
	workspace, err := s.workspaces.Get(ctx, identity.WorkspaceID)
	if err != nil {
		return nil, err
	}
	_, err = workspaceconfig.Update(workspace.RootPath, input.Revision, func(config workspaceconfig.Config) error {
		definition, exists := config.Services[identity.Key]
		if !exists {
			return repository.ErrNotFound
		}
		if input.Command != nil || input.Args != nil {
			words, err := workspaceconfig.SplitCommand(definition.Command)
			if err != nil {
				return err
			}
			if input.Command != nil {
				words[0] = *input.Command
			}
			if input.Args != nil {
				words = append(words[:1], (*input.Args)...)
			}
			definition.Command = workspaceconfig.JoinCommand(words)
		}
		if input.WorkingDirectory != nil {
			definition.CWD = *input.WorkingDirectory
		}
		config.Services[identity.Key] = definition
		encoded, err := workspaceconfig.Encode(config)
		if err != nil {
			return err
		}
		_, err = workspaceconfig.Parse(workspace.RootPath, encoded)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID, revision string) error {
	identity, err := s.store.Identity(ctx, id)
	if err != nil {
		return err
	}
	if s.runtime != nil {
		state := s.runtime.Status(id).State
		if state == process.Running || state == process.Starting || state == process.Stopping {
			return process.ErrActive
		}
	}
	workspace, err := s.workspaces.Get(ctx, identity.WorkspaceID)
	if err != nil {
		return err
	}
	_, err = workspaceconfig.Update(workspace.RootPath, revision, func(config workspaceconfig.Config) error {
		if _, exists := config.Services[identity.Key]; !exists {
			return repository.ErrNotFound
		}
		delete(config.Services, identity.Key)
		return nil
	})
	return err
}

func project(ctx context.Context, identity *model.Identity, definition workspaceconfig.Service, root string) *model.Service {
	words, _ := workspaceconfig.SplitCommand(definition.Command)
	enabled := true
	item := &model.Service{ID: identity.ID, WorkspaceID: identity.WorkspaceID, Name: identity.Key, Command: words[0], Args: words[1:], WorkingDirectory: definition.CWD, Env: map[string]string{}, Enabled: &enabled}
	if directory, err := workspaceconfig.ResolveDirectory(root, definition.CWD); err == nil {
		if output, err := exec.CommandContext(ctx, "git", "-C", directory, "symbolic-ref", "--quiet", "--short", "HEAD").Output(); err == nil {
			item.GitBranch = strings.TrimSpace(string(output))
		}
	}
	return item
}

func commandLine(command string, args []string) string {
	if len(args) == 0 {
		return command
	}
	return workspaceconfig.JoinCommand(append([]string{command}, args...))
}

func IsNotFound(err error) bool {
	return errors.Is(err, repository.ErrNotFound) || errors.Is(err, workspaceRepository.ErrNotFound) || errors.Is(err, os.ErrNotExist)
}
func IsDuplicate(err error) bool { return errors.Is(err, repository.ErrDuplicate) }
func IsConflict(err error) bool {
	return errors.Is(err, workspaceconfig.ErrConflict) || errors.Is(err, process.ErrActive)
}
func IsInvalid(err error) bool { return errors.Is(err, workspaceconfig.ErrInvalid) }

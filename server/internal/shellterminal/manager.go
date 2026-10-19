package shellterminal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

var ErrNotFound = errors.New("terminal session not found")
var ErrLimit = errors.New("workspace terminal limit reached")
var ErrHasSessions = errors.New("workspace has terminal sessions")

const maxPerWorkspace = 16

type WorkspaceStore interface {
	Get(context.Context, uuid.UUID) (*model.Workspace, error)
	Delete(context.Context, uuid.UUID) error
}

type Session struct {
	ID          uuid.UUID     `json:"id"`
	WorkspaceID uuid.UUID     `json:"workspace_id"`
	Shell       string        `json:"shell"`
	State       process.State `json:"state"`
	StartedAt   *time.Time    `json:"started_at"`
	ExitCode    *int          `json:"exit_code"`
}

type Manager struct {
	mu         sync.Mutex
	workspaces WorkspaceStore
	supervisor *process.Supervisor
	sessions   map[uuid.UUID]Session
	closed     bool
}

func New(workspaces WorkspaceStore, grace time.Duration) *Manager {
	return &Manager{workspaces: workspaces, supervisor: process.New(grace), sessions: make(map[uuid.UUID]Session)}
}

func shell() string {
	if runtime.GOOS == "windows" {
		if value := os.Getenv("COMSPEC"); value != "" {
			return value
		}
		return "cmd.exe"
	}
	if value := os.Getenv("SHELL"); value != "" {
		return value
	}
	return "/bin/sh"
}

func (m *Manager) Create(ctx context.Context, workspaceID uuid.UUID) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Session{}, process.ErrClosed
	}
	workspace, err := m.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return Session{}, err
	}
	count := 0
	for _, session := range m.sessions {
		if session.WorkspaceID == workspaceID {
			count++
		}
	}
	if count >= maxPerWorkspace {
		return Session{}, ErrLimit
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return Session{}, err
	}
	command := shell()
	var env map[string]string
	if runtime.GOOS != "windows" {
		env = map[string]string{"TERM": "xterm-256color"}
	}
	status, err := m.supervisor.Start(id, process.Config{WorkspaceID: workspaceID, Command: command, Dir: workspace.RootPath, Env: env, Interactive: true})
	if err != nil {
		_ = m.supervisor.Forget(id)
		return Session{}, err
	}
	session := Session{ID: id, WorkspaceID: workspaceID, Shell: filepath.Base(command), State: status.State, StartedAt: status.StartedAt, ExitCode: status.ExitCode}
	m.sessions[id] = session
	return session, nil
}

func (m *Manager) session(workspaceID, id uuid.UUID) (Session, error) {
	session, ok := m.sessions[id]
	if !ok || session.WorkspaceID != workspaceID {
		return Session{}, ErrNotFound
	}
	status := m.supervisor.Status(id)
	session.State, session.StartedAt, session.ExitCode = status.State, status.StartedAt, status.ExitCode
	return session, nil
}

func (m *Manager) List(ctx context.Context, workspaceID uuid.UUID) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.workspaces.Get(ctx, workspaceID); err != nil {
		return nil, err
	}
	sessions := make([]Session, 0)
	for id, session := range m.sessions {
		if session.WorkspaceID == workspaceID {
			current, _ := m.session(workspaceID, id)
			sessions = append(sessions, current)
		}
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].StartedAt.Before(*sessions[j].StartedAt) })
	return sessions, nil
}

func (m *Manager) Terminal(workspaceID, id uuid.UUID) (*process.Terminal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.session(workspaceID, id); err != nil {
		return nil, err
	}
	status := m.supervisor.Status(id)
	return m.supervisor.Terminal(id, workspaceID, status.ExecutionID)
}

func (m *Manager) Input(workspaceID, id uuid.UUID, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.session(workspaceID, id); err != nil {
		return err
	}
	return m.supervisor.TerminalInput(id, workspaceID, m.supervisor.Status(id).ExecutionID, data)
}

func (m *Manager) Resize(workspaceID, id uuid.UUID, cols, rows uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.session(workspaceID, id); err != nil {
		return err
	}
	return m.supervisor.TerminalResize(id, workspaceID, m.supervisor.Status(id).ExecutionID, cols, rows)
}

func (m *Manager) Delete(ctx context.Context, workspaceID, id uuid.UUID) error {
	m.mu.Lock()
	if _, err := m.session(workspaceID, id); err != nil {
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	if _, err := m.supervisor.Stop(ctx, id); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.supervisor.Forget(id); err != nil {
		return err
	}
	delete(m.sessions, id)
	return nil
}

func (m *Manager) DeleteWorkspace(ctx context.Context, workspaceID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, session := range m.sessions {
		if session.WorkspaceID == workspaceID {
			return ErrHasSessions
		}
	}
	return m.workspaces.Delete(ctx, workspaceID)
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	err := m.supervisor.Shutdown(ctx)
	m.mu.Lock()
	clear(m.sessions)
	m.mu.Unlock()
	return err
}

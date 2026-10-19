//go:build !windows

package shellterminal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

type workspaceStore struct{ workspace *model.Workspace }

func (s *workspaceStore) Get(_ context.Context, id uuid.UUID) (*model.Workspace, error) {
	if s.workspace == nil || s.workspace.ID != id {
		return nil, errors.New("workspace not found")
	}
	return s.workspace, nil
}

func (s *workspaceStore) Delete(_ context.Context, id uuid.UUID) error {
	if s.workspace == nil || s.workspace.ID != id {
		return errors.New("workspace not found")
	}
	s.workspace = nil
	return nil
}

func TestShellSessionsAreIndependentAndRetainOutputUntilClosed(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	root := t.TempDir()
	store := &workspaceStore{workspace: &model.Workspace{ID: uuid.New(), RootPath: root}}
	m := New(store, 50*time.Millisecond)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	first, err := m.Create(context.Background(), store.workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Create(context.Background(), store.workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("sessions share an ID")
	}
	if err := m.DeleteWorkspace(context.Background(), store.workspace.ID); !errors.Is(err, ErrHasSessions) {
		t.Fatalf("delete with shells: %v", err)
	}
	if err := m.Input(uuid.New(), first.ID, []byte("ignored\n")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace input: %v", err)
	}
	terminal, err := m.Terminal(store.workspace.ID, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Resize(store.workspace.ID, first.ID, 90, 30); err != nil {
		t.Fatal(err)
	}
	if err := m.Input(store.workspace.ID, first.ID, []byte("pwd\nexit\n")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sessions, err := m.List(context.Background(), store.workspace.ID)
		if err != nil {
			t.Fatal(err)
		}
		if sessions[0].State == "exited" || sessions[1].State == "exited" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	frames, _, live, cancel := terminal.Subscribe(0)
	defer cancel()
	var output strings.Builder
	for _, frame := range frames {
		output.Write(frame.Data)
	}
	if !strings.Contains(output.String(), root) {
		t.Fatalf("shell did not run in workspace root: %q", output.String())
	}
	select {
	case <-live:
	default:
		t.Fatal("exited terminal did not close its stream")
	}
	if err := m.Delete(context.Background(), store.workspace.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete(context.Background(), store.workspace.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Terminal(store.workspace.ID, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("closed session available: %v", err)
	}
	if err := m.DeleteWorkspace(context.Background(), store.workspace.ID); err != nil {
		t.Fatal(err)
	}
}

func TestShellInputEchoesBeforeEnter(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	store := &workspaceStore{workspace: &model.Workspace{ID: uuid.New(), RootPath: t.TempDir()}}
	m := New(store, 50*time.Millisecond)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	session, err := m.Create(context.Background(), store.workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := m.Terminal(store.workspace.ID, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	frames, _, live, cancel := terminal.Subscribe(0)
	defer cancel()
	if err := m.Input(store.workspace.ID, session.ID, []byte("echo visible_input")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for _, frame := range frames {
		output.Write(frame.Data)
	}
	deadline := time.After(3 * time.Second)
	for !strings.Contains(output.String(), "echo visible_input") {
		select {
		case frame, ok := <-live:
			if !ok {
				t.Fatalf("shell closed before echoing input: %q", output.String())
			}
			output.Write(frame.Data)
		case <-deadline:
			t.Fatalf("shell input was invisible before Enter: %q", output.String())
		}
	}
}

func TestShutdownEndsShellSessions(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	store := &workspaceStore{workspace: &model.Workspace{ID: uuid.New(), RootPath: t.TempDir()}}
	m := New(store, 50*time.Millisecond)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	session, err := m.Create(context.Background(), store.workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := m.Terminal(store.workspace.ID, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !terminal.Closed() {
		t.Fatal("shell terminal remained open after backend shutdown")
	}
	if _, err := m.Create(context.Background(), store.workspace.ID); !errors.Is(err, process.ErrClosed) {
		t.Fatalf("create after shutdown: %v", err)
	}
}

package dockercompose

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type workspaceStub struct{ item *model.Workspace }

func (s workspaceStub) Get(_ context.Context, id uuid.UUID) (*model.Workspace, error) {
	if id != s.item.ID {
		return nil, errors.New("wrong workspace")
	}
	return s.item, nil
}

func TestSnapshotListsDefinedServicesWithEngineUnavailable(t *testing.T) {
	root := t.TempDir()
	if _, err := workspaceconfig.Write(root, []byte("services: {}\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte("services:\n  postgres:\n    image: postgres:16\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runner := &stubRunner{reply: func(_ context.Context, args []string) ([]byte, error) {
		switch {
		case len(args) == 1 && args[0] == "--version":
			return []byte("Docker version 1"), nil
		case len(args) == 2 && args[0] == "compose" && args[1] == "version":
			return []byte("Docker Compose version 2"), nil
		case args[0] == "info":
			return nil, errors.New("permission denied")
		case args[0] == "compose" && args[len(args)-3] == "config":
			return []byte(`{"name":"test-project","services":{"postgres":{"image":"postgres:16"}}}`), nil
		default:
			t.Fatalf("unexpected Docker call: %v", args)
			return nil, nil
		}
	}}
	item := &model.Workspace{ID: uuid.New(), RootPath: root}
	controller := NewController(workspaceStub{item})
	controller.Manager.Runner = runner
	router := chi.NewRouter()
	router.Get("/workspaces/{id}/docker", controller.Snapshot)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workspaces/"+item.ID.String()+"/docker", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var snapshot Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Availability.State != "engine_unavailable" || snapshot.Project == nil || snapshot.Project.Name != "test-project" || len(snapshot.Project.Services) != 1 || snapshot.Project.Services[0].Name != "postgres" || snapshot.Error != "" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if len(runner.calls) != 4 {
		t.Fatalf("runtime status was requested with unavailable Engine: %+v", runner.calls)
	}
}

func TestDockerTerminalRejectsContainerOutsideWorkspaceProject(t *testing.T) {
	root := t.TempDir()
	if _, err := workspaceconfig.Write(root, []byte("services: {}\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte("services:\n  api:\n    image: api:dev\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runner := &stubRunner{reply: func(_ context.Context, args []string) ([]byte, error) {
		switch {
		case args[0] == "--version", args[0] == "info", args[0] == "compose" && len(args) == 2:
			return []byte("ready"), nil
		case args[0] == "compose" && slices.Contains(args, "config"):
			return []byte(`{"name":"demo","services":{"api":{"image":"api:dev"}}}`), nil
		case args[0] == "container":
			return nil, nil
		case args[0] == "compose" && slices.Contains(args, "ps"):
			return []byte(`[{"ID":"owned","Name":"demo-api-1","Project":"demo","Service":"api","State":"running"}]`), nil
		case args[0] == "inspect":
			return []byte(`{"state":{"Status":"running"},"restart_count":0,"ports":{}}`), nil
		default:
			t.Fatalf("unexpected Docker call: %v", args)
			return nil, nil
		}
	}}
	item := &model.Workspace{ID: uuid.New(), RootPath: root}
	controller := NewController(workspaceStub{item})
	controller.Manager.Runner = runner
	router := chi.NewRouter()
	router.Post("/workspaces/{id}/docker/terminals", controller.CreateTerminal)
	request := httptest.NewRequest(http.MethodPost, "/workspaces/"+item.ID.String()+"/docker/terminals", strings.NewReader(`{"container_id":"foreign"}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || len(controller.terminals) != 0 {
		t.Fatalf("foreign container terminal: %d %s", response.Code, response.Body.String())
	}
}

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	configModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/orchestration"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type orchestrationWorkspaces map[uuid.UUID]*workspaceModel.Workspace

func (s orchestrationWorkspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
	if item := s[id]; item != nil {
		return item, nil
	}
	return nil, workspaceRepo.ErrNotFound
}

func TestOrchestrationInvalidYAMLReturnsBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	orchestrationError(w, fmt.Errorf("%w: invalid service definition", workspaceconfig.ErrInvalid))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"code":"invalid_configuration"`) {
		t.Fatalf("invalid YAML returned internal error: %d %s", w.Code, w.Body.String())
	}
}

type orchestrationServices []*configModel.Service

func (s orchestrationServices) List(_ context.Context, id uuid.UUID, _, _ int) ([]*configModel.Service, error) {
	items := make([]*configModel.Service, 0)
	for _, item := range s {
		if item.WorkspaceID == id {
			items = append(items, item)
		}
	}
	return items, nil
}

type orchestrationRuntime map[uuid.UUID]process.Status

func (s orchestrationRuntime) Status(id uuid.UUID) process.Status {
	if status, ok := s[id]; ok {
		return status
	}
	return process.Status{ServiceID: id, State: process.Stopped}
}
func (orchestrationRuntime) Closed() bool { return false }

type orchestrationLifecycle struct{ runtime orchestrationRuntime }

func (s orchestrationLifecycle) Start(_ context.Context, id uuid.UUID) (process.Status, error) {
	status := process.Status{ServiceID: id, State: process.Running}
	s.runtime[id] = status
	return status, nil
}
func (s orchestrationLifecycle) Stop(_ context.Context, id uuid.UUID) (process.Status, error) {
	status := process.Status{ServiceID: id, State: process.Stopped}
	s.runtime[id] = status
	return status, nil
}

func TestOrchestrationAPI(t *testing.T) {
	id, serviceID := uuid.New(), uuid.New()
	workspaces := orchestrationWorkspaces{id: {ID: id, Name: "Web"}}
	services := orchestrationServices{{ID: serviceID, WorkspaceID: id, Name: "API"}}
	runtime := orchestrationRuntime{}
	c := NewOrchestration(orchestration.New(workspaces, services, orchestrationLifecycle{runtime}, runtime))
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Post("/workspaces/{id}/start", c.Start)
		r.Post("/workspaces/{id}/stop", c.Stop)
		r.Get("/workspaces/{id}/status", c.Status)
	})
	request := func(method, path, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	base := "/api/v1/workspaces/" + id.String()
	for _, path := range []string{base + "/start", base + "/stop", base + "/status"} {
		method := http.MethodPost
		if strings.HasSuffix(path, "/status") {
			method = http.MethodGet
		}
		if w := request(method, path, ""); w.Code != http.StatusUnauthorized {
			t.Fatalf("unauthorized %s: %d", path, w.Code)
		}
		if w := request(method, "/api/v1/workspaces/bad"+path[strings.LastIndex(path, "/"):], "token"); w.Code != http.StatusBadRequest {
			t.Fatalf("bad ID %s: %d %s", path, w.Code, w.Body.String())
		}
		if w := request(method, "/api/v1/workspaces/"+uuid.NewString()+path[strings.LastIndex(path, "/"):], "token"); w.Code != http.StatusNotFound {
			t.Fatalf("missing workspace %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := request(http.MethodPost, base+"/start", "token"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"outcome":"started"`) {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	w := request(http.MethodGet, base+"/status", "token")
	var status orchestration.WorkspaceStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || w.Code != http.StatusOK || status.AggregateState != "running" || status.Services[0].ServiceID != serviceID {
		t.Fatalf("status: %d %s %v", w.Code, w.Body.String(), err)
	}
	if w := request(http.MethodPost, base+"/stop", "token"); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"outcome":"stopped"`) {
		t.Fatalf("stop: %d %s", w.Code, w.Body.String())
	}
	runtime[serviceID] = process.Status{ServiceID: serviceID, State: process.Failed, LastError: "private execution detail"}
	if w := request(http.MethodGet, base+"/status", "token"); w.Code != http.StatusOK || strings.Contains(w.Body.String(), "private execution detail") || !strings.Contains(w.Body.String(), "service execution failed") {
		t.Fatalf("status leaked internal error: %d %s", w.Code, w.Body.String())
	}
}

package controller

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	"github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	configRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

type configStore map[uuid.UUID]*model.Service

func (s configStore) Get(_ context.Context, id uuid.UUID) (*model.Service, error) {
	if item := s[id]; item != nil {
		return item, nil
	}
	return nil, configRepo.ErrNotFound
}
func (configStore) Create(context.Context, *model.Service) error { return nil }
func (configStore) List(context.Context, uuid.UUID, int, int) ([]*model.Service, error) {
	return nil, nil
}
func (configStore) Update(context.Context, uuid.UUID, map[string]any) (*model.Service, error) {
	return nil, nil
}
func (configStore) Delete(context.Context, uuid.UUID) error { return nil }
func (configStore) NameExists(context.Context, uuid.UUID, string, uuid.UUID) (bool, error) {
	return false, nil
}

type workspaceStore map[uuid.UUID]*workspaceModel.Workspace

func (s workspaceStore) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
	if item := s[id]; item != nil {
		return item, nil
	}
	return nil, workspaceRepo.ErrNotFound
}
func (workspaceStore) Create(context.Context, *workspaceModel.Workspace) error { return nil }
func (workspaceStore) List(context.Context, int, int) ([]*workspaceModel.Workspace, error) {
	return nil, nil
}
func (workspaceStore) Update(context.Context, uuid.UUID, map[string]any) (*workspaceModel.Workspace, error) {
	return nil, nil
}
func (workspaceStore) Delete(context.Context, uuid.UUID) error { return nil }

func TestHTTPChild(t *testing.T) {
	if os.Getenv("DEVORBIT_HTTP_CHILD") == "1" {
		for {
			time.Sleep(time.Second)
		}
	}
}

func TestProcessAPI(t *testing.T) {
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	wid, id, disabledID := uuid.New(), uuid.New(), uuid.New()
	enabled, disabled := true, false
	workspaces := workspaceStore{wid: {ID: wid, RootPath: t.TempDir()}}
	configs := configStore{
		id:         {ID: id, WorkspaceID: wid, Command: self, Args: []string{"-test.run=^TestHTTPChild$"}, Env: map[string]string{"DEVORBIT_HTTP_CHILD": "1"}, Enabled: &enabled},
		disabledID: {ID: disabledID, WorkspaceID: wid, Command: self, Enabled: &disabled},
	}
	supervisor := process.New(100 * time.Millisecond)
	t.Cleanup(func() { _ = supervisor.Shutdown(context.Background()) })
	c := New(service.New(configService.New(configs, workspaces), workspaceService.New(workspaces), supervisor))
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Post("/services/{id}/start", c.Start)
		r.Post("/services/{id}/stop", c.Stop)
		r.Post("/services/{id}/restart", c.Restart)
		r.Get("/services/{id}/status", c.Status)
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
	base := "/api/v1/services/" + id.String()
	for _, tc := range []struct {
		method, path, token string
		want                int
		state               process.State
	}{
		{http.MethodPost, base + "/start", "", 401, ""},
		{http.MethodPost, base + "/stop", "", 401, ""},
		{http.MethodPost, base + "/restart", "", 401, ""},
		{http.MethodGet, base + "/status", "", 401, ""},
		{http.MethodPost, "/api/v1/services/bad/start", "token", 400, ""},
		{http.MethodPost, "/api/v1/services/bad/stop", "token", 400, ""},
		{http.MethodPost, "/api/v1/services/bad/restart", "token", 400, ""},
		{http.MethodGet, "/api/v1/services/" + uuid.NewString() + "/status", "token", 404, ""},
		{http.MethodPost, "/api/v1/services/" + uuid.NewString() + "/stop", "token", 404, ""},
		{http.MethodPost, "/api/v1/services/" + uuid.NewString() + "/restart", "token", 404, ""},
		{http.MethodPost, "/api/v1/services/" + disabledID.String() + "/start", "token", 409, ""},
		{http.MethodPost, "/api/v1/services/" + disabledID.String() + "/restart", "token", 409, ""},
		{http.MethodPost, "/api/v1/services/" + disabledID.String() + "/stop", "token", 200, ""},
		{http.MethodGet, base + "/status", "token", 200, process.Stopped},
		{http.MethodPost, base + "/start", "token", 202, process.Running},
		{http.MethodPost, base + "/start", "token", 409, ""},
		{http.MethodGet, base + "/status", "token", 200, process.Running},
	} {
		w := request(tc.method, tc.path, tc.token)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if tc.state != "" {
			var status process.Status
			if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.ServiceID != id || status.State != tc.state {
				t.Fatalf("status response: %s (%v)", w.Body.String(), err)
			}
		}
		if strings.Contains(w.Body.String(), "DEVORBIT_HTTP_CHILD") {
			t.Fatalf("response exposed environment: %s", w.Body.String())
		}
	}
	before := supervisor.Status(id)
	w := request(http.MethodPost, base+"/restart", "token")
	if w.Code != http.StatusOK {
		t.Fatalf("restart: %d %s", w.Code, w.Body.String())
	}
	var restarted process.Status
	if err := json.Unmarshal(w.Body.Bytes(), &restarted); err != nil || restarted.State != process.Running || *restarted.PID == *before.PID {
		t.Fatalf("restart status: %+v %v", restarted, err)
	}
	w = request(http.MethodPost, base+"/stop", "token")
	var stopped process.Status
	if err := json.Unmarshal(w.Body.Bytes(), &stopped); err != nil || w.Code != http.StatusOK || stopped.State != process.Stopped || stopped.ExitedAt == nil {
		t.Fatalf("stop: %d %+v %v", w.Code, stopped, err)
	}
	if w := request(http.MethodPost, base+"/stop", "token"); w.Code != http.StatusOK {
		t.Fatalf("repeat stop: %d %s", w.Code, w.Body.String())
	}
	configs[id].Command = "missing-devorbit-executable"
	if w := request(http.MethodPost, base+"/restart", "token"); w.Code != http.StatusBadRequest {
		t.Fatalf("restart should use current config: %d %s", w.Code, w.Body.String())
	}
}

package controller

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type apiStore struct{ items map[uuid.UUID]*model.Identity }

func (s *apiStore) Identity(_ context.Context, id uuid.UUID) (*model.Identity, error) {
	if item := s.items[id]; item != nil {
		return item, nil
	}
	return nil, repository.ErrNotFound
}
func (s *apiStore) Resolve(_ context.Context, wid uuid.UUID, key string) (*model.Identity, error) {
	for _, item := range s.items {
		if item.WorkspaceID == wid && item.Key == key {
			return item, nil
		}
	}
	item := &model.Identity{ID: uuid.New(), WorkspaceID: wid, Key: key}
	s.items[item.ID] = item
	return item, nil
}

type apiWorkspaces map[uuid.UUID]*workspaceModel.Workspace

func (w apiWorkspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
	if item := w[id]; item != nil {
		return item, nil
	}
	return nil, workspaceRepository.ErrNotFound
}
func TestServiceAPI(t *testing.T) {
	wid := uuid.New()
	root := t.TempDir()
	if _, err := workspaceconfig.Write(root, []byte("services: {}\n"), ""); err != nil {
		t.Fatal(err)
	}
	c := New(service.New(&apiStore{items: map[uuid.UUID]*model.Identity{}}, apiWorkspaces{wid: {ID: wid, RootPath: root}}))
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Get("/workspaces/{id}/configuration", c.Configuration)
		r.Post("/workspaces/{id}/configuration/validate", c.Validate)
		r.Put("/workspaces/{id}/configuration", c.Save)
		r.Post("/workspaces/{id}/services", c.Create)
		r.Get("/workspaces/{id}/services", c.List)
		r.Get("/services/{id}", c.Get)
		r.Patch("/services/{id}", c.Update)
		r.Delete("/services/{id}", c.Delete)
		r.Get("/services/{id}/environment", c.ListEnvironment)
		r.Get("/services/{id}/environment/{name}", c.ReadEnvironment)
		r.Put("/services/{id}/environment/{name}", c.SaveEnvironment)
	})
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer token")
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	base := "/api/v1/workspaces/" + wid.String()
	w := request("GET", base+"/configuration", "")
	if w.Code != 200 {
		t.Fatalf("configuration: %d %s", w.Code, w.Body.String())
	}
	var snapshot workspaceconfig.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", base+"/services", `{"name":"api","command":"go","ports":{"http":8080},"revision":"`+snapshot.Revision+`"}`); w.Code != 400 {
		t.Fatalf("removed ports field accepted on create: %d %s", w.Code, w.Body.String())
	}
	w = request("POST", base+"/services", `{"name":"api","command":"go","args":["run","."],"revision":"`+snapshot.Revision+`"}`)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"ports"`) {
		t.Fatalf("removed ports field returned: %s", w.Body.String())
	}
	var item model.Service
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if w := request("PATCH", "/api/v1/services/"+item.ID.String(), `{"ports":{"http":8080},"revision":"`+snapshot.Revision+`"}`); w.Code != 400 {
		t.Fatalf("removed ports field accepted on update: %d %s", w.Code, w.Body.String())
	}
	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("VALUE=1\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	envURL := "/api/v1/services/" + item.ID.String() + "/environment"
	if w := request("GET", envURL, ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != `[".env"]` {
		t.Fatalf("environment list: %d %s", w.Code, w.Body.String())
	}
	w = request("GET", envURL+"/.env", "")
	if w.Code != 200 {
		t.Fatalf("environment read: %d %s", w.Code, w.Body.String())
	}
	var env service.EnvFile
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil || env.Content != "VALUE=1\r\n" {
		t.Fatalf("environment read: %+v %v", env, err)
	}
	if w := request("PUT", envURL+"/.env", `{"revision":"`+env.Revision+`"}`); w.Code != 400 {
		t.Fatalf("missing content: %d %s", w.Code, w.Body.String())
	}
	w = request("PUT", envURL+"/.env", `{"content":"VALUE=2\r\n","revision":"`+env.Revision+`"}`)
	if w.Code != 200 {
		t.Fatalf("environment save: %d %s", w.Code, w.Body.String())
	}
	if data, err := os.ReadFile(envPath); err != nil || string(data) != "VALUE=2\r\n" {
		t.Fatalf("environment on disk: %q %v", data, err)
	}
	if w := request("PUT", envURL+"/.env", `{"content":"OLD=1","revision":"`+env.Revision+`"}`); w.Code != 409 {
		t.Fatalf("stale environment save: %d %s", w.Code, w.Body.String())
	}
	if w := request("GET", envURL+"/.env.local", ""); w.Code != 404 || !strings.Contains(w.Body.String(), "environment file not found") {
		t.Fatalf("missing environment file: %d %s", w.Code, w.Body.String())
	}
	if w := request("POST", base+"/services", `{"name":"other","command":"go","revision":"`+snapshot.Revision+`"}`); w.Code != 409 {
		t.Fatalf("stale create: %d %s", w.Code, w.Body.String())
	}
	w = request("GET", base+"/configuration", "")
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", base+"/configuration/validate", `{"content":"services:\n  api:\n    command: go\n    wrong: true\n"}`); w.Code != 400 {
		t.Fatalf("invalid YAML: %d %s", w.Code, w.Body.String())
	}
	if w := request("PUT", base+"/configuration", `{"content":"services: {}\n","revision":"`+snapshot.Revision+`"}`); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	if w := request("GET", "/api/v1/services/"+item.ID.String(), ""); w.Code != 404 {
		t.Fatalf("removed: %d %s", w.Code, w.Body.String())
	}
}

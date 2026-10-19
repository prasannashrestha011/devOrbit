package controller

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
)

type apiStore struct{ items map[uuid.UUID]*model.Service }

func (s *apiStore) Create(_ context.Context, item *model.Service) error {
	s.items[item.ID] = item
	return nil
}
func (s *apiStore) Get(_ context.Context, id uuid.UUID) (*model.Service, error) {
	if item := s.items[id]; item != nil {
		return item, nil
	}
	return nil, repository.ErrNotFound
}
func (s *apiStore) List(_ context.Context, wid uuid.UUID, _, _ int) ([]*model.Service, error) {
	items := []*model.Service{}
	for _, item := range s.items {
		if item.WorkspaceID == wid {
			items = append(items, item)
		}
	}
	return items, nil
}
func (s *apiStore) Update(ctx context.Context, id uuid.UUID, changes map[string]any) (*model.Service, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if value, ok := changes["enabled"]; ok {
		item.Enabled = new(bool)
		*item.Enabled = value.(bool)
	}
	if value, ok := changes["auto_restart"]; ok {
		item.AutoRestart = value.(bool)
	}
	if value, ok := changes["args"]; ok {
		_ = json.Unmarshal([]byte(value.(string)), &item.Args)
	}
	if value, ok := changes["env"]; ok {
		_ = json.Unmarshal([]byte(value.(string)), &item.Env)
	}
	return item, nil
}
func (s *apiStore) Delete(_ context.Context, id uuid.UUID) error {
	if s.items[id] == nil {
		return repository.ErrNotFound
	}
	delete(s.items, id)
	return nil
}
func (s *apiStore) NameExists(_ context.Context, wid uuid.UUID, name string, except uuid.UUID) (bool, error) {
	for _, item := range s.items {
		if item.WorkspaceID == wid && item.ID != except && item.Name == name {
			return true, nil
		}
	}
	return false, nil
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
	store := &apiStore{items: map[uuid.UUID]*model.Service{}}
	c := New(service.New(store, apiWorkspaces{wid: {ID: wid, RootPath: t.TempDir()}}))
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Post("/workspaces/{id}/services", c.Create)
		r.Get("/workspaces/{id}/services", c.List)
		r.Get("/services/{id}", c.Get)
		r.Patch("/services/{id}", c.Update)
		r.Delete("/services/{id}", c.Delete)
	})
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	path := "/api/v1/workspaces/" + wid.String() + "/services"
	if w := request("GET", path, "", ""); w.Code != 401 {
		t.Fatalf("unauthorized: %d", w.Code)
	}
	if w := request("GET", path, "", "token"); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %s", w.Code, w.Body.String())
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/api/v1/workspaces/bad/services", `{}`, 400},
		{"POST", "/api/v1/workspaces/" + uuid.NewString() + "/services", `{"name":"x","command":"go"}`, 404},
		{"POST", path, `{"name":"x","command":"go","args":[1]}`, 400},
		{"POST", path, `{"name":"x","command":"go","env":{"A":1}}`, 400},
		{"POST", path, `{"name":"x","command":"go","enabled":null}`, 400},
		{"POST", path, `{"name":"x","command":"go","unknown":1}`, 400},
		{"GET", path + "?limit=0", "", 400},
		{"GET", path + "?offset=-1", "", 400},
		{"GET", path + "?offset=1000001", "", 400},
		{"GET", "/api/v1/services/bad", "", 400},
	} {
		w := request(tc.method, tc.path, tc.body, "token")
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	w := request("POST", path, `{"name":"api","command":"go","args":["run"],"enabled":false,"auto_restart":true}`, "token")
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var item model.Service
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil || item.ID == uuid.Nil || *item.Enabled || !item.AutoRestart {
		t.Fatalf("create response: %+v %v", item, err)
	}
	if w := request("POST", path, `{"name":"api","command":"go"}`, "token"); w.Code != 409 {
		t.Fatalf("duplicate: %d", w.Code)
	}
	idPath := "/api/v1/services/" + item.ID.String()
	for _, tc := range []struct {
		method, body string
		status       int
	}{
		{"GET", "", 200}, {"PATCH", `{}`, 400}, {"PATCH", `{"args":null}`, 400},
		{"PATCH", `{"workspace_id":"` + uuid.NewString() + `"}`, 400},
		{"PATCH", `{"args":[],"env":{},"enabled":false,"auto_restart":false}`, 200},
	} {
		w := request(tc.method, idPath, tc.body, "token")
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.method, w.Code, w.Body.String())
		}
	}
	w = request("GET", idPath, "", "token")
	if !strings.Contains(w.Body.String(), `"args":[]`) || !strings.Contains(w.Body.String(), `"env":{}`) || !strings.Contains(w.Body.String(), `"auto_restart":false`) {
		t.Fatalf("patch result: %s", w.Body.String())
	}
	if w := request("DELETE", idPath, "", "token"); w.Code != 204 {
		t.Fatalf("delete: %d", w.Code)
	}
	if w := request("GET", idPath, "", "token"); w.Code != 404 {
		t.Fatalf("missing get: %d", w.Code)
	}
	if w := request("DELETE", idPath, "", "token"); w.Code != 404 {
		t.Fatalf("missing delete: %d", w.Code)
	}
	if w := request("GET", "/api/v1/workspaces/"+uuid.NewString()+"/services", "", "token"); w.Code != 404 {
		t.Fatalf("missing workspace list: %d", w.Code)
	}
	if w := request(http.MethodGet, path, "", "token"); w.Code != 200 {
		t.Fatalf("final list: %d", w.Code)
	}
}

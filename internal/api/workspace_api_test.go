package api

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
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
)

type workspaceMemoryStore struct {
	items map[uuid.UUID]*model.Workspace
}

func (s *workspaceMemoryStore) Create(_ context.Context, item *model.Workspace) error {
	s.items[item.ID] = item
	return nil
}
func (s *workspaceMemoryStore) Get(_ context.Context, id uuid.UUID) (*model.Workspace, error) {
	if item := s.items[id]; item != nil {
		return item, nil
	}
	return nil, repository.ErrNotFound
}
func (s *workspaceMemoryStore) List(_ context.Context, _, _ int) ([]*model.Workspace, error) {
	items := []*model.Workspace{}
	for _, item := range s.items {
		items = append(items, item)
	}
	return items, nil
}
func (s *workspaceMemoryStore) Update(ctx context.Context, id uuid.UUID, changes map[string]any) (*model.Workspace, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if name, ok := changes["name"]; ok {
		item.Name = name.(string)
	}
	if description, ok := changes["description"]; ok {
		item.Description = description.(string)
	}
	return item, nil
}
func (s *workspaceMemoryStore) Delete(_ context.Context, id uuid.UUID) error {
	if s.items[id] == nil {
		return repository.ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func TestWorkspaceAPI(t *testing.T) {
	store := &workspaceMemoryStore{items: make(map[uuid.UUID]*model.Workspace)}
	svc := service.New(func(context.Context) (service.Store, error) { return store, nil })
	c := controller.New(svc)
	r := chi.NewRouter()
	r.Post("/", c.Create)
	r.Get("/", c.List)
	r.Get("/{id}", c.Get)
	r.Patch("/{id}", c.Update)
	r.Delete("/{id}", c.Delete)
	api := Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "test-token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, r)
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, req)
		return w
	}
	path := "/api/v1/workspaces"
	if w := request(http.MethodGet, path, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", w.Code)
	}
	if w := request(http.MethodGet, path, "", "test-token"); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %s", w.Code, w.Body.String())
	}
	for _, tc := range []struct{ method, suffix, body string }{
		{http.MethodPost, "", `{"name":" ","root_path":"/tmp"}`},
		{http.MethodPost, "", `{"name":"x","root_path":"/tmp","extra":true}`},
		{http.MethodGet, "?limit=0", ""},
		{http.MethodGet, "?offset=-1", ""},
		{http.MethodGet, "/invalid", ""},
		{http.MethodPatch, "/" + uuid.NewString(), `{}`},
		{http.MethodPatch, "/" + uuid.NewString(), `{"id":"bad"}`},
	} {
		w := request(tc.method, path+tc.suffix, tc.body, "test-token")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s %s: %d %s", tc.method, tc.suffix, w.Code, w.Body.String())
		}
	}
	root := t.TempDir()
	createBody, _ := json.Marshal(map[string]string{"name": " project ", "description": "notes", "root_path": root})
	created := request(http.MethodPost, path, string(createBody), "test-token")
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var item model.Workspace
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil || item.ID == uuid.Nil || item.Name != "project" {
		t.Fatalf("create response: %+v %v", item, err)
	}
	idPath := path + "/" + item.ID.String()
	for _, tc := range []struct {
		method, suffix, body string
		status               int
	}{
		{http.MethodGet, "", "", http.StatusOK},
		{http.MethodGet, "/" + item.ID.String(), "", http.StatusOK},
		{http.MethodPatch, "/" + item.ID.String(), `{"description":""}`, http.StatusOK},
		{http.MethodPatch, "/" + item.ID.String(), `{"name":null}`, http.StatusBadRequest},
		{http.MethodPatch, "/" + item.ID.String(), `{"created_at":"bad"}`, http.StatusBadRequest},
	} {
		w := request(tc.method, path+tc.suffix, tc.body, "test-token")
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.suffix, w.Code, w.Body.String())
		}
	}
	if w := request(http.MethodGet, idPath, "", "test-token"); !strings.Contains(w.Body.String(), `"description":""`) {
		t.Fatalf("description was not cleared: %s", w.Body.String())
	}
	if w := request(http.MethodDelete, idPath, "", "test-token"); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodGet, idPath, "", "test-token"); w.Code != http.StatusNotFound {
		t.Fatalf("missing get: %d %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodDelete, idPath, "", "test-token"); w.Code != http.StatusNotFound {
		t.Fatalf("missing delete: %d %s", w.Code, w.Body.String())
	}
}

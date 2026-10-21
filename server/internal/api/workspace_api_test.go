package api

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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type workspaceMemoryStore struct {
	items     map[uuid.UUID]*model.Workspace
	deleteErr error
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
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if s.items[id] == nil {
		return repository.ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func TestWorkspaceAPI(t *testing.T) {
	store := &workspaceMemoryStore{items: make(map[uuid.UUID]*model.Workspace)}
	dataDir := t.TempDir()
	svc := service.New(store, dataDir)
	c := controller.New(svc, nil)
	api := Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "test-token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(router chi.Router) {
		router.Post("/workspaces", c.Create)
		router.Get("/workspaces", c.List)
		router.Get("/workspaces/{id}", c.Get)
		router.Patch("/workspaces/{id}", c.Update)
		router.Delete("/workspaces/{id}", c.Delete)
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
		{http.MethodPost, "", `{"name":" "}`},
		{http.MethodPost, "", `{"name":"x","root_path":"/tmp"}`},
		{http.MethodGet, "?limit=0", ""},
		{http.MethodGet, "?offset=-1", ""},
		{http.MethodGet, "?offset=1000001", ""},
		{http.MethodGet, "/invalid", ""},
		{http.MethodPatch, "/" + uuid.NewString(), `{}`},
		{http.MethodPatch, "/" + uuid.NewString(), `{"id":"bad"}`},
	} {
		w := request(tc.method, path+tc.suffix, tc.body, "test-token")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s %s: %d %s", tc.method, tc.suffix, w.Code, w.Body.String())
		}
	}
	missingType := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"name":"x"}`))
	missingType.Header.Set("Authorization", "Bearer test-token")
	missingTypeResponse := httptest.NewRecorder()
	api.ServeHTTP(missingTypeResponse, missingType)
	if missingTypeResponse.Code != http.StatusBadRequest {
		t.Fatalf("missing JSON content type: %d", missingTypeResponse.Code)
	}
	createBody, _ := json.Marshal(map[string]string{"name": " project ", "description": "notes"})
	created := request(http.MethodPost, path, string(createBody), "test-token")
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var item model.Workspace
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil || item.ID == uuid.Nil || item.Name != "project" {
		t.Fatalf("create response: %+v %v", item, err)
	}
	root := filepath.Join(dataDir, "workspaces", item.ID.String())
	if item.RootPath != root {
		t.Fatalf("workspace root = %q, want %q", item.RootPath, root)
	}
	idPath := path + "/" + item.ID.String()
	config, err := workspaceconfig.Read(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceconfig.Write(root, []byte("services:\n  api:\n    command: go run .\n"), config.Revision); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		method, suffix, body string
		status               int
	}{
		{http.MethodGet, "", "", http.StatusOK},
		{http.MethodGet, "/" + item.ID.String(), "", http.StatusOK},
		{http.MethodPatch, "/" + item.ID.String(), `{"description":""}`, http.StatusOK},
		{http.MethodPatch, "/" + item.ID.String(), `{"name":null}`, http.StatusBadRequest},
		{http.MethodPatch, "/" + item.ID.String(), `{"created_at":"bad"}`, http.StatusBadRequest},
		{http.MethodPatch, "/" + item.ID.String(), `{"root_path":"/tmp"}`, http.StatusBadRequest},
	} {
		w := request(tc.method, path+tc.suffix, tc.body, "test-token")
		if w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.suffix, w.Code, w.Body.String())
		}
	}
	if w := request(http.MethodGet, idPath, "", "test-token"); !strings.Contains(w.Body.String(), `"description":""`) {
		t.Fatalf("description was not cleared: %s", w.Body.String())
	}
	store.deleteErr = repository.ErrConflict
	if w := request(http.MethodDelete, idPath, "", "test-token"); w.Code != http.StatusConflict {
		t.Fatalf("workspace delete conflict: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(workspaceconfig.Path(root)); err != nil {
		t.Fatalf("configuration lost on delete conflict: %v", err)
	}
	store.deleteErr = nil
	if w := request(http.MethodDelete, idPath, "", "test-token"); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(workspaceconfig.Path(root)); !os.IsNotExist(err) {
		t.Fatalf("configuration remained after workspace deletion: %v", err)
	}
	if w := request(http.MethodGet, idPath, "", "test-token"); w.Code != http.StatusNotFound {
		t.Fatalf("missing get: %d %s", w.Code, w.Body.String())
	}
	if w := request(http.MethodDelete, idPath, "", "test-token"); w.Code != http.StatusNotFound {
		t.Fatalf("missing delete: %d %s", w.Code, w.Body.String())
	}
}

package controller

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	serviceService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
)

type services map[uuid.UUID]*serviceModel.Identity

func (s services) Identity(_ context.Context, id uuid.UUID) (*serviceModel.Identity, error) {
	if v := s[id]; v != nil {
		return v, nil
	}
	return nil, serviceRepo.ErrNotFound
}
func (s services) Resolve(_ context.Context, wid uuid.UUID, key string) (*serviceModel.Identity, error) {
	for _, item := range s {
		if item.WorkspaceID == wid && item.Key == key {
			return item, nil
		}
	}
	return nil, serviceRepo.ErrNotFound
}

type workspaces map[uuid.UUID]*workspaceModel.Workspace

func (s workspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
	if v := s[id]; v != nil {
		return v, nil
	}
	return nil, workspaceRepo.ErrNotFound
}
func (workspaces) Create(context.Context, *workspaceModel.Workspace) error { return nil }
func (workspaces) List(context.Context, int, int) ([]*workspaceModel.Workspace, error) {
	return nil, nil
}
func (workspaces) Update(context.Context, uuid.UUID, map[string]any) (*workspaceModel.Workspace, error) {
	return nil, nil
}
func (workspaces) Delete(context.Context, uuid.UUID) error { return nil }

func TestLogAPI(t *testing.T) {
	m, err := logs.New(filepath.Join(t.TempDir(), "logs"))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	wid, sid := uuid.New(), uuid.New()
	capture, err := m.Begin(wid, sid)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = capture.Stdout.Write([]byte("one\ntwo\n"))
	if err := capture.Close(); err != nil {
		t.Fatal(err)
	}
	c := New(m, serviceService.New(services{sid: {ID: sid, WorkspaceID: wid}}, workspaces{wid: {ID: wid}}), workspaceService.New(workspaces{wid: {ID: wid}}, ""))
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Get("/services/{id}/logs", c.Service)
		r.Get("/workspaces/{id}/logs", c.Workspace)
	})
	request := func(path, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	base := "/api/v1/services/" + sid.String() + "/logs"
	for _, tc := range []struct {
		path, token string
		code        int
	}{
		{base, "", 401}, {"/api/v1/services/bad/logs", "token", 400},
		{base + "?limit=0", "token", 400}, {"/api/v1/services/" + uuid.NewString() + "/logs", "token", 404},
		{base + "?limit=1&limit=2", "token", 400}, {base + "?unknown=1", "token", 400},
		{"/api/v1/workspaces/" + uuid.NewString() + "/logs", "token", 404},
	} {
		if w := request(tc.path, tc.token); w.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	w := request(base+"?limit=1", "token")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"message":"two\n"`) || strings.Contains(w.Body.String(), `"message":"one\n"`) {
		t.Fatalf("service logs: %d %s", w.Code, w.Body.String())
	}
	w = request("/api/v1/workspaces/"+wid.String()+"/logs", "token")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"message":"one\n"`) || !strings.Contains(w.Body.String(), `"message":"two\n"`) {
		t.Fatalf("workspace logs: %d %s", w.Code, w.Body.String())
	}
}

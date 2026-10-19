//go:build !windows

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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/shellterminal"
)

type store struct{ workspace *model.Workspace }

func (s *store) Get(_ context.Context, id uuid.UUID) (*model.Workspace, error) {
	if s.workspace != nil && s.workspace.ID == id {
		return s.workspace, nil
	}
	return nil, shellterminal.ErrNotFound
}
func (s *store) Delete(_ context.Context, _ uuid.UUID) error { s.workspace = nil; return nil }

func TestShellTerminalAPIAuthenticationAndWorkspaceScope(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	workspaceID := uuid.New()
	manager := shellterminal.New(&store{workspace: &model.Workspace{ID: workspaceID, RootPath: t.TempDir()}}, 50*time.Millisecond)
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	c := New(manager)
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Get("/workspaces/{workspace}/terminals", c.List)
		r.Post("/workspaces/{workspace}/terminals", c.Create)
		r.Delete("/workspaces/{workspace}/terminals/{terminal}", c.Delete)
		r.Get("/workspaces/{workspace}/terminals/{terminal}/stream", c.Stream)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	base := server.URL + "/api/v1/workspaces/" + workspaceID.String() + "/terminals"
	request := func(method, url, token string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	if response := request(http.MethodPost, base, ""); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized create: %d", response.StatusCode)
	}
	response := request(http.MethodPost, base, "token")
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", response.StatusCode)
	}
	var session shellterminal.Session
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/workspaces/" + workspaceID.String() + "/terminals/" + session.ID.String() + "/stream"
	headers := http.Header{"Authorization": {"Bearer token"}}
	if conn, reply, err := websocket.DefaultDialer.Dial(strings.Replace(endpoint, workspaceID.String(), uuid.NewString(), 1), headers); err == nil || reply == nil || reply.StatusCode != http.StatusNotFound {
		if conn != nil {
			conn.Close()
		}
		t.Fatalf("cross-workspace attach: response=%v err=%v", reply, err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, headers)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(map[string]any{"type": "input", "data": "echo ready\n"}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	seen := false
	for !seen {
		var frame process.TerminalFrame
		if err := conn.ReadJSON(&frame); err != nil {
			t.Fatal(err)
		}
		seen = strings.Contains(string(frame.Data), "ready")
	}
	_ = conn.Close()
	sessions, err := manager.List(context.Background(), workspaceID)
	if err != nil || len(sessions) != 1 || sessions[0].State != process.Running {
		t.Fatalf("viewer close stopped shell: %+v %v", sessions, err)
	}
	if response := request(http.MethodDelete, base+"/"+session.ID.String(), "token"); response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", response.StatusCode)
	}
}

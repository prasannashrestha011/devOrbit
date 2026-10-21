package controller

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	"github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	configRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type configStore map[uuid.UUID]*model.Identity

func (s configStore) Identity(_ context.Context, id uuid.UUID) (*model.Identity, error) {
	if item := s[id]; item != nil {
		return item, nil
	}
	return nil, configRepo.ErrNotFound
}
func (s configStore) Resolve(_ context.Context, wid uuid.UUID, key string) (*model.Identity, error) {
	for _, item := range s {
		if item.WorkspaceID == wid && item.Key == key {
			return item, nil
		}
	}
	return nil, configRepo.ErrNotFound
}
func writeConfig(t *testing.T, root, name, command string) {
	t.Helper()
	data, err := workspaceconfig.Encode(workspaceconfig.Config{Services: map[string]workspaceconfig.Service{name: {Command: command}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceconfig.Write(root, data, ""); err != nil {
		t.Fatal(err)
	}
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
	if os.Getenv("DEVORBIT_HTTP_CHILD") == "input" {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err == nil {
			fmt.Print("ack:", strings.TrimSpace(line), "\r\n")
		}
		return
	}
	if os.Getenv("DEVORBIT_HTTP_CHILD") == "1" {
		for {
			time.Sleep(time.Second)
		}
	}
}

func TestServiceTerminalInputOverWebSocket(t *testing.T) {
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	wid, id := uuid.New(), uuid.New()
	root := t.TempDir()
	workspaces := workspaceStore{wid: {ID: wid, RootPath: root}}
	configs := configStore{id: {ID: id, WorkspaceID: wid, Key: "child"}}
	writeConfig(t, root, "child", "env DEVORBIT_HTTP_CHILD=input "+self+" -test.run=^TestHTTPChild")
	supervisor := process.New(50 * time.Millisecond)
	t.Cleanup(func() { _ = supervisor.Shutdown(context.Background()) })
	c := New(service.New(configService.New(configs, workspaces), workspaceService.New(workspaces, ""), supervisor))
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Get("/services/{id}/terminal/{execution}", c.Terminal)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	status, err := c.service.Start(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/services/" + id.String() + "/terminal/" + status.ExecutionID.String()
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, http.Header{"Authorization": {"Bearer token"}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]any{"type": "input", "data": "ping\r"}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var output string
	for !strings.Contains(output, "ack:ping") {
		var frame process.TerminalFrame
		if err := conn.ReadJSON(&frame); err != nil {
			t.Fatalf("reading terminal output: %v (%q)", err, output)
		}
		output += string(frame.Data)
		if frame.Type == "stopped" {
			t.Fatalf("service exited before reading input: %q", output)
		}
	}
	if first, reply := strings.Index(output, "ping"), strings.Index(output, "ack:ping"); first < 0 || reply < first {
		t.Fatalf("terminal did not echo visible input before the child response: %q", output)
	}
	deadline := time.Now().Add(5 * time.Second)
	current, err := c.service.Status(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	for current.State == process.Running && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		current, err = c.service.Status(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
	}
	if current.State != process.Exited {
		t.Fatalf("service status after command exit: %s", current.State)
	}
	if err := conn.WriteJSON(map[string]any{"type": "input", "data": "pwd\r"}); err != nil {
		t.Fatal(err)
	}
	for !strings.Contains(output, root) {
		var frame process.TerminalFrame
		if err := conn.ReadJSON(&frame); err != nil {
			t.Fatalf("tmux shell did not accept command after service exit: %v (%q)", err, output)
		}
		output += string(frame.Data)
	}
}

func TestProcessAPI(t *testing.T) {
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	wid, id := uuid.New(), uuid.New()
	root := t.TempDir()
	workspaces := workspaceStore{wid: {ID: wid, RootPath: root}}
	configs := configStore{id: {ID: id, WorkspaceID: wid, Key: "child"}}
	writeConfig(t, root, "child", "env DEVORBIT_HTTP_CHILD=1 "+self+" -test.run=^TestHTTPChild")
	supervisor := process.New(100 * time.Millisecond)
	t.Cleanup(func() { _ = supervisor.Shutdown(context.Background()) })
	c := New(service.New(configService.New(configs, workspaces), workspaceService.New(workspaces, ""), supervisor))
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
	snapshot, _ := workspaceconfig.Read(root)
	if _, err := workspaceconfig.Write(root, []byte("services:\n  child:\n    command: missing-devorbit-executable\n"), snapshot.Revision); err != nil {
		t.Fatal(err)
	}
	if w := request(http.MethodPost, base+"/restart", "token"); w.Code != http.StatusBadRequest {
		t.Fatalf("restart should use current config: %d %s", w.Code, w.Body.String())
	}
}

func TestTerminalWebSocketAuthenticationAndScope(t *testing.T) {
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	wid, id := uuid.New(), uuid.New()
	root := t.TempDir()
	workspaces := workspaceStore{wid: {ID: wid, RootPath: root}}
	configs := configStore{id: {ID: id, WorkspaceID: wid, Key: "child"}}
	writeConfig(t, root, "child", "env DEVORBIT_HTTP_CHILD=1 "+self+" -test.run=^TestHTTPChild")
	supervisor := process.New(50 * time.Millisecond)
	t.Cleanup(func() { _ = supervisor.Shutdown(context.Background()) })
	c := New(service.New(configService.New(configs, workspaces), workspaceService.New(workspaces, ""), supervisor))
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Get("/services/{id}/terminal/{execution}", c.Terminal)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	cfg := process.Config{WorkspaceID: wid, Command: self, Args: []string{"-test.run=^TestHTTPChild$"}, Env: map[string]string{"DEVORBIT_HTTP_CHILD": "1"}, Dir: workspaces[wid].RootPath}
	status, err := supervisor.Start(id, cfg)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/services/" + id.String() + "/terminal/" + status.ExecutionID.String()
	dial := func(raw string, headers http.Header) (*websocket.Conn, int) {
		t.Helper()
		conn, response, err := websocket.DefaultDialer.Dial(raw, headers)
		if err != nil && response == nil {
			t.Fatal(err)
		}
		if response != nil {
			defer response.Body.Close()
			return conn, response.StatusCode
		}
		return conn, http.StatusSwitchingProtocols
	}
	if conn, code := dial(endpoint, nil); conn != nil || code != http.StatusUnauthorized {
		t.Fatalf("unauthorized: %d", code)
	}
	if conn, code := dial(endpoint, http.Header{"Authorization": {"Bearer token"}, "Origin": {"https://other.example"}}); conn != nil || code != http.StatusForbidden {
		t.Fatalf("origin: %d", code)
	}
	headers := http.Header{"Authorization": {"Bearer token"}}
	conn, code := dial(endpoint, headers)
	if code != http.StatusSwitchingProtocols {
		t.Fatalf("upgrade: %d", code)
	}
	if err := conn.WriteJSON(map[string]any{"type": "resize", "cols": 80, "rows": 24}); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(map[string]any{"type": "close"}); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if supervisor.Status(id).State != process.Running {
		t.Fatal("closing a viewer stopped its service")
	}
	stale, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	stale.Path = strings.Replace(stale.Path, status.ExecutionID.String(), uuid.NewString(), 1)
	if conn, code := dial(stale.String(), headers); conn != nil || code != http.StatusConflict {
		t.Fatalf("stale execution: %d", code)
	}
}

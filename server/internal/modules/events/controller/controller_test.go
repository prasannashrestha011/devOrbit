package controller

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
)

type services map[uuid.UUID]*serviceModel.Identity

func (s services) Identity(_ context.Context, id uuid.UUID) (*serviceModel.Identity, error) {
	if item := s[id]; item != nil {
		return item, nil
	}
	return nil, serviceRepo.ErrNotFound
}

type workspaces map[uuid.UUID]*workspaceModel.Workspace

func (s workspaces) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
	if item := s[id]; item != nil {
		return item, nil
	}
	return nil, workspaceRepo.ErrNotFound
}

func TestAuthenticatedSSEFiltersAndWriteTimeout(t *testing.T) {
	b := events.New()
	defer b.Close()
	wid, sid := uuid.New(), uuid.New()
	c := New(b, services{sid: {ID: sid, WorkspaceID: wid}}, workspaces{wid: {ID: wid}})
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "token", "http://127.0.0.1:7456", func(context.Context) error { return nil }, func(r chi.Router) {
		r.Get("/events", c.Stream)
	})
	server := httptest.NewUnstartedServer(handler)
	server.Config.WriteTimeout = 100 * time.Millisecond
	server.Start()
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	request := func(path, token string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	for _, tc := range []struct {
		path, token string
		status      int
	}{
		{"/api/v1/events", "", 401},
		{"/api/v1/events?workspace_id=bad", "token", 400},
		{"/api/v1/events?service_id=" + sid.String() + "&service_id=" + sid.String(), "token", 400},
		{"/api/v1/events?unknown=1", "token", 400},
		{"/api/v1/events?service_id=" + uuid.NewString(), "token", 404},
		{"/api/v1/events?workspace_id=" + uuid.NewString(), "token", 404},
		{"/api/v1/events?workspace_id=" + wid.String() + "&service_id=" + sid.String(), "token", 200},
	} {
		resp := request(tc.path, tc.token)
		if resp.StatusCode != tc.status {
			t.Fatalf("%s: %d", tc.path, resp.StatusCode)
		}
		resp.Body.Close()
	}
	resp := request("/api/v1/events?service_id="+sid.String(), "token")
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream response: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	reader := bufio.NewReader(resp.Body)
	if line, err := reader.ReadString('\n'); err != nil || line != ": connected\n" {
		t.Fatalf("initial flush: %q, %v", line, err)
	}
	_, _ = reader.ReadString('\n')
	time.Sleep(150 * time.Millisecond)
	b.Publish(events.Event{Type: events.ProcessStarted, WorkspaceID: wid, ServiceID: uuid.New()})
	b.Publish(events.Event{Type: events.ProcessStarted, WorkspaceID: wid, ServiceID: sid, Data: map[string]any{"state": "running"}})
	var frame string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		frame += line
		if line == "\n" {
			break
		}
	}
	if !strings.HasPrefix(frame, "id: ") || !strings.Contains(frame, "event: process.started\n") || !strings.Contains(frame, `"service_id":"`+sid.String()+`"`) {
		t.Fatalf("unexpected SSE frame: %s", frame)
	}
}

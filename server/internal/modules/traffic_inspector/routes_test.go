package traffic_inspector

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

type workspaceStub struct{ id uuid.UUID }

func (s workspaceStub) Get(_ context.Context, id uuid.UUID) (*workspaceModel.Workspace, error) {
	if id != s.id {
		return nil, ErrNotFound
	}
	return &workspaceModel.Workspace{ID: id}, nil
}

type serviceStub struct{ item *serviceModel.Service }

func (s serviceStub) Get(_ context.Context, id uuid.UUID) (*serviceModel.Service, error) {
	if id != s.item.ID {
		return nil, ErrNotFound
	}
	return s.item, nil
}

type processStub struct{ status process.Status }

func (s processStub) Status(uuid.UUID) process.Status { return s.status }

func TestStartRouteAcceptsDesktopPayloadAndScopesSessions(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("listening port detection is unsupported on this platform")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	defer upstream.Close()
	port, _ := strconv.Atoi(strings.TrimPrefix(upstream.URL, "http://127.0.0.1:"))
	portKey := strconv.Itoa(port)
	pid := os.Getpid()
	workspaceID, serviceID := uuid.New(), uuid.New()
	manager := newTestManager(t, nil)
	defer manager.Close(context.Background())
	controller := &Controller{Manager: manager, Resolver: Resolver{
		Workspaces: workspaceStub{workspaceID},
		Services:   serviceStub{&serviceModel.Service{ID: serviceID, WorkspaceID: workspaceID}},
		Processes:  processStub{process.Status{State: process.Running, PID: &pid}},
	}}
	handler := api.Router(slog.New(slog.NewTextHandler(io.Discard, nil)), "test-token", "http://127.0.0.1:9900", func(context.Context) error { return nil }, controller.Register)
	path := "/api/v1/workspaces/" + workspaceID.String() + "/traffic"
	request := func(method, path, body string, authorized bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if authorized {
			req.Header.Set("Authorization", "Bearer test-token")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	if got := request("GET", path, "", false).Code; got != 401 {
		t.Fatalf("unauthenticated status = %d", got)
	}
	if got := request("POST", path, `{"kind":"local","service_key":"`+serviceID.String()+`","port_key":"`+portKey+`","upstream":"http://evil"}`, true).Code; got != 400 {
		t.Fatalf("arbitrary upstream accepted: %d", got)
	}
	created := request("POST", path, `{"kind":"local","service_key":"`+serviceID.String()+`","port_key":"`+portKey+`"}`, true)
	if created.Code != 201 {
		t.Fatalf("start status = %d: %s", created.Code, created.Body.String())
	}
	var info SessionInfo
	if err := json.Unmarshal(created.Body.Bytes(), &info); err != nil || info.Target.ServiceKey != serviceID.String() || info.Target.PortKey != portKey {
		t.Fatalf("decoded session = %+v, %v", info, err)
	}
	if got := request("POST", path, `{"kind":"local","service_key":"`+serviceID.String()+`","port_key":"`+portKey+`"}`, true).Code; got != 409 {
		t.Fatalf("duplicate status = %d", got)
	}
	historyPath := path + "/" + info.ID.String() + "/records"
	checkHistory := func(wantCount int, wantSequence float64) {
		t.Helper()
		response := request("GET", historyPath, "", true)
		if response.Code != 200 {
			t.Fatalf("history status = %d: %s", response.Code, response.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		records, ok := payload["records"].([]any)
		if !ok || len(records) != wantCount || payload["latest_sequence"] != wantSequence || payload["gap"] != false {
			t.Fatalf("unexpected history response: %s", response.Body.String())
		}
	}
	checkHistory(0, 0)
	appendTestRecord(t, manager, manager.sessions[info.ID], Record{ID: uuid.New(), SessionID: info.ID})
	checkHistory(1, 1)
	if got := request("DELETE", historyPath, "", true).Code; got != 204 {
		t.Fatalf("clear history status = %d", got)
	}
	checkHistory(0, 1)
	other := "/api/v1/workspaces/" + uuid.NewString() + "/traffic/" + info.ID.String() + "/records"
	if got := request("GET", other, "", true).Code; got != 404 {
		t.Fatalf("cross-workspace history status = %d", got)
	}
	stop := path + "/" + info.ID.String()
	if got := request("DELETE", stop, "", true).Code; got != 204 {
		t.Fatalf("stop status = %d", got)
	}
	if got := request("DELETE", stop, "", true).Code; got != 204 {
		t.Fatalf("repeated stop status = %d", got)
	}
}

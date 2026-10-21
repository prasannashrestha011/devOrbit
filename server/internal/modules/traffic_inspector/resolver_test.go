package traffic_inspector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/dockercompose"
	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

func TestResolveLocalPortSelection(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/" {
			t.Errorf("probe must use HEAD / without following redirects: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Location", "/login")
		w.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(address.Port())
	workspaceID, serviceID := uuid.New(), uuid.New()

	for _, test := range []struct {
		name    string
		ports   map[string]int
		portKey string
		apiHost string
		wantKey string
		wantErr string
	}{
		{name: "automatic HTTP port", ports: map[string]int{"http": port, "db": 5432}, wantKey: "http"},
		{name: "explicit port", ports: map[string]int{"http": port, "api": port}, portKey: "api", wantKey: "api"},
		{name: "ambiguous ports", ports: map[string]int{"http": port, "api": port}, wantErr: "multiple HTTP ports; select one"},
		{name: "unknown port", ports: map[string]int{"http": port}, portKey: "web", wantErr: `HTTP port "web" not found`},
		{name: "non HTTP port", ports: map[string]int{"db": 5432}, wantErr: "no HTTP port available"},
		{name: "invalid port", ports: map[string]int{"http": 65536}, wantErr: "invalid HTTP port"},
		{name: "DevOrbit API", ports: map[string]int{"http": port}, apiHost: address.Host, wantErr: "cannot inspect the DevOrbit API"},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := Resolver{
				Workspaces: workspaceStub{workspaceID},
				Services: serviceStub{&serviceModel.Service{
					ID: serviceID, WorkspaceID: workspaceID, Ports: test.ports,
				}},
				Processes: processStub{},
				APIHost:   test.apiHost,
			}
			target := Target{WorkspaceID: workspaceID, Kind: "local", ServiceKey: serviceID.String(), PortKey: test.portKey, File: "ignored.yaml"}
			resolved, gotPort, err := resolver.Resolve(context.Background(), target)
			if test.wantErr != "" {
				if err == nil || err.Error() != test.wantErr || resolved != (Target{}) || gotPort != 0 {
					t.Fatalf("Resolve = %+v, %d, %v; want error %q", resolved, gotPort, err, test.wantErr)
				}
				return
			}
			target.PortKey = test.wantKey
			target.File = ""
			if err != nil || resolved != target || gotPort != port {
				t.Fatalf("Resolve = %+v, %d, %v; want %+v, %d", resolved, gotPort, err, target, port)
			}
		})
	}
}

type resolverWorkspace struct{ workspaceModel.Workspace }

func (s resolverWorkspace) Get(context.Context, uuid.UUID) (*workspaceModel.Workspace, error) {
	return &s.Workspace, nil
}

type resolverDocker struct{ project dockercompose.Project }

func (s resolverDocker) Inspect(context.Context, string, dockercompose.Candidate) (*dockercompose.Project, error) {
	return &s.project, nil
}

func (resolverDocker) Status(context.Context, string, *dockercompose.Project) error { return nil }

func TestResolveDockerPublishedPorts(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(address.Port())
	root := t.TempDir()
	if _, err := workspaceconfig.Write(root, []byte("services: {}\n"), ""); err != nil {
		t.Fatal(err)
	}
	composeFile := filepath.Join(root, "compose.yaml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	workspaceID := uuid.New()

	for _, test := range []struct {
		name    string
		state   string
		binding dockercompose.Port
		wantErr string
	}{
		{name: "loopback TCP", state: "running", binding: dockercompose.Port{Published: address.Port(), Protocol: "tcp", Host: "127.0.0.1"}},
		{name: "wildcard TCP", state: "running", binding: dockercompose.Port{Published: address.Port(), Protocol: "tcp", Host: "0.0.0.0"}},
		{name: "stopped container", state: "exited", binding: dockercompose.Port{Published: address.Port(), Protocol: "tcp"}, wantErr: "no HTTP port available"},
		{name: "remote binding", state: "running", binding: dockercompose.Port{Published: address.Port(), Protocol: "tcp", Host: "192.0.2.1"}, wantErr: "no HTTP port available"},
		{name: "UDP binding", state: "running", binding: dockercompose.Port{Published: address.Port(), Protocol: "udp"}, wantErr: "no HTTP port available"},
		{name: "unpublished port", state: "running", binding: dockercompose.Port{Target: port, Protocol: "tcp"}, wantErr: "no HTTP port available"},
		{name: "invalid published port", state: "running", binding: dockercompose.Port{Published: "65536", Protocol: "tcp"}, wantErr: "no HTTP port available"},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := Resolver{
				Workspaces: resolverWorkspace{workspaceModel.Workspace{ID: workspaceID, RootPath: root}},
				Docker: resolverDocker{dockercompose.Project{Services: []dockercompose.Service{
					{Name: "other", Containers: []dockercompose.Container{{State: "running", Ports: []dockercompose.Port{{Published: "9000", Protocol: "tcp"}}}}},
					{Name: "api", Containers: []dockercompose.Container{{State: test.state, Ports: []dockercompose.Port{test.binding}}}},
				}}},
			}
			target := Target{WorkspaceID: workspaceID, Kind: "docker", ServiceKey: "api"}
			resolved, gotPort, err := resolver.Resolve(context.Background(), target)
			if test.wantErr != "" {
				if err == nil || err.Error() != test.wantErr || resolved != (Target{}) || gotPort != 0 {
					t.Fatalf("Resolve = %+v, %d, %v; want error %q", resolved, gotPort, err, test.wantErr)
				}
				return
			}
			target.PortKey = address.Port()
			target.File = composeFile
			if err != nil || resolved != target || gotPort != port {
				t.Fatalf("Resolve = %+v, %d, %v; want %+v, %d", resolved, gotPort, err, target, port)
			}
		})
	}
}

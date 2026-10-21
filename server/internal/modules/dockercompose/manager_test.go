package dockercompose

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type recordedCommand struct {
	dir  string
	args []string
}

type stubRunner struct {
	calls []recordedCommand
	reply func(context.Context, []string) ([]byte, error)
}

func (r *stubRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, recordedCommand{dir, slices.Clone(args)})
	return r.reply(ctx, args)
}

func TestDiscoverAndSelect(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"compose.yaml", "compose.override.yml", "docker-compose.yml"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("services: {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	candidates, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{
		{File: filepath.Join(root, "compose.yaml"), Files: []string{filepath.Join(root, "compose.yaml"), filepath.Join(root, "compose.override.yml")}, Directory: root},
		{File: filepath.Join(root, "docker-compose.yml"), Files: []string{filepath.Join(root, "docker-compose.yml")}, Directory: root},
	}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates = %#v, want %#v", candidates, want)
	}
	if _, err := Select(candidates, ""); err == nil {
		t.Fatal("multiple configurations must require selection")
	}
	if got, err := Select(candidates, filepath.Join(root, "docker-compose.yml")); err != nil || got.File != filepath.Join(root, "docker-compose.yml") {
		t.Fatalf("selection = %+v, %v", got, err)
	}
	if _, err := Select(candidates, "../compose.yaml"); err == nil {
		t.Fatal("path traversal was accepted")
	}
	if _, err := Select(nil, ""); err == nil {
		t.Fatal("missing configuration was accepted")
	}
	if err := os.Remove(filepath.Join(root, "docker-compose.yml")); err != nil {
		t.Fatal(err)
	}
	if got, err := Select(candidates[:1], ""); err != nil || got.File != filepath.Join(root, "compose.yaml") {
		t.Fatalf("single selection = %+v, %v", got, err)
	}
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	if err := os.WriteFile(outside, []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "compose.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "compose.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(root); err == nil {
		t.Fatal("Compose symlink outside workspace was accepted")
	}
}

func TestDiscoverWorkspaceUsesConfiguredServiceDirectory(t *testing.T) {
	root, project := t.TempDir(), t.TempDir()
	if _, err := workspaceconfig.Write(root, []byte("services:\n  api:\n    command: go run main.go\n    cwd: "+project+"\n"), ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "docker-compose.yaml"), []byte("services: {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	candidates, err := DiscoverWorkspace(root)
	if err != nil || len(candidates) != 1 || candidates[0].File != filepath.Join(project, "docker-compose.yaml") || candidates[0].Directory != project {
		t.Fatalf("project directory discovery = %+v, %v", candidates, err)
	}
}

func TestInspectUsesComposeConfiguration(t *testing.T) {
	root := t.TempDir()
	runner := &stubRunner{reply: func(_ context.Context, args []string) ([]byte, error) {
		return []byte(`{"name":"effective-project","services":{"db":{"image":"postgres:16","ports":[{"target":5432,"published":"15432","protocol":"tcp"}]},"api":{"build":{"context":"."}}}}`), nil
	}}
	manager := &Manager{Runner: runner}
	project, err := manager.Inspect(context.Background(), root, Candidate{File: filepath.Join(root, "compose.yaml"), Files: []string{filepath.Join(root, "compose.yaml"), filepath.Join(root, "compose.override.yml")}, Directory: root})
	if err != nil {
		t.Fatal(err)
	}
	if project.Name != "effective-project" || len(project.Services) != 2 || project.Services[0].Name != "api" || !project.Services[0].Build || project.Services[1].Ports[0].Published != "15432" {
		t.Fatalf("project = %+v", project)
	}
	want := []string{"compose", "--project-directory", root, "-f", filepath.Join(root, "compose.yaml"), "-f", filepath.Join(root, "compose.override.yml"), "config", "--format", "json"}
	if len(runner.calls) != 1 || runner.calls[0].dir != root || !reflect.DeepEqual(runner.calls[0].args, want) {
		t.Fatalf("Compose command = %+v, want %v", runner.calls, want)
	}
}

func TestStatusSupportsArrayAndLinesWithReplicas(t *testing.T) {
	root := t.TempDir()
	rows := []map[string]any{
		{"ID": "one", "Name": "demo-api-1", "Image": "api:dev", "Project": "demo", "Service": "api", "State": "running", "Health": "healthy", "Publishers": []map[string]any{{"TargetPort": 3000, "PublishedPort": 13000, "Protocol": "tcp"}}},
		{"ID": "two", "Name": "demo-api-2", "Image": "api:dev", "Project": "demo", "Service": "api", "State": "exited", "Publishers": []any{}},
	}
	for _, format := range []string{"array", "lines"} {
		t.Run(format, func(t *testing.T) {
			first, _ := json.Marshal(rows[0])
			second, _ := json.Marshal(rows[1])
			output := append(append(slices.Clone(first), '\n'), second...)
			if format == "array" {
				output, _ = json.Marshal(rows)
			}
			runner := &stubRunner{reply: func(_ context.Context, args []string) ([]byte, error) {
				if len(args) > 1 && args[0] == "container" {
					return nil, nil
				}
				return output, nil
			}}
			project := &Project{File: filepath.Join(root, "compose.yaml"), Files: []string{filepath.Join(root, "compose.yaml")}, Directory: root, Name: "demo", Services: []Service{{Name: "api"}}}
			if err := (&Manager{Runner: runner}).Status(context.Background(), root, project); err != nil {
				t.Fatal(err)
			}
			if project.Created != 2 || project.Running != 1 || project.Stopped != 1 || len(project.Services[0].Containers) != 2 || project.Services[0].Containers[0].Health != "healthy" || project.Services[0].Containers[0].Ports[0].Published != "13000" {
				t.Fatalf("status = %+v", project)
			}
		})
	}
}

func TestStatusUsesEngineHealthAndPortBindings(t *testing.T) {
	root := t.TempDir()
	runner := &stubRunner{reply: func(_ context.Context, args []string) ([]byte, error) {
		switch args[0] {
		case "container":
			return nil, nil
		case "compose":
			return []byte(`[{"ID":"one","Name":"demo-api-1","Project":"demo","Service":"api","State":"restarting"}]`), nil
		case "inspect":
			return []byte(`{"state":{"Status":"running","Health":{"Status":"unhealthy","Log":[{"Output":"connection refused"}]}},"restart_count":3,"ports":{"3000/tcp":[{"HostIp":"127.0.0.1","HostPort":"13000"}],"9000/tcp":null}}`), nil
		default:
			t.Fatalf("unexpected Docker call: %v", args)
			return nil, nil
		}
	}}
	project := &Project{File: filepath.Join(root, "compose.yaml"), Files: []string{filepath.Join(root, "compose.yaml")}, Directory: root, Name: "demo", Services: []Service{{Name: "api"}}}
	if err := (&Manager{Runner: runner}).Status(context.Background(), root, project); err != nil {
		t.Fatal(err)
	}
	container := project.Services[0].Containers[0]
	if container.State != "running" || container.Health != "unhealthy" || container.RestartCount != 3 || container.HealthError != "connection refused" || project.Running != 1 || len(container.Ports) != 2 || container.Ports[0].Host != "127.0.0.1" || container.Ports[0].Published != "13000" || container.Ports[1].Published != "" {
		t.Fatalf("engine details = %+v, project = %+v", container, project)
	}
}

func TestRunScopesActionsAndRejectsInvalidService(t *testing.T) {
	root := t.TempDir()
	project := &Project{File: filepath.Join(root, "compose.yaml"), Files: []string{filepath.Join(root, "compose.yaml")}, Directory: root, Name: "demo", Services: []Service{{Name: "api", Build: true}, {Name: "-bad", Build: true}}}
	runner := &stubRunner{reply: func(_ context.Context, _ []string) ([]byte, error) { return nil, nil }}
	manager := &Manager{Runner: runner}
	for _, tc := range []struct {
		action, service string
		command         []string
	}{
		{"up", "", []string{"up", "-d"}},
		{"up", "api", []string{"up", "-d", "api"}},
		{"start", "api", []string{"start", "api"}},
		{"stop", "api", []string{"stop", "api"}},
		{"restart", "api", []string{"restart", "api"}},
		{"build", "api", []string{"build", "api"}},
		{"down", "", []string{"down"}},
	} {
		if err := manager.Run(context.Background(), root, project, tc.action, tc.service); err != nil {
			t.Fatalf("%s %s: %v", tc.action, tc.service, err)
		}
		got := runner.calls[len(runner.calls)-1]
		want := append([]string{"compose", "--project-directory", root, "-f", filepath.Join(root, "compose.yaml")}, tc.command...)
		if got.dir != root || !reflect.DeepEqual(got.args, want) || slices.Contains(got.args, "-v") {
			t.Fatalf("%s %s command = %+v, want %v", tc.action, tc.service, got, want)
		}
	}
	before := len(runner.calls)
	for _, tc := range []struct{ action, service string }{{"stop", "unknown"}, {"stop", "-bad"}, {"down", "api"}, {"build", "unknown"}, {"sh", ""}} {
		if err := manager.Run(context.Background(), root, project, tc.action, tc.service); err == nil {
			t.Fatalf("%s %s was accepted", tc.action, tc.service)
		}
	}
	if len(runner.calls) != before {
		t.Fatalf("unexpected commands after invalid actions: %+v", runner.calls[before:])
	}
}

func TestRunPropagatesCancellationAndErrors(t *testing.T) {
	root := t.TempDir()
	project := &Project{File: filepath.Join(root, "compose.yaml"), Files: []string{filepath.Join(root, "compose.yaml")}, Directory: root, Name: "demo"}
	ctx, cancel := context.WithCancel(context.Background())
	runner := &stubRunner{reply: func(ctx context.Context, args []string) ([]byte, error) {
		if args[0] == "container" {
			return nil, nil
		}
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	if err := (&Manager{Runner: runner}).Run(ctx, root, project, "up", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	runner.reply = func(_ context.Context, args []string) ([]byte, error) {
		if args[0] == "container" {
			return nil, nil
		}
		return nil, errors.New("port is already allocated")
	}
	if err := (&Manager{Runner: runner}).Run(context.Background(), root, project, "up", ""); err == nil || !strings.Contains(err.Error(), "port is already allocated") {
		t.Fatalf("operation error = %v", err)
	}
}

func TestStatusRejectsProjectNameCollision(t *testing.T) {
	root := t.TempDir()
	project := &Project{File: filepath.Join(root, "compose.yaml"), Files: []string{filepath.Join(root, "compose.yaml")}, Directory: root, Name: "shared"}
	runner := &stubRunner{reply: func(_ context.Context, args []string) ([]byte, error) {
		if args[0] == "container" {
			return []byte("container-id\n"), nil
		}
		if args[0] == "inspect" {
			return []byte(`{"com.docker.compose.project.working_dir":"/another-workspace","com.docker.compose.project.config_files":"/another-workspace/compose.yaml"}`), nil
		}
		t.Fatalf("collision reached Compose: %v", args)
		return nil, nil
	}}
	manager := &Manager{Runner: runner}
	if err := manager.Status(context.Background(), root, project); err == nil || !strings.Contains(err.Error(), "another directory") {
		t.Fatalf("status collision = %v", err)
	}
	if err := manager.Run(context.Background(), root, project, "down", ""); err == nil || !strings.Contains(err.Error(), "another directory") {
		t.Fatalf("down collision = %v", err)
	}
}

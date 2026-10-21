package dockercompose

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

var bases = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type commandRunner struct{}

type cappedBuffer struct{ bytes.Buffer }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, errors.New("Docker output exceeded 1 MiB")
	}
	return b.Buffer.Write(p)
}

func (commandRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	var stdout, stderr cappedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("docker %s: %s: %w", strings.Join(args, " "), message, err)
	}
	return stdout.Bytes(), nil
}

type Manager struct{ Runner Runner }

func New() *Manager { return &Manager{Runner: commandRunner{}} }

type Availability struct {
	State   string `json:"state"`
	Message string `json:"message"`
}

func (m *Manager) Availability(ctx context.Context, root string) Availability {
	checks := []struct {
		args           []string
		state, message string
	}{
		{[]string{"--version"}, "missing", "Docker CLI is not installed or not on PATH."},
		{[]string{"compose", "version"}, "compose_unavailable", "Docker Compose V2 is unavailable."},
		{[]string{"info", "--format", "{{.ServerVersion}}"}, "engine_unavailable", "Docker Engine is stopped or inaccessible. Check Docker permissions and start Docker."},
	}
	for _, check := range checks {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := m.Runner.Run(checkCtx, root, check.args...)
		cancel()
		if err != nil {
			if check.state == "missing" && !errors.Is(err, exec.ErrNotFound) {
				return Availability{"missing", check.message + " " + err.Error()}
			}
			return Availability{check.state, check.message + " " + err.Error()}
		}
	}
	return Availability{"ready", "Docker Compose and Docker Engine are ready."}
}

type Candidate struct {
	File      string   `json:"file"`
	Files     []string `json:"files"`
	Directory string   `json:"directory"`
}

func DiscoverWorkspace(root string) ([]Candidate, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	snapshot, err := workspaceconfig.Read(root)
	if err != nil {
		return nil, err
	}
	directories := []string{root}
	seen := map[string]bool{root: true}
	for _, service := range snapshot.Config.Services {
		directory, err := workspaceconfig.ResolveDirectory(root, service.CWD)
		if err != nil {
			return nil, err
		}
		if !seen[directory] {
			directories = append(directories, directory)
			seen[directory] = true
		}
	}
	slices.Sort(directories[1:])
	result := []Candidate{}
	for _, directory := range directories {
		candidates, err := Discover(directory)
		if err != nil {
			return nil, err
		}
		result = append(result, candidates...)
	}
	return result, nil
}

func Discover(root string) ([]Candidate, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	result := []Candidate{}
	for _, name := range bases {
		if ok, err := regularFile(root, name); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		candidate := Candidate{File: filepath.Join(root, name), Files: []string{filepath.Join(root, name)}, Directory: root}
		prefix := strings.TrimSuffix(name, filepath.Ext(name))
		for _, suffix := range []string{".override.yaml", ".override.yml"} {
			override := prefix + suffix
			if ok, err := regularFile(root, override); err != nil {
				return nil, err
			} else if ok {
				candidate.Files = append(candidate.Files, filepath.Join(root, override))
			}
		}
		result = append(result, candidate)
	}
	return result, nil
}

func regularFile(root, name string) (bool, error) {
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return false, err
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false, errors.New("Compose file resolves outside workspace")
		}
		info, err = os.Stat(path)
		if err != nil {
			return false, err
		}
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("Compose path %s is not a regular file", name)
	}
	return true, nil
}

func Select(candidates []Candidate, file string) (Candidate, error) {
	if file == "" && len(candidates) == 1 {
		return candidates[0], nil
	}
	if file == "" && len(candidates) == 0 {
		return Candidate{}, errors.New("no Compose file found in workspace root")
	}
	if file == "" {
		return Candidate{}, errors.New("select a Compose file")
	}
	for _, candidate := range candidates {
		if candidate.File == file {
			return candidate, nil
		}
	}
	return Candidate{}, errors.New("unknown Compose file")
}

type Port struct {
	Target    int    `json:"target"`
	Published string `json:"published"`
	Protocol  string `json:"protocol"`
	Host      string `json:"host,omitempty"`
}
type Service struct {
	Name       string      `json:"name"`
	Image      string      `json:"image"`
	Build      bool        `json:"build"`
	Ports      []Port      `json:"ports"`
	Containers []Container `json:"containers"`
}
type Container struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	State        string `json:"state"`
	Health       string `json:"health"`
	RestartCount int    `json:"restart_count"`
	HealthError  string `json:"health_error,omitempty"`
	Error        string `json:"error,omitempty"`
	Ports        []Port `json:"ports"`
}
type Project struct {
	File      string    `json:"file"`
	Files     []string  `json:"files"`
	Directory string    `json:"directory"`
	Name      string    `json:"name"`
	Services  []Service `json:"services"`
	Created   int       `json:"created"`
	Running   int       `json:"running"`
	Stopped   int       `json:"stopped"`
}
type Snapshot struct {
	Availability Availability `json:"availability"`
	Candidates   []Candidate  `json:"candidates"`
	Project      *Project     `json:"project,omitempty"`
	Error        string       `json:"error,omitempty"`
}

func composeArgs(root string, candidate Candidate) []string {
	args := []string{"compose", "--project-directory", root}
	for _, file := range candidate.Files {
		args = append(args, "-f", file)
	}
	return args
}

func (m *Manager) Inspect(ctx context.Context, root string, candidate Candidate) (*Project, error) {
	args := append(composeArgs(root, candidate), "config", "--format", "json")
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := m.Runner.Run(callCtx, root, args...)
	if err != nil {
		return nil, err
	}
	var config struct {
		Name     string `json:"name"`
		Services map[string]struct {
			Image string          `json:"image"`
			Build json.RawMessage `json:"build"`
			Ports []Port          `json:"ports"`
		} `json:"services"`
	}
	if err := json.Unmarshal(output, &config); err != nil {
		return nil, fmt.Errorf("invalid Compose configuration output: %w", err)
	}
	if config.Name == "" {
		return nil, errors.New("Compose project name is empty")
	}
	project := &Project{File: candidate.File, Files: candidate.Files, Directory: root, Name: config.Name, Services: []Service{}}
	for name, item := range config.Services {
		project.Services = append(project.Services, Service{Name: name, Image: item.Image, Build: len(item.Build) > 0 && string(item.Build) != "null", Ports: item.Ports, Containers: []Container{}})
	}
	slices.SortFunc(project.Services, func(a, b Service) int { return strings.Compare(a.Name, b.Name) })
	return project, nil
}

func (m *Manager) checkIdentity(ctx context.Context, root string, project *Project) error {
	output, err := m.Runner.Run(ctx, root, "container", "ls", "-a", "--filter", "label=com.docker.compose.project="+project.Name, "--format", "{{.ID}}")
	if err != nil {
		return err
	}
	ids := strings.Fields(string(output))
	if len(ids) == 0 {
		return nil
	}
	args := append([]string{"inspect", "--format", "{{json .Config.Labels}}"}, ids...)
	output, err = m.Runner.Run(ctx, root, args...)
	if err != nil {
		return err
	}
	expected := make([]string, len(project.Files))
	copy(expected, project.Files)
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
		var labels map[string]string
		if err := json.Unmarshal(line, &labels); err != nil {
			return fmt.Errorf("invalid Docker labels: %w", err)
		}
		if labels["com.docker.compose.project.working_dir"] != root || labels["com.docker.compose.project.config_files"] != strings.Join(expected, ",") {
			return fmt.Errorf("Compose project %q already has containers from another directory or Compose file selection", project.Name)
		}
	}
	return nil
}

func (m *Manager) Status(ctx context.Context, root string, project *Project) error {
	if err := m.checkIdentity(ctx, root, project); err != nil {
		return err
	}
	candidate := Candidate{File: project.File, Files: project.Files}
	args := append(composeArgs(root, candidate), "ps", "-a", "--format", "json")
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := m.Runner.Run(callCtx, root, args...)
	if err != nil {
		return err
	}
	var rows []json.RawMessage
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return nil
	}
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return err
		}
	} else {
		for _, line := range bytes.Split(trimmed, []byte("\n")) {
			rows = append(rows, line)
		}
	}
	for _, row := range rows {
		var item struct {
			ID, Name, Image, Project, Service, State, Health string
			Publishers                                       []struct {
				TargetPort, PublishedPort int
				Protocol, URL             string
			} `json:"Publishers"`
		}
		if err := json.Unmarshal(row, &item); err != nil {
			return fmt.Errorf("invalid Compose status: %w", err)
		}
		if item.Project != project.Name {
			return errors.New("Compose returned a container from another project")
		}
		for i := range project.Services {
			if project.Services[i].Name != item.Service {
				continue
			}
			container := Container{ID: item.ID, Name: item.Name, Image: item.Image, State: item.State, Health: item.Health, Ports: []Port{}}
			for _, port := range item.Publishers {
				if port.PublishedPort > 0 {
					container.Ports = append(container.Ports, Port{Target: port.TargetPort, Published: fmt.Sprint(port.PublishedPort), Protocol: port.Protocol, Host: port.URL})
				}
			}
			project.Services[i].Containers = append(project.Services[i].Containers, container)
			project.Created++
			break
		}
	}
	for i := range project.Services {
		for j := range project.Services[i].Containers {
			container := &project.Services[i].Containers[j]
			if container.ID == "" {
				continue
			}
			// Docker inspect supplies health diagnostics and exposed ports omitted by Compose ps.
			output, err := m.Runner.Run(callCtx, root, "inspect", "--format", `{"state":{{json .State}},"restart_count":{{.RestartCount}},"ports":{{json .NetworkSettings.Ports}}}`, container.ID)
			if err != nil {
				container.Error = "Container disappeared or Docker inspect failed. Refresh to retry."
				continue
			}
			var detail struct {
				State struct {
					Status string `json:"Status"`
					Error  string `json:"Error"`
					Health struct {
						Status string `json:"Status"`
						Log    []struct {
							Output string `json:"Output"`
						} `json:"Log"`
					} `json:"Health"`
				} `json:"state"`
				RestartCount int `json:"restart_count"`
				Ports        map[string][]struct {
					HostIP   string `json:"HostIp"`
					HostPort string `json:"HostPort"`
				} `json:"ports"`
			}
			if err := json.Unmarshal(bytes.TrimSpace(output), &detail); err != nil {
				container.Error = "Docker returned invalid container details. Refresh to retry."
				continue
			}
			if detail.State.Status != "" {
				container.State = detail.State.Status
			}
			if detail.State.Health.Status != "" {
				container.Health = detail.State.Health.Status
			}
			container.RestartCount = detail.RestartCount
			container.Error = detail.State.Error
			if logs := detail.State.Health.Log; len(logs) > 0 && container.Health == "unhealthy" {
				container.HealthError = strings.TrimSpace(logs[len(logs)-1].Output)
			}
			if detail.Ports != nil {
				container.Ports = []Port{}
			}
			for _, target := range slices.Sorted(maps.Keys(detail.Ports)) {
				bindings := detail.Ports[target]
				parts := strings.SplitN(target, "/", 2)
				port, err := strconv.Atoi(parts[0])
				if err != nil {
					continue
				}
				protocol := "tcp"
				if len(parts) == 2 {
					protocol = parts[1]
				}
				if len(bindings) == 0 {
					container.Ports = append(container.Ports, Port{Target: port, Protocol: protocol})
					continue
				}
				for _, binding := range bindings {
					container.Ports = append(container.Ports, Port{Target: port, Protocol: protocol, Published: binding.HostPort, Host: binding.HostIP})
				}
			}
		}
	}
	for _, service := range project.Services {
		for _, container := range service.Containers {
			if container.State == "running" {
				project.Running++
			} else {
				project.Stopped++
			}
		}
	}
	return nil
}

func (m *Manager) Run(ctx context.Context, root string, project *Project, action, service string) error {
	if service != "" {
		if strings.HasPrefix(service, "-") {
			return errors.New("unknown Compose service")
		}
		valid := false
		for _, item := range project.Services {
			if item.Name == service {
				valid = true
				if action == "build" && !item.Build {
					return errors.New("service has no build definition")
				}
				break
			}
		}
		if !valid {
			return errors.New("unknown Compose service")
		}
	}
	if action == "down" && service != "" {
		return errors.New("down only supports the whole project")
	}
	args := composeArgs(root, Candidate{File: project.File, Files: project.Files})
	switch action {
	case "up":
		args = append(args, "up", "-d")
	case "start", "stop", "restart", "down", "build":
		args = append(args, action)
	default:
		return errors.New("unsupported Docker action")
	}
	if service != "" {
		args = append(args, service)
	}
	if err := m.checkIdentity(ctx, root, project); err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	_, err := m.Runner.Run(callCtx, root, args...)
	return err
}

var _ io.Writer = (*cappedBuffer)(nil)

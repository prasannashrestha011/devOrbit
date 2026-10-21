package dockercompose

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/terminalstream"
)

type dockerTerminal struct {
	ID            uuid.UUID     `json:"id"`
	WorkspaceID   uuid.UUID     `json:"workspace_id"`
	ContainerID   string        `json:"container_id"`
	ContainerName string        `json:"container_name"`
	Shell         string        `json:"shell"`
	State         process.State `json:"state"`
}

func (c *Controller) CreateTerminal(w http.ResponseWriter, r *http.Request) {
	workspace, ok := c.workspace(w, r)
	if !ok {
		return
	}
	var input struct {
		File        string `json:"file"`
		ContainerID string `json:"container_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.ContainerID == "" {
		replyError(w, 400, "invalid_request", "Choose a container")
		return
	}
	root, err := filepath.EvalSymlinks(workspace.RootPath)
	if err != nil {
		replyError(w, 400, "invalid_workspace", err.Error())
		return
	}
	if available := c.Manager.Availability(r.Context(), root); available.State != "ready" {
		replyError(w, 503, available.State, available.Message)
		return
	}
	_, project, err := c.prepare(r.Context(), root, input.File)
	if err != nil {
		replyError(w, 400, "invalid_compose", err.Error())
		return
	}
	if err := c.Manager.Status(r.Context(), project.Directory, project); err != nil {
		replyError(w, 409, "project_unavailable", err.Error())
		return
	}
	var container *Container
	for _, service := range project.Services {
		for i := range service.Containers {
			if service.Containers[i].ID == input.ContainerID {
				container = &service.Containers[i]
				break
			}
		}
	}
	if container == nil || container.State != "running" {
		replyError(w, 409, "container_unavailable", "Choose a running container from this Compose project")
		return
	}
	probeCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	shell := ""
	for _, candidate := range []string{"sh", "bash"} {
		if _, err := c.Manager.Runner.Run(probeCtx, root, "exec", input.ContainerID, candidate, "-c", ":"); err == nil {
			shell = candidate
			break
		}
	}
	if shell == "" {
		replyError(w, 422, "shell_unavailable", "This container has no supported shell (sh or bash)")
		return
	}
	id := uuid.New()
	c.terminalMu.Lock()
	defer c.terminalMu.Unlock()
	count := 0
	for _, session := range c.terminals {
		if session.WorkspaceID == workspace.ID {
			count++
		}
	}
	if count >= 16 {
		replyError(w, 409, "terminal_limit", "Close an existing container terminal before opening another")
		return
	}
	status, err := c.terminalSupervisor.Start(id, process.Config{WorkspaceID: workspace.ID, Command: "docker", Args: []string{"exec", "-it", input.ContainerID, shell}, Dir: project.Directory, Env: map[string]string{"TERM": "xterm-256color"}, Interactive: true})
	if err != nil {
		replyError(w, 503, "terminal_unavailable", "Cannot start a terminal in this container")
		return
	}
	terminal := dockerTerminal{ID: id, WorkspaceID: workspace.ID, ContainerID: input.ContainerID, ContainerName: container.Name, Shell: shell, State: status.State}
	c.terminals[id] = terminal
	replyJSON(w, 201, terminal)
}

func (c *Controller) terminal(w http.ResponseWriter, r *http.Request) (dockerTerminal, bool) {
	workspace, ok := c.workspace(w, r)
	if !ok {
		return dockerTerminal{}, false
	}
	id, err := uuid.Parse(chi.URLParam(r, "terminalID"))
	if err != nil {
		replyError(w, 400, "invalid_request", "invalid terminal id")
		return dockerTerminal{}, false
	}
	c.terminalMu.Lock()
	terminal, found := c.terminals[id]
	c.terminalMu.Unlock()
	if !found || terminal.WorkspaceID != workspace.ID {
		replyError(w, 404, "not_found", "Docker terminal not found")
		return dockerTerminal{}, false
	}
	return terminal, true
}

func (c *Controller) StreamTerminal(w http.ResponseWriter, r *http.Request) {
	terminal, ok := c.terminal(w, r)
	if !ok {
		return
	}
	after, err := terminalstream.After(r)
	if err != nil {
		replyError(w, 400, "invalid_request", "invalid sequence")
		return
	}
	status := c.terminalSupervisor.Status(terminal.ID)
	stream, err := c.terminalSupervisor.Terminal(terminal.ID, terminal.WorkspaceID, status.ExecutionID)
	if err != nil {
		replyError(w, 409, "terminal_unavailable", "Container terminal has stopped")
		return
	}
	terminalstream.Serve(w, r, stream, after,
		func(data []byte) error {
			return c.terminalSupervisor.TerminalInput(terminal.ID, terminal.WorkspaceID, status.ExecutionID, data)
		},
		func(cols, rows uint16) error {
			return c.terminalSupervisor.TerminalResize(terminal.ID, terminal.WorkspaceID, status.ExecutionID, cols, rows)
		},
	)
}

func (c *Controller) DeleteTerminal(w http.ResponseWriter, r *http.Request) {
	terminal, ok := c.terminal(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err := c.terminalSupervisor.Stop(ctx, terminal.ID); err != nil {
		replyError(w, 503, "terminal_unavailable", "Cannot close container terminal")
		return
	}
	if err := c.terminalSupervisor.Forget(terminal.ID); err != nil {
		replyError(w, 503, "terminal_unavailable", "Cannot release container terminal")
		return
	}
	c.terminalMu.Lock()
	delete(c.terminals, terminal.ID)
	c.terminalMu.Unlock()
	w.WriteHeader(204)
}

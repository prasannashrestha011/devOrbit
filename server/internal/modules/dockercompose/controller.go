package dockercompose

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

type WorkspaceStore interface {
	Get(context.Context, uuid.UUID) (*model.Workspace, error)
}

type Operation struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Action      string    `json:"action"`
	State       string    `json:"state"`
	Error       string    `json:"error,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
	cancel      context.CancelFunc
}

type Controller struct {
	Manager            *Manager
	Store              WorkspaceStore
	mu                 sync.Mutex
	operations         map[string]*Operation
	terminalSupervisor *process.Supervisor
	terminalMu         sync.Mutex
	terminals          map[uuid.UUID]dockerTerminal
}

func NewController(store WorkspaceStore, supervisors ...*process.Supervisor) *Controller {
	supervisor := process.New(5 * time.Second)
	if len(supervisors) > 0 {
		supervisor = supervisors[0]
	}
	return &Controller{Manager: New(), Store: store, operations: map[string]*Operation{}, terminalSupervisor: supervisor, terminals: map[uuid.UUID]dockerTerminal{}}
}

func (c *Controller) workspace(w http.ResponseWriter, r *http.Request) (*model.Workspace, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		replyError(w, 400, "invalid_request", "invalid workspace id")
		return nil, false
	}
	workspace, err := c.Store.Get(r.Context(), id)
	if errors.Is(err, workspaceRepo.ErrNotFound) {
		replyError(w, 404, "not_found", "workspace not found")
		return nil, false
	}
	if err != nil {
		replyError(w, 500, "internal_error", "workspace lookup failed")
		return nil, false
	}
	return workspace, true
}

func (c *Controller) prepare(ctx context.Context, root, file string) (Candidate, *Project, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Candidate{}, nil, err
	}
	candidates, err := DiscoverWorkspace(root)
	if err != nil {
		return Candidate{}, nil, err
	}
	candidate, err := Select(candidates, file)
	if err != nil {
		return Candidate{}, nil, err
	}
	project, err := c.Manager.Inspect(ctx, candidate.Directory, candidate)
	return candidate, project, err
}

func (c *Controller) Snapshot(w http.ResponseWriter, r *http.Request) {
	workspace, ok := c.workspace(w, r)
	if !ok {
		return
	}
	root, err := filepath.EvalSymlinks(workspace.RootPath)
	if err != nil {
		replyError(w, 400, "invalid_workspace", err.Error())
		return
	}
	candidates, err := DiscoverWorkspace(root)
	if err != nil {
		replyError(w, 400, "invalid_compose_path", err.Error())
		return
	}
	snapshot := Snapshot{Candidates: candidates, Availability: c.Manager.Availability(r.Context(), root)}
	file := r.URL.Query().Get("file")
	if len(candidates) > 0 && (snapshot.Availability.State == "ready" || snapshot.Availability.State == "engine_unavailable") {
		candidate, selectErr := Select(candidates, file)
		if selectErr != nil {
			snapshot.Error = selectErr.Error()
		} else {
			project, inspectErr := c.Manager.Inspect(r.Context(), candidate.Directory, candidate)
			if inspectErr != nil {
				snapshot.Error = inspectErr.Error()
			} else {
				snapshot.Project = project
				if snapshot.Availability.State == "ready" {
					if statusErr := c.Manager.Status(r.Context(), project.Directory, project); statusErr != nil {
						snapshot.Error = statusErr.Error()
					}
				}
			}
		}
	}
	replyJSON(w, 200, snapshot)
}

func (c *Controller) Start(w http.ResponseWriter, r *http.Request) {
	workspace, ok := c.workspace(w, r)
	if !ok {
		return
	}
	var input struct{ File, Action, Service string }
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		replyError(w, 400, "invalid_request", "invalid JSON request")
		return
	}
	if !slicesAction(input.Action) {
		replyError(w, 400, "invalid_request", "unsupported Docker action")
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
	if input.Service != "" {
		valid := false
		for _, service := range project.Services {
			if service.Name == input.Service {
				valid = true
				break
			}
		}
		if !valid || strings.HasPrefix(input.Service, "-") {
			replyError(w, 400, "unknown_service", "unknown Compose service")
			return
		}
	}
	if input.Action == "down" && input.Service != "" {
		replyError(w, 400, "invalid_request", "down only supports the whole project")
		return
	}
	if input.Action == "build" && input.Service != "" {
		for _, service := range project.Services {
			if service.Name == input.Service && !service.Build {
				replyError(w, 400, "invalid_request", "service has no build definition")
				return
			}
		}
	}
	if err := c.Manager.checkIdentity(r.Context(), project.Directory, project); err != nil {
		replyError(w, 409, "project_conflict", err.Error())
		return
	}
	c.mu.Lock()
	for id, operation := range c.operations {
		if time.Since(operation.UpdatedAt) > time.Hour && operation.State != "running" {
			delete(c.operations, id)
		}
		if operation.WorkspaceID == workspace.ID.String() && operation.State == "running" {
			c.mu.Unlock()
			replyError(w, 409, "operation_running", "a Docker operation is already running for this workspace")
			return
		}
	}
	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	operation := &Operation{ID: id, WorkspaceID: workspace.ID.String(), Action: input.Action, State: "running", UpdatedAt: time.Now(), cancel: cancel}
	c.operations[id] = operation
	response := *operation
	c.mu.Unlock()
	go func() {
		defer cancel()
		// The workspace can be removed only after this operation finishes.
		err := c.Manager.Run(ctx, project.Directory, project, input.Action, input.Service)
		c.mu.Lock()
		defer c.mu.Unlock()
		if err != nil {
			operation.State, operation.Error = "failed", err.Error()
		} else {
			operation.State = "succeeded"
		}
		operation.UpdatedAt = time.Now()
	}()
	replyJSON(w, 202, response)
}

func slicesAction(action string) bool {
	switch action {
	case "up", "start", "stop", "restart", "down", "build":
		return true
	}
	return false
}

func (c *Controller) Operation(w http.ResponseWriter, r *http.Request) {
	workspace, ok := c.workspace(w, r)
	if !ok {
		return
	}
	c.mu.Lock()
	operation := c.operations[chi.URLParam(r, "operationID")]
	if operation == nil || operation.WorkspaceID != workspace.ID.String() {
		c.mu.Unlock()
		replyError(w, 404, "not_found", "operation not found")
		return
	}
	copy := *operation
	if r.Method == http.MethodDelete && operation.State == "running" {
		operation.cancel()
	}
	c.mu.Unlock()
	replyJSON(w, 200, copy)
}

func (c *Controller) Active(id uuid.UUID) bool {
	c.mu.Lock()
	for _, operation := range c.operations {
		if operation.WorkspaceID == id.String() && operation.State == "running" {
			c.mu.Unlock()
			return true
		}
	}
	c.mu.Unlock()
	c.terminalMu.Lock()
	defer c.terminalMu.Unlock()
	for _, terminal := range c.terminals {
		if terminal.WorkspaceID == id {
			return true
		}
	}
	return false
}

func replyJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func replyError(w http.ResponseWriter, code int, kind, message string) {
	replyJSON(w, code, map[string]any{"error": map[string]string{"code": kind, "message": message}})
}

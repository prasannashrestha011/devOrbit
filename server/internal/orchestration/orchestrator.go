package orchestration

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/google/uuid"
	processService "github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

var ErrConflict = errors.New("workspace lifecycle operation is already active")
var ErrInvalidID = errors.New("invalid workspace id")

type WorkspaceStore interface {
	Get(context.Context, uuid.UUID) (*workspace.Workspace, error)
}

type ServiceStore interface {
	List(context.Context, uuid.UUID, int, int) ([]*model.Service, error)
}

type Lifecycle interface {
	Start(context.Context, uuid.UUID) (process.Status, error)
	Stop(context.Context, uuid.UUID) (process.Status, error)
}

type Runtime interface {
	Status(uuid.UUID) process.Status
	Closed() bool
}

type ServiceResult struct {
	ServiceID   uuid.UUID `json:"service_id"`
	ServiceName string    `json:"service_name"`
	Outcome     string    `json:"outcome"`
	PID         *int      `json:"pid"`
	Error       *string   `json:"error"`
}

type OperationResult struct {
	WorkspaceID uuid.UUID       `json:"workspace_id"`
	Operation   string          `json:"operation"`
	Total       int             `json:"total"`
	Succeeded   int             `json:"succeeded"`
	Failed      int             `json:"failed"`
	Skipped     int             `json:"skipped"`
	Results     []ServiceResult `json:"results"`
}

type ServiceStatus struct {
	Name           string `json:"name"`
	Enabled        bool   `json:"enabled"`
	ListeningPorts []int  `json:"listening_ports"`
	process.Status
}

type WorkspaceStatus struct {
	WorkspaceID      uuid.UUID       `json:"workspace_id"`
	WorkspaceName    string          `json:"workspace_name"`
	AggregateState   string          `json:"aggregate_state"`
	TotalServices    int             `json:"total_services"`
	EnabledServices  int             `json:"enabled_services"`
	RunningServices  int             `json:"running_services"`
	StoppedServices  int             `json:"stopped_services"`
	FailedServices   int             `json:"failed_services"`
	StartingServices int             `json:"starting_services"`
	StoppingServices int             `json:"stopping_services"`
	ExitedServices   int             `json:"exited_services"`
	Services         []ServiceStatus `json:"services"`
}

type Orchestrator struct {
	workspaces WorkspaceStore
	services   ServiceStore
	lifecycle  Lifecycle
	runtime    Runtime
	mu         sync.Mutex
	active     map[uuid.UUID]bool
}

func New(workspaces WorkspaceStore, services ServiceStore, lifecycle Lifecycle, runtime Runtime) *Orchestrator {
	return &Orchestrator{workspaces: workspaces, services: services, lifecycle: lifecycle, runtime: runtime, active: make(map[uuid.UUID]bool)}
}

func (o *Orchestrator) reserve(id uuid.UUID) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.active[id] {
		return ErrConflict
	}
	o.active[id] = true
	return nil
}

func (o *Orchestrator) release(id uuid.UUID) {
	o.mu.Lock()
	delete(o.active, id)
	o.mu.Unlock()
}

func (o *Orchestrator) load(ctx context.Context, id uuid.UUID) (*workspace.Workspace, []*model.Service, error) {
	if id == uuid.Nil {
		return nil, nil, ErrInvalidID
	}
	w, err := o.workspaces.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	services, err := o.services.List(ctx, id, 1<<30, 0)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(services, func(i, j int) bool {
		if services[i].Name == services[j].Name {
			return services[i].ID.String() < services[j].ID.String()
		}
		return services[i].Name < services[j].Name
	})
	return w, services, nil
}

func enabled(s *model.Service) bool { return s.Enabled == nil || *s.Enabled }

func (o *Orchestrator) StartWorkspace(ctx context.Context, id uuid.UUID) (OperationResult, error) {
	if id == uuid.Nil {
		return OperationResult{}, ErrInvalidID
	}
	if o.runtime.Closed() {
		return OperationResult{}, process.ErrClosed
	}
	if err := o.reserve(id); err != nil {
		return OperationResult{}, err
	}
	defer o.release(id)
	_, services, err := o.load(ctx, id)
	if err != nil {
		return OperationResult{}, err
	}
	result := OperationResult{WorkspaceID: id, Operation: "start", Total: len(services), Results: make([]ServiceResult, 0, len(services))}
	for _, item := range services {
		if err := ctx.Err(); err != nil {
			return OperationResult{}, err
		}
		entry := ServiceResult{ServiceID: item.ID, ServiceName: item.Name}
		state := o.runtime.Status(item.ID)
		switch {
		case !enabled(item):
			entry.Outcome = "disabled"
		case state.State == process.Running:
			entry.Outcome, entry.PID = "already_running", state.PID
		case state.State == process.Starting || state.State == process.Stopping:
			entry.Outcome = "conflict"
		case o.runtime.Closed():
			entry.Outcome = "conflict"
		default:
			status, err := o.lifecycle.Start(ctx, item.ID)
			switch {
			case err == nil:
				entry.Outcome, entry.PID = "started", status.PID
			case errors.Is(err, process.ErrActive), errors.Is(err, process.ErrClosed):
				entry.Outcome = "conflict"
			case errors.Is(err, processService.ErrDisabled):
				entry.Outcome = "disabled"
			default:
				entry.Outcome = "failed"
			}
		}
		result.add(entry)
	}
	return result, nil
}

func (o *Orchestrator) StopWorkspace(ctx context.Context, id uuid.UUID) (OperationResult, error) {
	if id == uuid.Nil {
		return OperationResult{}, ErrInvalidID
	}
	if err := o.reserve(id); err != nil {
		return OperationResult{}, err
	}
	defer o.release(id)
	if _, err := o.workspaces.Get(ctx, id); err != nil {
		return OperationResult{}, err
	}
	_, services, err := o.load(ctx, id)
	if err != nil {
		services = nil
	}
	services = o.withActive(id, services)
	result := OperationResult{WorkspaceID: id, Operation: "stop", Total: len(services), Results: make([]ServiceResult, 0, len(services))}
	for i := len(services) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return OperationResult{}, err
		}
		item := services[i]
		entry := ServiceResult{ServiceID: item.ID, ServiceName: item.Name}
		state := o.runtime.Status(item.ID)
		switch state.State {
		case process.Starting:
			entry.Outcome = "conflict"
		case process.Running, process.Stopping:
			status, err := o.lifecycle.Stop(ctx, item.ID)
			switch {
			case err == nil && status.State == process.Stopped:
				entry.Outcome = "stopped"
			case err == nil && (status.State == process.Exited || status.State == process.Failed):
				entry.Outcome = "already_stopped"
			case errors.Is(err, process.ErrActive), errors.Is(err, process.ErrClosed):
				entry.Outcome = "conflict"
			default:
				entry.Outcome = "failed"
			}
		default:
			entry.Outcome = "already_stopped"
		}
		result.add(entry)
	}
	return result, nil
}

func (r *OperationResult) add(entry ServiceResult) {
	switch entry.Outcome {
	case "started", "stopped":
		r.Succeeded++
	case "already_running", "already_stopped", "disabled":
		r.Skipped++
	default:
		r.Failed++
		message := "service operation failed"
		if entry.Outcome == "conflict" {
			message = "service lifecycle operation is already active or shutting down"
		}
		entry.Error = &message
	}
	r.Results = append(r.Results, entry)
}

func (o *Orchestrator) GetWorkspaceStatus(ctx context.Context, id uuid.UUID) (WorkspaceStatus, error) {
	w, services, err := o.load(ctx, id)
	if err != nil {
		return WorkspaceStatus{}, err
	}
	services = o.withActive(id, services)
	result := WorkspaceStatus{WorkspaceID: id, WorkspaceName: w.Name, TotalServices: len(services), Services: make([]ServiceStatus, 0, len(services))}
	enabledRunning := 0
	for _, item := range services {
		status := o.runtime.Status(item.ID)
		if status.LastError != "" {
			status.LastError = "service execution failed"
		}
		isEnabled := enabled(item)
		serviceStatus := ServiceStatus{Name: item.Name, Enabled: isEnabled, Status: status}
		if status.State == process.Running && status.PID != nil {
			serviceStatus.ListeningPorts = process.ListeningPorts(*status.PID)
		}
		result.Services = append(result.Services, serviceStatus)
		if isEnabled {
			result.EnabledServices++
		}
		switch status.State {
		case process.Running:
			result.RunningServices++
			if isEnabled {
				enabledRunning++
			}
		case process.Starting:
			result.StartingServices++
		case process.Stopping:
			result.StoppingServices++
		case process.Failed:
			result.FailedServices++
		case process.Exited:
			result.ExitedServices++
		default:
			result.StoppedServices++
		}
	}
	switch {
	case result.StartingServices+result.StoppingServices > 0:
		result.AggregateState = "transitioning"
	case result.EnabledServices > 0 && enabledRunning == result.EnabledServices:
		result.AggregateState = "running"
	case result.RunningServices > 0:
		result.AggregateState = "partial"
	default:
		result.AggregateState = "idle"
	}
	return result, nil
}

func (o *Orchestrator) withActive(id uuid.UUID, services []*model.Service) []*model.Service {
	provider, ok := o.runtime.(interface {
		WorkspaceStatuses(uuid.UUID) []process.Status
	})
	if !ok {
		return services
	}
	seen := make(map[uuid.UUID]bool, len(services))
	for _, item := range services {
		seen[item.ID] = true
	}
	for _, status := range provider.WorkspaceStatuses(id) {
		if seen[status.ServiceID] {
			continue
		}
		if status.State != process.Running && status.State != process.Starting && status.State != process.Stopping {
			continue
		}
		services = append(services, &model.Service{ID: status.ServiceID, WorkspaceID: id, Name: status.ServiceID.String()})
	}
	return services
}

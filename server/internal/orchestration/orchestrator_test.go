package orchestration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

type fakeWorkspaces map[uuid.UUID]*workspace.Workspace

func (s fakeWorkspaces) Get(_ context.Context, id uuid.UUID) (*workspace.Workspace, error) {
	if item := s[id]; item != nil {
		return item, nil
	}
	return nil, workspaceRepo.ErrNotFound
}

type fakeServices []*model.Service

func (s fakeServices) List(_ context.Context, id uuid.UUID, limit, offset int) ([]*model.Service, error) {
	items := make([]*model.Service, 0)
	for _, item := range s {
		if item.WorkspaceID == id {
			items = append(items, item)
		}
	}
	if offset >= len(items) {
		return nil, nil
	}
	end := min(offset+limit, len(items))
	return items[offset:end], nil
}

type fakeRuntime struct {
	mu       sync.Mutex
	statuses map[uuid.UUID]process.Status
	closed   bool
}

func (r *fakeRuntime) Status(id uuid.UUID) process.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	if status, ok := r.statuses[id]; ok {
		return status
	}
	return process.Status{ServiceID: id, State: process.Stopped}
}

func (r *fakeRuntime) Closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

type fakeLifecycle struct {
	runtime  *fakeRuntime
	mu       sync.Mutex
	calls    []uuid.UUID
	startErr map[uuid.UUID]error
	stopErr  map[uuid.UUID]error
	gate     chan struct{}
	entered  chan struct{}
}

func (l *fakeLifecycle) Start(_ context.Context, id uuid.UUID) (process.Status, error) {
	l.mu.Lock()
	l.calls = append(l.calls, id)
	l.mu.Unlock()
	if l.entered != nil {
		l.entered <- struct{}{}
	}
	if l.gate != nil {
		<-l.gate
	}
	if err := l.startErr[id]; err != nil {
		return process.Status{}, err
	}
	status := process.Status{ServiceID: id, State: process.Running}
	l.runtime.mu.Lock()
	l.runtime.statuses[id] = status
	l.runtime.mu.Unlock()
	return status, nil
}

func (l *fakeLifecycle) Stop(_ context.Context, id uuid.UUID) (process.Status, error) {
	l.mu.Lock()
	l.calls = append(l.calls, id)
	l.mu.Unlock()
	if err := l.stopErr[id]; err != nil {
		return process.Status{}, err
	}
	status := process.Status{ServiceID: id, State: process.Stopped}
	l.runtime.mu.Lock()
	l.runtime.statuses[id] = status
	l.runtime.mu.Unlock()
	return status, nil
}

func fixture() (uuid.UUID, fakeWorkspaces, fakeServices, *fakeRuntime, *fakeLifecycle) {
	id := uuid.New()
	other := uuid.New()
	trueValue, falseValue := true, false
	created := time.Now().Add(-time.Hour)
	services := fakeServices{
		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), WorkspaceID: id, Name: "third", Enabled: &trueValue, CreatedAt: created.Add(time.Second)},
		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), WorkspaceID: id, Name: "second", Enabled: &falseValue, CreatedAt: created},
		{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), WorkspaceID: id, Name: "first", Enabled: &trueValue, CreatedAt: created},
		{ID: uuid.New(), WorkspaceID: other, Name: "other", Enabled: &trueValue},
	}
	runtime := &fakeRuntime{statuses: make(map[uuid.UUID]process.Status)}
	lifecycle := &fakeLifecycle{runtime: runtime, startErr: make(map[uuid.UUID]error), stopErr: make(map[uuid.UUID]error)}
	return id, fakeWorkspaces{id: {ID: id, Name: "workspace"}, other: {ID: other, Name: "other"}}, services, runtime, lifecycle
}

func TestStartStopAndStatus(t *testing.T) {
	id, workspaces, services, runtime, lifecycle := fixture()
	first, second, third := services[2].ID, services[1].ID, services[0].ID
	lifecycle.startErr[third] = errors.New("secret database error")
	o := New(workspaces, services, lifecycle, runtime)
	started, err := o.StartWorkspace(context.Background(), id)
	if err != nil || started.Total != 3 || started.Succeeded != 1 || started.Failed != 1 || started.Skipped != 1 {
		t.Fatalf("start: %+v %v", started, err)
	}
	if started.Results[0].ServiceID != first || started.Results[0].Outcome != "started" || started.Results[1].ServiceID != second || started.Results[1].Outcome != "disabled" || started.Results[2].ServiceID != third || started.Results[2].Outcome != "failed" || *started.Results[2].Error == "secret database error" {
		t.Fatalf("outcomes or order: %+v", started.Results)
	}
	if len(lifecycle.calls) != 2 || lifecycle.calls[0] != first || lifecycle.calls[1] != third {
		t.Fatalf("start calls: %v", lifecycle.calls)
	}
	status, err := o.GetWorkspaceStatus(context.Background(), id)
	if err != nil || status.AggregateState != "partial" || status.RunningServices != 1 || status.EnabledServices != 2 || len(status.Services) != 3 {
		t.Fatalf("status: %+v %v", status, err)
	}
	started, err = o.StartWorkspace(context.Background(), id)
	if err != nil || started.Results[0].Outcome != "already_running" {
		t.Fatalf("repeated start: %+v %v", started, err)
	}
	runtime.statuses[second] = process.Status{ServiceID: second, State: process.Running}
	stopped, err := o.StopWorkspace(context.Background(), id)
	if err != nil || stopped.Succeeded != 2 || stopped.Skipped != 1 || stopped.Results[0].ServiceID != third || stopped.Results[1].ServiceID != second || stopped.Results[2].ServiceID != first {
		t.Fatalf("stop: %+v %v", stopped, err)
	}
	if len(lifecycle.calls) != 5 || lifecycle.calls[3] != second || lifecycle.calls[4] != first {
		t.Fatalf("stop order: %v", lifecycle.calls)
	}
	status, _ = o.GetWorkspaceStatus(context.Background(), id)
	if status.AggregateState != "idle" || status.StoppedServices != 3 {
		t.Fatalf("idle: %+v", status)
	}
}

func TestConflictAndIndependentWorkspace(t *testing.T) {
	id, workspaces, services, runtime, lifecycle := fixture()
	lifecycle.gate = make(chan struct{})
	lifecycle.entered = make(chan struct{}, 1)
	o := New(workspaces, services, lifecycle, runtime)
	done := make(chan error, 1)
	go func() { _, err := o.StartWorkspace(context.Background(), id); done <- err }()
	<-lifecycle.entered
	if _, err := o.StartWorkspace(context.Background(), id); !errors.Is(err, ErrConflict) {
		t.Fatalf("same workspace start conflict: %v", err)
	}
	if _, err := o.StopWorkspace(context.Background(), id); !errors.Is(err, ErrConflict) {
		t.Fatalf("same workspace stop conflict: %v", err)
	}
	other := services[3].WorkspaceID
	if _, err := o.GetWorkspaceStatus(context.Background(), other); err != nil {
		t.Fatalf("independent workspace status: %v", err)
	}
	close(lifecycle.gate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := o.StartWorkspace(context.Background(), id); err != nil {
		t.Fatalf("reservation leaked: %v", err)
	}
	runtime.mu.Lock()
	runtime.closed = true
	runtime.mu.Unlock()
	if _, err := o.StartWorkspace(context.Background(), id); !errors.Is(err, process.ErrClosed) {
		t.Fatalf("shutdown conflict: %v", err)
	}
}

func TestAggregationAndValidation(t *testing.T) {
	id, workspaces, services, runtime, lifecycle := fixture()
	o := New(workspaces, services, lifecycle, runtime)
	if _, err := o.StartWorkspace(context.Background(), uuid.Nil); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("nil ID: %v", err)
	}
	if _, err := o.StopWorkspace(context.Background(), uuid.New()); !errors.Is(err, workspaceRepo.ErrNotFound) {
		t.Fatalf("missing workspace: %v", err)
	}
	runtime.statuses[services[0].ID] = process.Status{ServiceID: services[0].ID, State: process.Starting}
	status, _ := o.GetWorkspaceStatus(context.Background(), id)
	if status.AggregateState != "transitioning" || status.StartingServices != 1 {
		t.Fatalf("transitioning: %+v", status)
	}
	runtime.statuses[services[0].ID] = process.Status{ServiceID: services[0].ID, State: process.Running}
	runtime.statuses[services[2].ID] = process.Status{ServiceID: services[2].ID, State: process.Running}
	status, _ = o.GetWorkspaceStatus(context.Background(), id)
	if status.AggregateState != "running" || status.RunningServices != 2 {
		t.Fatalf("running: %+v", status)
	}
	other := services[3].WorkspaceID
	status, _ = o.GetWorkspaceStatus(context.Background(), other)
	if status.AggregateState != "idle" {
		t.Fatalf("empty runtime: %+v", status)
	}
}

func TestAllServicesAreLoaded(t *testing.T) {
	id := uuid.New()
	yes := true
	services := make(fakeServices, 205)
	for i := range services {
		services[i] = &model.Service{ID: uuid.New(), WorkspaceID: id, Enabled: &yes}
	}
	runtime := &fakeRuntime{statuses: make(map[uuid.UUID]process.Status)}
	lifecycle := &fakeLifecycle{runtime: runtime}
	o := New(fakeWorkspaces{id: {ID: id}}, services, lifecycle, runtime)
	result, err := o.StartWorkspace(context.Background(), id)
	if err != nil || result.Total != 205 || result.Succeeded != 205 || len(result.Results) != 205 {
		t.Fatalf("paginated start: %+v %v", result, err)
	}
}

func TestLifecycleConflictsAndStopFailure(t *testing.T) {
	id, workspaces, services, runtime, lifecycle := fixture()
	first, third := services[2].ID, services[0].ID
	runtime.statuses[first] = process.Status{ServiceID: first, State: process.Starting}
	runtime.statuses[third] = process.Status{ServiceID: third, State: process.Stopping}
	o := New(workspaces, services, lifecycle, runtime)
	started, err := o.StartWorkspace(context.Background(), id)
	if err != nil || started.Failed != 2 || started.Results[0].Outcome != "conflict" || started.Results[2].Outcome != "conflict" {
		t.Fatalf("start conflicts: %+v %v", started, err)
	}
	runtime.statuses[first] = process.Status{ServiceID: first, State: process.Running}
	lifecycle.stopErr[first] = errors.New("private stop detail")
	lifecycle.stopErr[third] = process.ErrActive
	stopped, err := o.StopWorkspace(context.Background(), id)
	if err != nil || stopped.Failed != 2 || stopped.Results[0].Outcome != "conflict" || stopped.Results[2].Outcome != "failed" || *stopped.Results[2].Error == "private stop detail" {
		t.Fatalf("stop outcomes: %+v %v", stopped, err)
	}
	if runtime.Status(first).State != process.Running {
		t.Fatal("failed stop claimed termination")
	}
	noEnabledID := uuid.New()
	no := false
	workspaces[noEnabledID] = &workspace.Workspace{ID: noEnabledID}
	services = append(services, &model.Service{ID: uuid.New(), WorkspaceID: noEnabledID, Enabled: &no})
	o = New(workspaces, services, lifecycle, runtime)
	status, err := o.GetWorkspaceStatus(context.Background(), noEnabledID)
	if err != nil || status.AggregateState != "idle" || status.EnabledServices != 0 {
		t.Fatalf("disabled-only workspace: %+v %v", status, err)
	}
}

type realLifecycle struct {
	supervisor *process.Supervisor
	configs    map[uuid.UUID]process.Config
}

func (l realLifecycle) Start(_ context.Context, id uuid.UUID) (process.Status, error) {
	return l.supervisor.Start(id, l.configs[id])
}
func (l realLifecycle) Stop(ctx context.Context, id uuid.UUID) (process.Status, error) {
	return l.supervisor.Stop(ctx, id)
}

func TestOrchestrationChild(t *testing.T) {
	if os.Getenv("DEVORBIT_ORCHESTRATION_CHILD") == "1" {
		for {
			time.Sleep(time.Second)
		}
	}
}

func TestRealSupervisorPartialStartAndStop(t *testing.T) {
	id := uuid.New()
	good, bad, last := uuid.New(), uuid.New(), uuid.New()
	yes := true
	services := fakeServices{
		{ID: good, WorkspaceID: id, Name: "good", Enabled: &yes, CreatedAt: time.Unix(1, 0)},
		{ID: bad, WorkspaceID: id, Name: "bad", Enabled: &yes, CreatedAt: time.Unix(2, 0)},
		{ID: last, WorkspaceID: id, Name: "last", Enabled: &yes, CreatedAt: time.Unix(3, 0)},
	}
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cfg := process.Config{Command: self, Args: []string{"-test.run=^TestOrchestrationChild$"}, Dir: dir, Env: map[string]string{"DEVORBIT_ORCHESTRATION_CHILD": "1"}}
	supervisor := process.New(30 * time.Millisecond)
	t.Cleanup(func() { _ = supervisor.Shutdown(context.Background()) })
	lifecycle := realLifecycle{supervisor: supervisor, configs: map[uuid.UUID]process.Config{
		good: cfg,
		bad:  {Command: "missing-devorbit-orchestration-executable", Dir: dir},
		last: cfg,
	}}
	o := New(fakeWorkspaces{id: {ID: id, Name: "workspace"}}, services, lifecycle, supervisor)
	started, err := o.StartWorkspace(context.Background(), id)
	if err != nil || started.Succeeded != 2 || started.Failed != 1 || started.Results[1].Outcome != "failed" || supervisor.Status(last).State != process.Running {
		t.Fatalf("partial start: %+v %v", started, err)
	}
	stopped, err := o.StopWorkspace(context.Background(), id)
	if err != nil || stopped.Succeeded != 2 || stopped.Skipped != 1 || supervisor.Status(good).State != process.Stopped || supervisor.Status(last).State != process.Stopped {
		t.Fatalf("stop after partial start: %+v %v", stopped, err)
	}
}

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
)

type State string

const (
	Stopped  State = "stopped"
	Starting State = "starting"
	Running  State = "running"
	Stopping State = "stopping"
	Exited   State = "exited"
	Failed   State = "failed"
)

var ErrActive = errors.New("service lifecycle operation is already active")
var ErrClosed = errors.New("process supervisor is shutting down")
var ErrInvalid = errors.New("invalid execution configuration")
var errGraceUnavailable = errors.New("graceful signal unavailable")

func invalid(reason string) error { return fmt.Errorf("%w: %s", ErrInvalid, reason) }

type Config struct {
	WorkspaceID uuid.UUID
	Command     string
	Args        []string
	Dir         string
	Env         map[string]string
}

type Status struct {
	ServiceID   uuid.UUID  `json:"service_id"`
	ExecutionID uuid.UUID  `json:"execution_id"`
	PID         *int       `json:"pid"`
	State       State      `json:"state"`
	StartedAt   *time.Time `json:"started_at"`
	ExitedAt    *time.Time `json:"exited_at"`
	ExitCode    *int       `json:"exit_code"`
	LastError   string     `json:"last_error"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type entry struct {
	status      Status
	workspaceID uuid.UUID
	cmd         *exec.Cmd
	tree        *processTree
	done        chan struct{}
	op          string
	opDone      *operation
}

type operation struct {
	done   chan struct{}
	status Status
	err    error
}

type Supervisor struct {
	mu      sync.Mutex
	entries map[uuid.UUID]*entry
	ops     sync.WaitGroup
	closed  bool
	grace   time.Duration
	logs    *logs.Manager
	events  *events.Broker
}

func New(grace time.Duration, manager ...*logs.Manager) *Supervisor {
	s := &Supervisor{entries: make(map[uuid.UUID]*entry), grace: grace}
	if len(manager) > 0 {
		s.logs = manager[0]
	}
	return s
}

// SetBroker wires lifecycle publishing before the supervisor accepts requests.
func (s *Supervisor) SetBroker(broker *events.Broker) { s.events = broker }

func (s *Supervisor) publish(kind events.Type, e *entry) {
	data := map[string]any{"state": e.status.State}
	if e.status.PID != nil {
		data["pid"] = *e.status.PID
	}
	if e.status.ExitCode != nil {
		data["exit_code"] = *e.status.ExitCode
	}
	var executionID *uuid.UUID
	if e.status.ExecutionID != uuid.Nil {
		id := e.status.ExecutionID
		executionID = &id
	}
	s.events.Publish(events.Event{Type: kind, Timestamp: e.status.UpdatedAt, WorkspaceID: e.workspaceID, ServiceID: e.status.ServiceID, ExecutionID: executionID, Data: data})
}

func (s *Supervisor) Status(id uuid.UUID) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.entries[id]; e != nil {
		return snapshot(e.status)
	}
	return Status{ServiceID: id, State: Stopped}
}

func (s *Supervisor) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Supervisor) Start(id uuid.UUID, cfg Config) (Status, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Status{}, ErrClosed
	}
	if e := s.entries[id]; e != nil && (e.op != "" || e.cmd != nil) {
		s.mu.Unlock()
		return Status{}, ErrActive
	}
	now := time.Now().UTC()
	e := &entry{status: Status{ServiceID: id, State: Starting, UpdatedAt: now}, op: "start"}
	s.entries[id] = e
	s.ops.Add(1)
	s.mu.Unlock()
	defer s.finish(e, nil)
	return s.launch(e, cfg)
}

func (s *Supervisor) launch(e *entry, cfg Config) (Status, error) {
	s.mu.Lock()
	e.workspaceID = cfg.WorkspaceID
	s.mu.Unlock()
	cmd, err := command(cfg)
	if err != nil {
		return s.failStart(e, err), err
	}
	tree, err := newProcessTree(cmd)
	if err != nil {
		return s.failStart(e, err), err
	}
	var capture *logs.Capture
	if s.logs != nil {
		capture, err = s.logs.Begin(cfg.WorkspaceID, e.status.ServiceID)
		if err != nil {
			tree.close()
			return s.failStart(e, errors.New("log capture unavailable")), errors.New("log capture unavailable")
		}
		cmd.Stdout, cmd.Stderr = capture.Stdout, capture.Stderr
		cmd.WaitDelay = 2 * time.Second
	}
	if err := cmd.Start(); err != nil {
		if capture != nil {
			_ = capture.Close()
		}
		tree.close()
		// Command errors may contain environment values; keep status and API errors generic.
		err = invalid("process launch failed")
		return s.failStart(e, err), err
	}
	if err := tree.attach(cmd); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if capture != nil {
			_ = capture.Close()
		}
		tree.close()
		err = errors.New("process ownership setup failed")
		return s.failStart(e, err), err
	}
	s.mu.Lock()
	now := time.Now().UTC()
	e.cmd = cmd
	e.tree = tree
	e.done = make(chan struct{})
	e.status.PID = &cmd.Process.Pid
	if capture != nil {
		e.status.ExecutionID = capture.ExecutionID()
	}
	e.status.StartedAt = &now
	e.status.UpdatedAt = now
	e.status.State = Running
	status := snapshot(e.status)
	closed := s.closed
	s.publish(events.ProcessStarted, e)
	if e.op == "restart" {
		s.publish(events.ProcessRestarted, e)
	}
	s.mu.Unlock()
	go s.wait(e, cmd, tree, capture)
	if closed {
		return status, ErrClosed
	}
	return status, nil
}

func (s *Supervisor) failStart(e *entry, err error) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	e.status.State = Failed
	e.status.PID = nil
	e.status.StartedAt = nil
	e.status.ExitedAt = nil
	e.status.ExitCode = nil
	e.status.LastError = err.Error()
	e.status.UpdatedAt = now
	s.publish(events.ProcessFailed, e)
	return snapshot(e.status)
}

func (s *Supervisor) wait(e *entry, cmd *exec.Cmd, tree *processTree, capture *logs.Capture) {
	err := cmd.Wait()
	// A launcher such as go run may exit while its child is still in the owned tree.
	_ = tree.kill()
	tree.close()
	if capture != nil {
		_ = capture.Close()
	}
	s.mu.Lock()
	if s.entries[e.status.ServiceID] == e && e.cmd == cmd {
		now := time.Now().UTC()
		code := cmd.ProcessState.ExitCode()
		e.status.ExitCode = &code
		e.status.ExitedAt = &now
		e.status.UpdatedAt = now
		if e.status.State == Stopping {
			e.status.State = Stopped
			e.status.LastError = ""
			s.publish(events.ProcessStopped, e)
		} else if err == nil {
			e.status.State = Exited
			s.publish(events.ProcessExited, e)
		} else {
			e.status.State = Failed
			e.status.LastError = fmt.Sprintf("process exited with code %d", code)
			s.publish(events.ProcessFailed, e)
		}
		e.cmd = nil
		e.tree = nil
	}
	close(e.done)
	s.mu.Unlock()
}

func (s *Supervisor) finish(e *entry, err error) {
	s.mu.Lock()
	e.op = ""
	if e.opDone != nil {
		e.opDone.status = snapshot(e.status)
		e.opDone.err = err
		close(e.opDone.done)
		e.opDone = nil
	}
	s.mu.Unlock()
	s.ops.Done()
}

func (s *Supervisor) Stop(ctx context.Context, id uuid.UUID) (Status, error) {
	return s.stop(ctx, id, false)
}

func (s *Supervisor) stop(ctx context.Context, id uuid.UUID, shutdown bool) (Status, error) {
	s.mu.Lock()
	if s.closed && !shutdown {
		s.mu.Unlock()
		return Status{}, ErrClosed
	}
	e := s.entries[id]
	if e == nil || e.cmd == nil && e.op == "" {
		status := Status{ServiceID: id, State: Stopped}
		if e != nil {
			status = snapshot(e.status)
		}
		s.mu.Unlock()
		return status, nil
	}
	if e.op == "stop" {
		op := e.opDone
		s.mu.Unlock()
		select {
		case <-op.done:
			return op.status, op.err
		case <-ctx.Done():
			return Status{}, ctx.Err()
		}
	}
	if e.op != "" {
		s.mu.Unlock()
		return Status{}, ErrActive
	}
	e.op, e.opDone = "stop", &operation{done: make(chan struct{})}
	e.status.State = Stopping
	e.status.UpdatedAt = time.Now().UTC()
	s.publish(events.ProcessStopping, e)
	s.ops.Add(1)
	s.mu.Unlock()
	err := s.stopProcess(ctx, e)
	s.mu.Lock()
	status := snapshot(e.status)
	s.mu.Unlock()
	s.finish(e, err)
	return status, err
}

func (s *Supervisor) stopProcess(ctx context.Context, e *entry) error {
	s.mu.Lock()
	tree, done := e.tree, e.done
	s.mu.Unlock()
	if tree == nil {
		return nil // The process completed between reserving the operation and stopping it.
	}
	err := tree.interrupt()
	if errors.Is(err, errGraceUnavailable) {
		err = nil
	} else if err == nil {
		timer := time.NewTimer(s.grace)
		defer timer.Stop()
		select {
		case <-done:
			return nil
		case <-timer.C:
		case <-ctx.Done():
		}
	}
	if killErr := tree.kill(); killErr != nil {
		return errors.Join(err, killErr)
	}
	<-done
	return err
}

func (s *Supervisor) Restart(ctx context.Context, id uuid.UUID, load func(context.Context) (Config, error)) (Status, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Status{}, ErrClosed
	}
	e := s.entries[id]
	if e != nil && e.op != "" {
		s.mu.Unlock()
		return Status{}, ErrActive
	}
	if e == nil {
		e = &entry{status: Status{ServiceID: id}}
		s.entries[id] = e
	}
	e.op = "restart"
	running := e.cmd != nil
	if running {
		e.status.State = Stopping
		e.status.UpdatedAt = time.Now().UTC()
		s.publish(events.ProcessStopping, e)
	}
	s.ops.Add(1)
	s.mu.Unlock()
	defer s.finish(e, nil)
	if running {
		if err := s.stopProcess(ctx, e); err != nil {
			return s.Status(id), err
		}
	}
	if err := ctx.Err(); err != nil {
		return s.Status(id), err
	}
	cfg, err := load(ctx)
	if err != nil {
		return s.Status(id), err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return Status{}, ErrClosed
	}
	e.status = Status{ServiceID: id, State: Starting, UpdatedAt: time.Now().UTC()}
	s.mu.Unlock()
	return s.launch(e, cfg)
}

func (s *Supervisor) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.ops.Wait()
	s.mu.Lock()
	ids := make([]uuid.UUID, 0)
	for id, e := range s.entries {
		if e.cmd != nil {
			ids = append(ids, id)
		}
	}
	s.mu.Unlock()
	var wg sync.WaitGroup
	errorsByService := make(chan error, len(ids))
	for _, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.stop(ctx, id, true)
			if err != nil {
				errorsByService <- err
			}
		}()
	}
	wg.Wait()
	close(errorsByService)
	for err := range errorsByService {
		if err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func snapshot(status Status) Status {
	if status.PID != nil {
		v := *status.PID
		status.PID = &v
	}
	if status.StartedAt != nil {
		v := *status.StartedAt
		status.StartedAt = &v
	}
	if status.ExitedAt != nil {
		v := *status.ExitedAt
		status.ExitedAt = &v
	}
	if status.ExitCode != nil {
		v := *status.ExitCode
		status.ExitCode = &v
	}
	return status
}

func command(cfg Config) (*exec.Cmd, error) {
	if cfg.Command == "" || cfg.Dir == "" || strings.ContainsRune(cfg.Command, 0) || strings.ContainsRune(cfg.Dir, 0) {
		return nil, ErrInvalid
	}
	for _, arg := range cfg.Args {
		if strings.ContainsRune(arg, 0) {
			return nil, ErrInvalid
		}
	}
	dir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, ErrInvalid
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, invalid("working directory unavailable")
	}
	env, err := environment(cfg.Env)
	if err != nil {
		return nil, err
	}
	path, err := executable(cfg.Command, dir, env)
	if err != nil {
		return nil, invalid("executable unavailable or not executable")
	}
	cmd := exec.Command(path, cfg.Args...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd, nil
}

func environment(overrides map[string]string) ([]string, error) {
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+len(overrides))
	index := make(map[string]int, len(inherited))
	key := func(k string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(k)
		}
		return k
	}
	for _, pair := range inherited {
		if pos := strings.IndexByte(pair, '='); pos > 0 {
			if i, ok := index[key(pair[:pos])]; ok {
				env[i] = pair
				continue
			}
			index[key(pair[:pos])] = len(env)
		}
		env = append(env, pair)
	}
	keys := make([]string, 0, len(overrides))
	for k, v := range overrides {
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
			return nil, ErrInvalid
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pair := k + "=" + overrides[k]
		if i, ok := index[key(k)]; ok {
			env[i] = pair
		} else {
			index[key(k)] = len(env)
			env = append(env, pair)
		}
	}
	return env, nil
}

func executable(name, dir string, env []string) (string, error) {
	paths := []string{name}
	if !filepath.IsAbs(name) {
		if strings.ContainsAny(name, `/\`) {
			paths = []string{filepath.Join(dir, name)}
		} else {
			paths = nil
			pathValue := ""
			for _, pair := range env {
				k, v, ok := strings.Cut(pair, "=")
				if ok && ((runtime.GOOS == "windows" && strings.EqualFold(k, "PATH")) || k == "PATH") {
					pathValue = v
				}
			}
			for _, part := range filepath.SplitList(pathValue) {
				if !filepath.IsAbs(part) {
					part = filepath.Join(dir, part)
				}
				paths = append(paths, filepath.Join(part, name))
			}
		}
	}
	for _, path := range paths {
		if found, err := executableFile(path, env); err == nil {
			return found, nil
		}
	}
	return "", ErrInvalid
}

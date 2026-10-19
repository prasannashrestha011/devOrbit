package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/phuslu/pty"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
)

func privateTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLifecycleAndLogEvents(t *testing.T) {
	b := events.New()
	defer b.Close()
	m, err := logs.New(privateTempDir(t), b)
	if err != nil {
		t.Fatal(err)
	}
	s := New(50*time.Millisecond, m)
	s.SetBroker(b)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()); _ = m.Close() })
	wid, sid := uuid.New(), uuid.New()
	stream, unsubscribe := b.Subscribe(events.Filter{ServiceID: sid})
	defer unsubscribe()
	next := func() events.Event {
		t.Helper()
		select {
		case event := <-stream:
			if event.WorkspaceID != wid || event.ServiceID != sid || event.ID == "" {
				t.Fatalf("bad event: %+v", event)
			}
			return event
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for event")
			return events.Event{}
		}
	}
	cfg := helperConfig(t, "logs")
	cfg.WorkspaceID = wid
	first, err := s.Start(sid, cfg)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[events.Type]int{}
	for {
		event := next()
		counts[event.Type]++
		if event.ExecutionID == nil || *event.ExecutionID != first.ExecutionID {
			t.Fatalf("wrong execution: %+v", event)
		}
		if event.Type == events.ProcessExited {
			break
		}
	}
	if counts[events.ProcessStarted] != 1 || counts[events.LogCreated] < 1 || counts[events.ProcessExited] != 1 {
		t.Fatalf("unexpected event counts: %+v", counts)
	}
	waitState(t, s, sid, Exited)
	wait := helperConfig(t, "wait")
	wait.WorkspaceID = wid
	if _, err := s.Start(sid, wait); err != nil {
		t.Fatal(err)
	}
	if event := next(); event.Type != events.ProcessStarted {
		t.Fatalf("want started: %+v", event)
	}
	exit := helperConfig(t, "exit")
	exit.WorkspaceID = wid
	if _, err := s.Restart(context.Background(), sid, configLoader(exit)); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []events.Type{events.ProcessStopping, events.ProcessStopped, events.ProcessStarted, events.ProcessRestarted, events.ProcessExited} {
		if event := next(); event.Type != kind {
			t.Fatalf("want %s: %+v", kind, event)
		}
	}
	waitState(t, s, sid, Exited)
	exit.Command = "missing-devorbit-executable"
	if _, err := s.Start(sid, exit); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if event := next(); event.Type != events.ProcessFailed || event.ExecutionID != nil {
		t.Fatalf("want failed start without execution: %+v", event)
	}
}

func TestProcessHelper(t *testing.T) {
	switch os.Getenv("DEVORBIT_HELPER") {
	case "wait":
		for {
			time.Sleep(time.Second)
		}
	case "graceful", "ignore":
		if os.Getenv("DEVORBIT_HELPER") == "ignore" {
			signal.Ignore(os.Interrupt)
		} else {
			interrupt := make(chan os.Signal, 1)
			signal.Notify(interrupt, os.Interrupt)
			defer signal.Stop(interrupt)
			defer func() { _ = os.WriteFile(os.Getenv("DEVORBIT_TEST_MARKER"), []byte("stopped"), 0600) }()
			_ = os.WriteFile(os.Getenv("DEVORBIT_TEST_READY"), []byte("ready"), 0600)
			<-interrupt
			return
		}
		_ = os.WriteFile(os.Getenv("DEVORBIT_TEST_READY"), []byte("ready"), 0600)
		for {
			time.Sleep(time.Second)
		}
	case "exit":
		os.Exit(0)
	case "logs":
		_, _ = os.Stdout.WriteString("stdout line\npartial")
		_, _ = os.Stderr.WriteString("stderr line\r\n")
		os.Exit(0)
	case "interactive":
		_, _ = os.Stdout.WriteString("\x1b[32mready\x1b[0m\n")
		var input [64]byte
		_, _ = os.Stdin.Read(input[:])
		_, _ = os.Stdout.WriteString("ack\n")
		os.Exit(0)
	case "noisy":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 2<<20))
		os.Exit(0)
	case "fail":
		os.Exit(7)
	case "env":
		if err := os.WriteFile(os.Getenv("DEVORBIT_TEST_OUT"), []byte(os.Getenv("DEVORBIT_TEST_VALUE")), 0600); err != nil {
			os.Exit(8)
		}
		os.Exit(0)
	case "spawn":
		child := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
		child.Env = append(os.Environ(), "DEVORBIT_HELPER=heartbeat")
		if err := child.Start(); err != nil {
			os.Exit(9)
		}
		if err := os.WriteFile(os.Getenv("DEVORBIT_TEST_READY"), []byte("ready"), 0600); err != nil {
			os.Exit(10)
		}
		for {
			time.Sleep(time.Second)
		}
	case "heartbeat":
		for {
			_ = os.WriteFile(os.Getenv("DEVORBIT_TEST_BEAT"), []byte(time.Now().Format(time.RFC3339Nano)), 0600)
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestSupervisorCapturesProcessOutput(t *testing.T) {
	m, err := logs.New(privateTempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	s := New(50*time.Millisecond, m)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()); _ = m.Close() })
	id, workspace := uuid.New(), uuid.New()
	cfg := helperConfig(t, "logs")
	cfg.WorkspaceID = workspace
	status, err := s.Start(id, cfg)
	if err != nil || status.ExecutionID == uuid.Nil {
		t.Fatalf("start = %+v, %v", status, err)
	}
	waitState(t, s, id, Exited)
	entries, err := m.List(context.Background(), logs.Filter{ServiceID: id, Limit: 10})
	if err != nil || len(entries) == 0 {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	var output string
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.ExecutionID != status.ExecutionID || e.WorkspaceID != workspace {
			t.Fatalf("wrong execution: %+v", e)
		}
		if e.Stream != "terminal" {
			t.Fatalf("unexpected stream: %+v", e)
		}
		output += e.Message
	}
	for _, expected := range []string{"stdout line", "partial", "stderr line"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("missing %q: %q", expected, output)
		}
	}
}

func TestTerminalInputResizeAndReplay(t *testing.T) {
	m, err := logs.New(privateTempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	s := New(50*time.Millisecond, m)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()); _ = m.Close() })
	id, workspace := uuid.New(), uuid.New()
	cfg := helperConfig(t, "interactive")
	cfg.WorkspaceID = workspace
	status, err := s.Start(id, cfg)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := s.Terminal(id, workspace, status.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	frames, gap, live, cancel := terminal.Subscribe(0)
	defer cancel()
	if gap {
		t.Fatal("unexpected replay gap")
	}
	if err := s.TerminalResize(id, workspace, status.ExecutionID, 88, 24); err != nil {
		t.Fatal(err)
	}
	if size, err := pty.GetSize(terminal.pty); err != nil || size.Cols != 88 || size.Rows != 24 {
		t.Fatalf("size: %+v %v", size, err)
	}
	if err := s.TerminalInput(id, workspace, status.ExecutionID, []byte("secret\n")); err != nil {
		t.Fatal(err)
	}
	var output string
	for _, frame := range frames {
		output += string(frame.Data)
	}
	deadline := time.After(5 * time.Second)
	for !strings.Contains(output, "ack") {
		select {
		case frame, ok := <-live:
			if !ok {
				t.Fatalf("terminal closed without ack: %q", output)
			}
			output += string(frame.Data)
		case <-deadline:
			t.Fatalf("timed out waiting for output: %q", output)
		}
	}
	if !strings.Contains(output, "\x1b[32mready") || strings.Contains(output, "secret") {
		t.Fatalf("unexpected terminal output: %q", output)
	}
	waitState(t, s, id, Exited)
	if err := s.TerminalInput(id, workspace, status.ExecutionID, []byte("x")); !errors.Is(err, ErrStaleTerminal) {
		t.Fatalf("stale input: %v", err)
	}
	if _, err := s.Terminal(id, uuid.New(), status.ExecutionID); !errors.Is(err, ErrStaleTerminal) {
		t.Fatalf("wrong workspace: %v", err)
	}
	entries, err := m.List(context.Background(), logs.Filter{ServiceID: id, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Message, "secret") {
			t.Fatalf("input entered logs: %+v", entry)
		}
	}
}

func TestTmuxServiceKeepsShellWithoutLoggingShellInput(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	m, err := logs.New(privateTempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	s := New(50*time.Millisecond, m)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()); _ = m.Close() })
	id, workspace := uuid.New(), uuid.New()
	cfg := helperConfig(t, "logs")
	cfg.WorkspaceID, cfg.Interactive, cfg.Tmux = workspace, true, true
	cfg.Name = "API server"
	status, err := s.Start(id, cfg)
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, s, id, Exited)
	s.mu.Lock()
	tmux := s.entries[id].tmux
	s.mu.Unlock()
	if tmux.name != "API-server" {
		t.Fatalf("tmux session name = %q", tmux.name)
	}
	terminal, err := s.Terminal(id, workspace, status.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, live, cancel := terminal.Subscribe(0)
	defer cancel()
	if err := s.TerminalInput(id, workspace, status.ExecutionID, []byte("echo after-service-marker\r")); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	var output string
	for strings.Count(output, "after-service-marker") < 2 {
		select {
		case frame := <-live:
			output += string(frame.Data)
		case <-deadline:
			t.Fatalf("tmux shell did not respond: %q", output)
		}
	}
	if err := s.TerminalInput(id, workspace, status.ExecutionID, []byte("\x02%")); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(3 * time.Second)
	for {
		panes, err := exec.Command(tmux.path, "-S", tmux.socket, "list-panes", "-t", tmux.name, "-F", "#{pane_id}").Output()
		if err == nil && len(strings.Fields(string(panes))) == 2 {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("tmux split did not create a second pane: %q (%v), output %q", panes, err, output)
		}
		time.Sleep(20 * time.Millisecond)
	}
	activePane := func() string {
		pane, err := exec.Command(tmux.path, "-S", tmux.socket, "display-message", "-p", "-t", tmux.name, "#{pane_id}").Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(pane))
	}
	otherPane := activePane()
	if otherPane == tmux.pane {
		t.Fatal("split did not focus its new pane")
	}
	for _, click := range []struct {
		column int
		want   string
	}{{2, tmux.pane}, {75, otherPane}} {
		mouse := fmt.Sprintf("\x1b[<0;%d;2M\x1b[<0;%d;2m", click.column, click.column)
		if err := s.TerminalInput(id, workspace, status.ExecutionID, []byte(mouse)); err != nil {
			t.Fatal(err)
		}
		until := time.Now().Add(2 * time.Second)
		for activePane() != click.want {
			if time.Now().After(until) {
				t.Fatalf("mouse click at column %d did not focus pane %s", click.column, click.want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	var foundStdout, foundStderr bool
	for _, entry := range m.Recent(id) {
		if strings.Contains(entry.Message, "stdout line") {
			foundStdout = true
		}
		if strings.Contains(entry.Message, "stderr line") {
			foundStderr = true
		}
		if strings.Contains(entry.Message, "after-service-marker") {
			t.Fatalf("shell input leaked into service logs: %+v", entry)
		}
	}
	if !foundStdout || !foundStderr {
		t.Fatalf("fast service output was missed: stdout=%v stderr=%v", foundStdout, foundStderr)
	}
}

func TestTmuxCtrlCLeavesServicePaneInteractive(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	s := New(2 * time.Second)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	id, workspace := uuid.New(), uuid.New()
	cfg := helperConfig(t, "graceful")
	cfg.WorkspaceID, cfg.Interactive, cfg.Tmux = workspace, true, true
	cfg.Name = "dev-app"
	cfg.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
	cfg.Env["DEVORBIT_TEST_MARKER"] = filepath.Join(t.TempDir(), "stopped")
	status, err := s.Start(id, cfg)
	if err != nil {
		t.Fatal(err)
	}
	waitFile(t, cfg.Env["DEVORBIT_TEST_READY"])
	terminal, err := s.Terminal(id, workspace, status.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, live, cancel := terminal.Subscribe(0)
	defer cancel()
	if err := s.TerminalInput(id, workspace, status.ExecutionID, []byte("\x03")); err != nil {
		t.Fatal(err)
	}
	waitFile(t, cfg.Env["DEVORBIT_TEST_MARKER"])
	waitState(t, s, id, Exited)
	if err := s.TerminalInput(id, workspace, status.ExecutionID, []byte("echo after-interrupt-marker\r")); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	var output string
	for strings.Count(output, "after-interrupt-marker") < 2 {
		select {
		case frame, ok := <-live:
			if !ok {
				t.Fatalf("terminal closed after first Ctrl+C: %q", output)
			}
			output += string(frame.Data)
		case <-deadline:
			t.Fatalf("no shell after first Ctrl+C: %q", output)
		}
	}
	if err := s.TerminalInput(id, workspace, status.ExecutionID, []byte("exit\r")); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(5 * time.Second)
	for !terminal.Closed() && time.Now().Before(until) {
		time.Sleep(20 * time.Millisecond)
	}
	if !terminal.Closed() {
		t.Fatal("shell did not close after second exit")
	}
}

func TestTmuxStopTargetsServicePaneAfterSplit(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	s := New(2 * time.Second)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	id := uuid.New()
	cfg := helperConfig(t, "graceful")
	cfg.WorkspaceID = uuid.New()
	cfg.Interactive, cfg.Tmux = true, true
	cfg.Name = "API server"
	cfg.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
	cfg.Env["DEVORBIT_TEST_MARKER"] = filepath.Join(t.TempDir(), "marker")
	status, err := s.Start(id, cfg)
	if err != nil {
		t.Fatal(err)
	}
	waitFile(t, cfg.Env["DEVORBIT_TEST_READY"])
	s.mu.Lock()
	tmux := s.entries[id].tmux
	s.mu.Unlock()
	if err := s.TerminalInput(id, cfg.WorkspaceID, status.ExecutionID, []byte("\x02%")); err != nil {
		t.Fatal(err)
	}
	until := time.Now().Add(3 * time.Second)
	for {
		panes, err := exec.Command(tmux.path, "-S", tmux.socket, "list-panes", "-t", tmux.name, "-F", "#{pane_id}").Output()
		if err == nil && len(strings.Fields(string(panes))) == 2 {
			break
		}
		if time.Now().After(until) {
			t.Fatalf("tmux split did not create a second pane: %q (%v)", panes, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if stopped, err := s.Stop(context.Background(), id); err != nil || stopped.State != Stopped {
		t.Fatalf("stop after split: %+v %v", stopped, err)
	}
	waitFile(t, cfg.Env["DEVORBIT_TEST_MARKER"])
}

func TestNoisyTerminalKeepsDrainingWithoutViewer(t *testing.T) {
	s := New(50 * time.Millisecond)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	id, workspace := uuid.New(), uuid.New()
	cfg := helperConfig(t, "noisy")
	cfg.WorkspaceID = workspace
	status, err := s.Start(id, cfg)
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, s, id, Exited)
	terminal, err := s.Terminal(id, workspace, status.ExecutionID)
	if err != nil {
		t.Fatal(err)
	}
	frames, gap, _, cancel := terminal.Subscribe(0)
	defer cancel()
	if !gap || len(frames) == 0 {
		t.Fatalf("expected bounded replay gap: frames=%d gap=%v", len(frames), gap)
	}
}

func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("helper did not create %s", path)
}

func TestStopAndRestart(t *testing.T) {
	s := New(100 * time.Millisecond)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	id := uuid.New()
	if status, err := s.Stop(context.Background(), id); err != nil || status.State != Stopped {
		t.Fatalf("stop never started: %+v %v", status, err)
	}
	cfg := helperConfig(t, "wait")
	first, err := s.Start(id, cfg)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Restart(context.Background(), id, configLoader(cfg))
	if err != nil || second.State != Running || *second.PID == *first.PID {
		t.Fatalf("restart: %+v %v", second, err)
	}
	stopped, err := s.Stop(context.Background(), id)
	if err != nil || stopped.State != Stopped || stopped.ExitedAt == nil || stopped.ExitCode == nil || stopped.LastError != "" {
		t.Fatalf("stop: %+v %v", stopped, err)
	}
	again, err := s.Stop(context.Background(), id)
	if err != nil || again.State != Stopped {
		t.Fatalf("idempotent stop: %+v %v", again, err)
	}
	third, err := s.Restart(context.Background(), id, configLoader(cfg))
	if err != nil || third.State != Running || *third.PID == *second.PID {
		t.Fatalf("restart stopped: %+v %v", third, err)
	}
	originalCommand := cfg.Command
	cfg.Command = "missing-devorbit-executable"
	if _, err := s.Restart(context.Background(), id, configLoader(cfg)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("failed replacement: %v", err)
	}
	if status := s.Status(id); status.State != Failed || status.PID != nil || status.LastError == "" {
		t.Fatalf("failed replacement status: %+v", status)
	}
	cfg.Command = originalCommand
	cfg.Env["DEVORBIT_HELPER"] = "exit"
	if _, err := s.Restart(context.Background(), id, configLoader(cfg)); err != nil {
		t.Fatalf("restart failed launch: %v", err)
	}
	waitState(t, s, id, Exited)
	cfg.Env["DEVORBIT_HELPER"] = "wait"
	if status, err := s.Restart(context.Background(), id, configLoader(cfg)); err != nil || status.State != Running {
		t.Fatalf("restart natural exit: %+v %v", status, err)
	}
}

func TestRestartLoadsAfterTermination(t *testing.T) {
	s := New(50 * time.Millisecond)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	id := uuid.New()
	cfg := helperConfig(t, "wait")
	if _, err := s.Start(id, cfg); err != nil {
		t.Fatal(err)
	}
	loaded := false
	status, err := s.Restart(context.Background(), id, func(context.Context) (Config, error) {
		loaded = true
		if old := s.Status(id); old.State != Stopped || old.ExitedAt == nil {
			t.Fatalf("configuration loaded before old process ended: %+v", old)
		}
		cfg.Env["DEVORBIT_HELPER"] = "exit"
		return cfg, nil
	})
	if err != nil || !loaded || status.State != Running {
		t.Fatalf("restart: %+v %v", status, err)
	}
	waitState(t, s, id, Exited)
}

func TestGracefulAndForcedStop(t *testing.T) {
	s := New(80 * time.Millisecond)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	if runtime.GOOS != "windows" {
		graceID := uuid.New()
		grace := helperConfig(t, "graceful")
		grace.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
		grace.Env["DEVORBIT_TEST_MARKER"] = filepath.Join(t.TempDir(), "marker")
		if _, err := s.Start(graceID, grace); err != nil {
			t.Fatal(err)
		}
		waitFile(t, grace.Env["DEVORBIT_TEST_READY"])
		if status, err := s.Stop(context.Background(), graceID); err != nil || status.State != Stopped {
			t.Fatalf("graceful stop: %+v %v", status, err)
		}
		waitFile(t, grace.Env["DEVORBIT_TEST_MARKER"])
	}
	forcedID := uuid.New()
	forced := helperConfig(t, "ignore")
	forced.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
	if _, err := s.Start(forcedID, forced); err != nil {
		t.Fatal(err)
	}
	waitFile(t, forced.Env["DEVORBIT_TEST_READY"])
	start := time.Now()
	status, err := s.Stop(context.Background(), forcedID)
	if err != nil || status.State != Stopped || runtime.GOOS != "windows" && time.Since(start) < 60*time.Millisecond {
		t.Fatalf("forced stop: %+v %v", status, err)
	}
}

func TestConcurrentLifecycle(t *testing.T) {
	s := New(40 * time.Millisecond)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	id := uuid.New()
	cfg := helperConfig(t, "wait")
	if _, err := s.Start(id, cfg); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Stop(context.Background(), id)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent stop: %v", err)
		}
	}
	if status := s.Status(id); status.State != Stopped {
		t.Fatalf("stop status: %+v", status)
	}
	if _, err := s.Start(id, cfg); err != nil {
		t.Fatal(err)
	}
	results = make(chan error, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Restart(context.Background(), id, configLoader(cfg))
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil && !errors.Is(err, ErrActive) {
			t.Fatalf("concurrent restart: %v", err)
		}
	}
	if status := s.Status(id); status.State != Running {
		t.Fatalf("restart status: %+v", status)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restart(ctx, id, configLoader(cfg)); !errors.Is(err, ErrClosed) {
		t.Fatalf("restart after shutdown: %v", err)
	}
}

func TestStopAfterNaturalCompletion(t *testing.T) {
	s := New(time.Second)
	if err := s.stopProcess(context.Background(), &entry{done: make(chan struct{})}); err != nil {
		t.Fatalf("completed process stop: %v", err)
	}
}

func TestLifecycleConflictsAndShutdown(t *testing.T) {
	s := New(100 * time.Millisecond)
	id := uuid.New()
	cfg := helperConfig(t, "ignore")
	cfg.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
	if _, err := s.Start(id, cfg); err != nil {
		t.Fatal(err)
	}
	waitFile(t, cfg.Env["DEVORBIT_TEST_READY"])
	stopped := make(chan error, 1)
	go func() { _, err := s.Stop(context.Background(), id); stopped <- err }()
	waitState(t, s, id, Stopping)
	if _, err := s.Start(id, cfg); !errors.Is(err, ErrActive) {
		t.Fatalf("start during stop: %v", err)
	}
	if _, err := s.Restart(context.Background(), id, configLoader(cfg)); !errors.Is(err, ErrActive) {
		t.Fatalf("restart during stop: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdown := make(chan error, 1)
	go func() { shutdown <- s.Shutdown(ctx) }()
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if err := <-shutdown; err != nil {
		t.Fatal(err)
	}
	if status := s.Status(id); status.State != Stopped {
		t.Fatalf("shutdown status: %+v", status)
	}
	if _, err := s.Start(id, cfg); !errors.Is(err, ErrClosed) {
		t.Fatalf("start after shutdown: %v", err)
	}
}

func helperConfig(t *testing.T, mode string) Config {
	t.Helper()
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	return Config{Command: self, Args: []string{"-test.run=^TestProcessHelper$"}, Dir: t.TempDir(), Env: map[string]string{"DEVORBIT_HELPER": mode}}
}

func configLoader(cfg Config) func(context.Context) (Config, error) {
	return func(context.Context) (Config, error) { return cfg, nil }
}

func waitState(t *testing.T, s *Supervisor, id uuid.UUID, state State) Status {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := s.Status(id)
		if status.State == state {
			return status
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status did not become %s: %+v", state, s.Status(id))
	return Status{}
}

func TestLifecycleAndRetry(t *testing.T) {
	s := New(100 * time.Millisecond)
	id := uuid.New()
	if status := s.Status(id); status.State != Stopped || status.PID != nil {
		t.Fatalf("initial status: %+v", status)
	}
	cfg := helperConfig(t, "exit")
	status, err := s.Start(id, cfg)
	if err != nil || status.State != Running || status.PID == nil || status.StartedAt == nil {
		t.Fatalf("start: %+v %v", status, err)
	}
	exited := waitState(t, s, id, Exited)
	if exited.ExitCode == nil || *exited.ExitCode != 0 || exited.ExitedAt == nil {
		t.Fatalf("exit: %+v", exited)
	}
	*exited.PID = -1
	if *s.Status(id).PID == -1 {
		t.Fatal("status exposed internal pointer")
	}
	cfg.Env["DEVORBIT_HELPER"] = "fail"
	if _, err := s.Start(id, cfg); err != nil {
		t.Fatal(err)
	}
	failed := waitState(t, s, id, Failed)
	if failed.ExitCode == nil || *failed.ExitCode != 7 || failed.LastError == "" {
		t.Fatalf("failed exit: %+v", failed)
	}
	cfg.Command = "missing-devorbit-executable"
	if _, err := s.Start(id, cfg); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing executable: %v", err)
	}
	if status := s.Status(id); status.State != Failed || status.PID != nil || status.LastError == "" {
		t.Fatalf("failed launch: %+v", status)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentStartsAndShutdown(t *testing.T) {
	s := New(100 * time.Millisecond)
	id := uuid.New()
	cfg := helperConfig(t, "wait")
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Start(id, cfg)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	started, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			started++
		case errors.Is(err, ErrActive):
			conflicts++
		default:
			t.Fatalf("unexpected start error: %v", err)
		}
	}
	if started != 1 || conflicts != 19 || s.Status(id).State != Running {
		t.Fatalf("starts=%d conflicts=%d status=%+v", started, conflicts, s.Status(id))
	}
	other := uuid.New()
	if _, err := s.Start(other, cfg); err != nil {
		t.Fatalf("different service: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	for _, serviceID := range []uuid.UUID{id, other} {
		if status := s.Status(serviceID); status.State != Stopped || status.ExitedAt == nil {
			t.Fatalf("shutdown status: %+v", status)
		}
	}
	if _, err := s.Start(uuid.New(), cfg); !errors.Is(err, ErrClosed) {
		t.Fatalf("start after shutdown: %v", err)
	}
}

func TestEnvironmentAndWorkingDirectoryResolution(t *testing.T) {
	s := New(100 * time.Millisecond)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	self, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "env.txt")
	t.Setenv("DEVORBIT_TEST_VALUE", "inherited")
	cfg := helperConfig(t, "env")
	cfg.Dir = filepath.Dir(self)
	cfg.Command = filepath.Base(self)
	cfg.Env["PATH"] = "."
	cfg.Env["DEVORBIT_TEST_VALUE"] = "override"
	cfg.Env["DEVORBIT_TEST_OUT"] = out
	id := uuid.New()
	if _, err := s.Start(id, cfg); err != nil {
		t.Fatal(err)
	}
	if state := waitState(t, s, id, Exited); state.ExitCode == nil || *state.ExitCode != 0 {
		t.Fatalf("env helper: %+v", state)
	}
	content, err := os.ReadFile(out)
	if err != nil || string(content) != "override" || os.Getenv("DEVORBIT_TEST_VALUE") != "inherited" {
		t.Fatalf("environment override: %q %v", content, err)
	}
	for _, invalid := range []Config{
		{Command: self, Dir: filepath.Join(t.TempDir(), "missing")},
		{Command: self, Dir: cfg.Dir, Env: map[string]string{"BAD=KEY": "x"}},
		{Command: self, Dir: cfg.Dir, Args: []string{"a\x00b"}},
	} {
		if _, err := s.Start(uuid.New(), invalid); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid config accepted: %v", err)
		}
	}
}

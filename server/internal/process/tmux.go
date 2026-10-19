//go:build !windows

package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type tmuxSession struct {
	path   string
	socket string
	dir    string
	name   string
	pane   string
}

func startTmux(service *exec.Cmd, serviceName string) (*tmuxSession, *exec.Cmd, error) {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return nil, nil, invalid("tmux is required for service terminals")
	}
	dir, err := os.MkdirTemp("", "devorbit-tmux-")
	if err != nil {
		return nil, nil, invalid("terminal setup failed")
	}
	name := strings.Trim(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, serviceName), "-")
	if name == "" {
		name = "service"
	}
	t := &tmuxSession{path: path, socket: filepath.Join(dir, "socket"), dir: dir, name: name}
	// Positional arguments keep configured commands and values out of shell parsing.
	// Ctrl+C also reaches this wrapper, so catch it until the service exits and the shell takes over.
	script := `start=$1; result=$2; ack=$3; shift 3; while [ ! -e "$start" ]; do sleep .02; done; trap ':' INT; "$@"; code=$?; printf '%s\n' "$code" > "$result"; while [ ! -e "$ack" ]; do sleep .02; done; exec "${SHELL:-/bin/sh}" -i`
	args := []string{"-f", "/dev/null", "-S", t.socket, "new-session", "-d", "-s", t.name, "-c", service.Dir, "-x", "100", "-y", "30", "--", "/bin/sh", "-c", script, "devorbit-service", t.startPath(), t.resultPath(), t.ackPath(), service.Path}
	args = append(args, service.Args[1:]...)
	create := exec.Command(path, args...)
	create.Env = append(service.Env, "TMUX=")
	create.Dir = service.Dir
	if err := create.Run(); err != nil {
		_ = os.RemoveAll(dir)
		return nil, nil, invalid("tmux session launch failed")
	}
	if err := exec.Command(path, "-S", t.socket, "set-option", "-t", t.name, "status", "off").Run(); err != nil {
		t.cleanup()
		return nil, nil, invalid("tmux terminal setup failed")
	}
	if err := exec.Command(path, "-S", t.socket, "set-option", "-t", t.name, "mouse", "on").Run(); err != nil {
		t.cleanup()
		return nil, nil, invalid("tmux terminal setup failed")
	}
	pane, err := exec.Command(path, "-S", t.socket, "display-message", "-p", "-t", t.name, "#{pane_id}").Output()
	if err != nil {
		t.cleanup()
		return nil, nil, invalid("tmux terminal setup failed")
	}
	t.pane = strings.TrimSpace(string(pane))
	attach := exec.Command(path, "-S", t.socket, "attach-session", "-t", t.name)
	attach.Env = append(service.Env, "TMUX=", "TERM=xterm-256color")
	attach.Dir = service.Dir
	return t, attach, nil
}

func (t *tmuxSession) startPath() string  { return filepath.Join(t.dir, "start") }
func (t *tmuxSession) resultPath() string { return filepath.Join(t.dir, "result") }
func (t *tmuxSession) ackPath() string    { return filepath.Join(t.dir, "ack") }

func (t *tmuxSession) start() error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		clients, err := exec.Command(t.path, "-S", t.socket, "list-clients", "-F", "#{client_pid}").Output()
		if err == nil && len(clients) > 0 {
			return os.WriteFile(t.startPath(), nil, 0600)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return invalid("tmux client did not attach")
}

func (t *tmuxSession) panePID() (int, error) {
	output, err := exec.Command(t.path, "-S", t.socket, "display-message", "-p", "-t", t.pane, "#{pane_pid}").Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(output)))
}

func (t *tmuxSession) result() (int, bool) {
	data, err := os.ReadFile(t.resultPath())
	if err != nil {
		return 0, false
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return code, err == nil
}

func (t *tmuxSession) acknowledge() error {
	return os.WriteFile(t.ackPath(), nil, 0600)
}

func (t *tmuxSession) interrupt() error {
	cmd := exec.Command(t.path, "-S", t.socket, "send-keys", "-t", t.pane, "C-c")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux interrupt: %w", err)
	}
	return nil
}

func (t *tmuxSession) kill() {
	_ = exec.Command(t.path, "-S", t.socket, "kill-server").Run()
}

func (t *tmuxSession) cleanup() { t.kill(); _ = os.RemoveAll(t.dir) }

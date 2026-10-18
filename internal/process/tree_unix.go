//go:build !windows

package process

import (
	"errors"
	"os/exec"
	"sync"
	"syscall"
)

type processTree struct {
	mu     sync.Mutex
	pid    int
	closed bool
}

func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &processTree{}, nil
}

func (t *processTree) attach(cmd *exec.Cmd) error {
	t.mu.Lock()
	t.pid = cmd.Process.Pid
	t.mu.Unlock()
	return nil
}

func (t *processTree) interrupt() error { return t.signal(syscall.SIGINT) }
func (t *processTree) kill() error      { return t.signal(syscall.SIGKILL) }

func (t *processTree) signal(sig syscall.Signal) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	err := syscall.Kill(-t.pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func (t *processTree) close() {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
}

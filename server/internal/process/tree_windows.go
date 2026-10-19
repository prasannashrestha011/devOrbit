//go:build windows

package process

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

var consoleControl = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

type processTree struct {
	ctx    context.Context
	cancel context.CancelFunc
	pid    int
}

func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	ctx, cancel := context.WithCancel(context.Background())
	return &processTree{ctx: ctx, cancel: cancel}, nil
}

func (t *processTree) context() context.Context { return t.ctx }

func (t *processTree) attach(cmd *exec.Cmd) error {
	t.pid = cmd.Process.Pid
	return nil
}

func (t *processTree) interrupt() error {
	r, _, _ := consoleControl.Call(1, uintptr(t.pid)) // CTRL_BREAK_EVENT
	if r == 0 {
		return errGraceUnavailable
	}
	return nil
}

func (t *processTree) kill() error { t.cancel(); return nil }

func (t *processTree) close() { t.cancel() }

func waitProcess(cmd *exec.Cmd) error {
	state, err := cmd.Process.Wait()
	cmd.ProcessState = state
	if err == nil && !state.Success() {
		return fmt.Errorf("process exited with code %d", state.ExitCode())
	}
	return err
}

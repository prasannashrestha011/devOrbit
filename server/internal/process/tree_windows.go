//go:build windows

package process

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var createJobObject = kernel32.NewProc("CreateJobObjectW")
var setJobInformation = kernel32.NewProc("SetInformationJobObject")
var assignToJob = kernel32.NewProc("AssignProcessToJobObject")
var terminateJob = kernel32.NewProc("TerminateJobObject")
var consoleControl = kernel32.NewProc("GenerateConsoleCtrlEvent")

type jobLimits struct {
	ProcessTime int64
	JobTime     int64
	Flags       uint32
	MinWorking  uintptr
	MaxWorking  uintptr
	MaxActive   uint32
	Affinity    uintptr
	Priority    uint32
	Scheduling  uint32
	IO          [6]uint64
	ProcessMem  uintptr
	JobMem      uintptr
	PeakProcess uintptr
	PeakJob     uintptr
}

type processTree struct {
	mu     sync.Mutex
	job    syscall.Handle
	pid    int
	closed bool
}

func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	h, _, err := createJobObject.Call(0, 0)
	if h == 0 {
		return nil, err
	}
	t := &processTree{job: syscall.Handle(h)}
	limits := jobLimits{Flags: 0x2000} // JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	r, _, err := setJobInformation.Call(h, 9, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if r == 0 {
		t.close()
		return nil, err
	}
	return t, nil
}

func (t *processTree) attach(cmd *exec.Cmd) error {
	// shortcut: Job assignment follows process creation; use atomic job assignment if early descendants must be contained.
	h, err := syscall.OpenProcess(0x0100|0x0001, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	t.mu.Lock()
	defer t.mu.Unlock()
	r, _, err := assignToJob.Call(uintptr(t.job), uintptr(h))
	if r == 0 {
		return err
	}
	t.pid = cmd.Process.Pid
	return nil
}

func (t *processTree) interrupt() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	r, _, _ := consoleControl.Call(1, uintptr(t.pid)) // CTRL_BREAK_EVENT
	if r == 0 {
		// Console control is unavailable without a shared console; force the owned job.
		return errGraceUnavailable
	}
	return nil
}

func (t *processTree) kill() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	r, _, err := terminateJob.Call(uintptr(t.job), 1)
	if r == 0 {
		return err
	}
	return nil
}

func (t *processTree) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		_ = syscall.CloseHandle(t.job)
		t.closed = true
	}
}

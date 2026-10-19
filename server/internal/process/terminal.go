package process

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/phuslu/pty"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
)

const terminalReplayLimit = 1 << 20

var ErrStaleTerminal = errors.New("terminal execution is no longer active")
var ErrTerminalBusy = errors.New("terminal input queue is full")

type TerminalFrame struct {
	Type     string `json:"type"`
	Sequence uint64 `json:"sequence,omitempty"`
	Data     []byte `json:"data,omitempty"`
}

type Terminal struct {
	pty     pty.Pty
	capture *logs.Capture
	mu      sync.Mutex
	seq     uint64
	replay  []TerminalFrame
	bytes   int
	views   map[chan TerminalFrame]struct{}
	input   chan []byte
	closed  chan struct{}
	done    chan struct{}
	once    sync.Once
}

func newTerminal(ptmx pty.Pty, capture *logs.Capture) *Terminal {
	t := &Terminal{pty: ptmx, capture: capture, views: make(map[chan TerminalFrame]struct{}), input: make(chan []byte, 64), closed: make(chan struct{}), done: make(chan struct{})}

	// channel waits for user terminal input; executes when input is passed
	go func() {
		for {
			select {
			case data := <-t.input:
				_, _ = t.pty.Write(data)
			case <-t.closed:
				return
			}
		}
	}()
	return t
}

func (t *Terminal) drain() {
	defer close(t.done)
	buf := make([]byte, 16<<10)
	for {
		n, err := t.pty.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			t.mu.Lock()
			if t.capture != nil {
				_, _ = t.capture.Terminal.Write(chunk)
			}
			t.seq++
			frame := TerminalFrame{Type: "output", Sequence: t.seq, Data: chunk}
			t.replay = append(t.replay, frame)
			t.bytes += n
			for t.bytes > terminalReplayLimit {
				t.bytes -= len(t.replay[0].Data)
				t.replay[0] = TerminalFrame{}
				t.replay = t.replay[1:]
			}
			for view := range t.views {
				select {
				case view <- frame:
				default:
					close(view)
					delete(t.views, view)
				}
			}
			t.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (t *Terminal) stopCapture() {
	t.mu.Lock()
	t.capture = nil
	t.mu.Unlock()
}

func (t *Terminal) close() {
	t.once.Do(func() {
		close(t.closed)
		select {
		case <-t.done:
		case <-time.After(2 * time.Second):
			_ = t.pty.Close()
			<-t.done
		}
		_ = t.pty.Close()
		t.mu.Lock()
		for view := range t.views {
			close(view)
			delete(t.views, view)
		}
		t.mu.Unlock()
	})
}

func (t *Terminal) Subscribe(after uint64) ([]TerminalFrame, bool, <-chan TerminalFrame, func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	gap := len(t.replay) > 0 && after+1 < t.replay[0].Sequence
	frames := make([]TerminalFrame, 0, len(t.replay))
	for _, frame := range t.replay {
		if frame.Sequence > after {
			frames = append(frames, frame)
		}
	}
	view := make(chan TerminalFrame, 64)
	select {
	case <-t.closed:
		close(view)
	default:
		t.views[view] = struct{}{}
	}
	return frames, gap, view, func() {
		t.mu.Lock()
		if _, ok := t.views[view]; ok {
			delete(t.views, view)
			close(view)
		}
		t.mu.Unlock()
	}
}

func (t *Terminal) Closed() bool {
	select {
	case <-t.closed:
		return true
	default:
		return false
	}
}

func (s *Supervisor) Terminal(serviceID, workspaceID, executionID uuid.UUID) (*Terminal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entries[serviceID]
	if e == nil || e.workspaceID != workspaceID || e.status.ExecutionID != executionID || e.terminal == nil {
		return nil, ErrStaleTerminal
	}
	return e.terminal, nil
}

func (s *Supervisor) TerminalInput(serviceID, workspaceID, executionID uuid.UUID, data []byte) error {
	if len(data) == 0 || len(data) > 4096 {
		return ErrInvalid
	}
	s.mu.Lock()
	e := s.entries[serviceID]
	if e == nil || e.workspaceID != workspaceID || e.status.ExecutionID != executionID || e.terminal == nil || e.tmux == nil && e.status.State != Running || e.terminal.Closed() {
		s.mu.Unlock()
		return ErrStaleTerminal
	}
	t := e.terminal
	s.mu.Unlock()
	select {
	case t.input <- append([]byte(nil), data...):
		return nil
	default:
		return ErrTerminalBusy
	}
}

func (s *Supervisor) TerminalResize(serviceID, workspaceID, executionID uuid.UUID, cols, rows uint16) error {
	if cols == 0 || rows == 0 || cols > 500 || rows > 300 {
		return ErrInvalid
	}
	s.mu.Lock()
	e := s.entries[serviceID]
	if e == nil || e.workspaceID != workspaceID || e.status.ExecutionID != executionID || e.terminal == nil || e.tmux == nil && e.status.State != Running || e.terminal.Closed() {
		s.mu.Unlock()
		return ErrStaleTerminal
	}
	t := e.terminal
	s.mu.Unlock()
	return pty.SetSize(t.pty, &pty.Winsize{Cols: cols, Rows: rows})
}

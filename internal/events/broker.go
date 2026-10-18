package events

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	ProcessStarted   Type = "process.started"
	ProcessStopping  Type = "process.stopping"
	ProcessStopped   Type = "process.stopped"
	ProcessExited    Type = "process.exited"
	ProcessFailed    Type = "process.failed"
	ProcessRestarted Type = "process.restarted"
	LogCreated       Type = "log.created"
)

type Event struct {
	ID          string     `json:"id"`
	Type        Type       `json:"type"`
	Timestamp   time.Time  `json:"timestamp"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	ServiceID   uuid.UUID  `json:"service_id"`
	ExecutionID *uuid.UUID `json:"execution_id,omitempty"`
	Data        any        `json:"data"`
}

type Filter struct {
	WorkspaceID uuid.UUID
	ServiceID   uuid.UUID
}

type Broker struct {
	mu          sync.Mutex
	subscribers map[chan Event]Filter
	closed      bool
}

func New() *Broker { return &Broker{subscribers: make(map[chan Event]Filter)} }

func (b *Broker) Publish(event Event) {
	if b == nil {
		return
	}
	event.ID = uuid.NewString()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	for ch, filter := range b.subscribers {
		if filter.WorkspaceID != uuid.Nil && filter.WorkspaceID != event.WorkspaceID || filter.ServiceID != uuid.Nil && filter.ServiceID != event.ServiceID {
			continue
		}
		select {
		case ch <- event:
		default:
			// A slow stream must reconnect; process output cannot wait for its reader.
			delete(b.subscribers, ch)
			close(ch)
		}
	}
}

func (b *Broker) Subscribe(filter Filter) (<-chan Event, func()) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	if b.closed {
		close(ch)
	} else {
		b.subscribers[ch] = filter
	}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if _, ok := b.subscribers[ch]; ok {
			delete(b.subscribers, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
}

func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subscribers {
		close(ch)
		delete(b.subscribers, ch)
	}
}

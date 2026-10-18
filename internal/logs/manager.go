package logs

import (
	"bufio"
	"bytes"
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	"github.com/prasannashrestha011/devOrbit/internal/security"
)

const (
	maxEntries    = 500
	maxBytes      = 2 << 20
	maxChunk      = 16 << 10
	maxFileBytes  = 10 << 20
	maxFiles      = 5
	maxExecutions = 20
	retention     = 7 * 24 * time.Hour
)

var ErrClosed = errors.New("log manager is closed")

type Entry struct {
	ID          string    `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	ServiceID   uuid.UUID `json:"service_id"`
	ExecutionID uuid.UUID `json:"execution_id"`
	Stream      string    `json:"stream"`
	Timestamp   time.Time `json:"timestamp"`
	Sequence    uint64    `json:"sequence"`
	Message     string    `json:"message"`
}

type Diagnostics struct {
	DroppedEntries uint64 `json:"dropped_entries"`
	StorageErrors  uint64 `json:"storage_errors"`
}

type Filter struct {
	WorkspaceID uuid.UUID
	ServiceID   uuid.UUID
	Before      string
	Limit       int
}

type serviceBuffer struct {
	entries []Entry
	bytes   int
	diag    Diagnostics
}

type Manager struct {
	root   string
	events *events.Broker
	mu     sync.Mutex
	closed bool
	active map[uuid.UUID]uuid.UUID // execution ID -> service ID
	buffer map[uuid.UUID]*serviceBuffer
	wg     sync.WaitGroup
}

func New(root string, broker ...*events.Broker) (*Manager, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if err := security.PrivateDirectory(root); err != nil {
		return nil, errors.New("log directory must be private")
	}
	root = filepath.Join(root, "services")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if err := security.PrivateDirectory(root); err != nil {
		return nil, errors.New("log directory must be private")
	}
	m := &Manager{root: root, active: make(map[uuid.UUID]uuid.UUID), buffer: make(map[uuid.UUID]*serviceBuffer)}
	if len(broker) > 0 {
		m.events = broker[0]
	}
	services, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, service := range services {
		if service.IsDir() {
			if id, err := uuid.Parse(service.Name()); err == nil {
				if err := m.prune(id); err != nil {
					return nil, err
				}
			}
		}
	}
	return m, nil
}

func (m *Manager) Begin(workspaceID, serviceID uuid.UUID) (*Capture, error) {
	if workspaceID == uuid.Nil || serviceID == uuid.Nil {
		return nil, errors.New("workspace and service IDs are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	if err := m.prune(serviceID); err != nil {
		return nil, err
	}
	executionID := uuid.New()
	dir := filepath.Join(m.root, serviceID.String(), executionID.String())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "output-0001.jsonl"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	c := &Capture{manager: m, workspaceID: workspaceID, serviceID: serviceID, executionID: executionID, dir: dir, file: file, fileNumber: 1, queue: make(chan pending, 1024), done: make(chan struct{})}
	c.Stdout = &streamWriter{capture: c, stream: "stdout"}
	c.Stderr = &streamWriter{capture: c, stream: "stderr"}
	m.active[executionID] = serviceID
	m.wg.Add(1)
	go c.run()
	return c, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.wg.Wait()
	return nil
}

func (m *Manager) Diagnostics(serviceID uuid.UUID) Diagnostics {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b := m.buffer[serviceID]; b != nil {
		return b.diag
	}
	return Diagnostics{}
}

func (m *Manager) Recent(serviceID uuid.UUID) []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b := m.buffer[serviceID]; b != nil {
		return append([]Entry(nil), b.entries...)
	}
	return nil
}

func (m *Manager) add(e Entry) {
	m.mu.Lock()
	b := m.buffer[e.ServiceID]
	if b == nil {
		b = &serviceBuffer{}
		m.buffer[e.ServiceID] = b
	}
	b.entries = append(b.entries, e)
	b.bytes += len(e.Message)
	for len(b.entries) > maxEntries || b.bytes > maxBytes {
		b.bytes -= len(b.entries[0].Message)
		b.entries[0] = Entry{}
		b.entries = b.entries[1:]
	}
	m.mu.Unlock()
	m.events.Publish(events.Event{Type: events.LogCreated, Timestamp: e.Timestamp, WorkspaceID: e.WorkspaceID, ServiceID: e.ServiceID, ExecutionID: &e.ExecutionID, Data: map[string]any{
		"stream": e.Stream, "sequence": e.Sequence, "message": e.Message,
	}})
}

func (m *Manager) diagnostic(serviceID uuid.UUID, dropped, storage uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b := m.buffer[serviceID]
	if b == nil {
		b = &serviceBuffer{}
		m.buffer[serviceID] = b
	}
	b.diag.DroppedEntries += dropped
	b.diag.StorageErrors += storage
}

type pending struct {
	stream  string
	message string
}

type Capture struct {
	Stdout, Stderr io.Writer
	manager        *Manager
	workspaceID    uuid.UUID
	serviceID      uuid.UUID
	executionID    uuid.UUID
	dir            string
	file           *os.File
	fileNumber     int
	fileSize       int64
	sequence       uint64
	queue          chan pending
	done           chan struct{}
	closeOnce      sync.Once
	writeErr       error
}

func (c *Capture) ExecutionID() uuid.UUID { return c.executionID }

func (c *Capture) enqueue(p pending) {
	select {
	case c.queue <- p:
	default:
		c.manager.diagnostic(c.serviceID, 1, 0)
	}
}

func (c *Capture) run() {
	defer close(c.done)
	defer c.manager.wg.Done()
	for p := range c.queue {
		c.sequence++
		now := time.Now().UTC()
		e := Entry{
			ID:          fmt.Sprintf("%020d-%s-%012d", now.UnixNano(), c.executionID, c.sequence),
			WorkspaceID: c.workspaceID, ServiceID: c.serviceID, ExecutionID: c.executionID,
			Stream: p.stream, Timestamp: now, Sequence: c.sequence, Message: p.message,
		}
		c.manager.add(e)
		if c.writeErr == nil {
			if err := c.persist(e); err != nil {
				c.writeErr = err
				c.manager.diagnostic(c.serviceID, 0, 1)
			}
		}
	}
	if c.file != nil {
		if err := c.file.Close(); err != nil {
			c.writeErr = err
			c.manager.diagnostic(c.serviceID, 0, 1)
		}
	}
	_ = os.Chtimes(c.dir, time.Now(), time.Now())
	c.manager.mu.Lock()
	delete(c.manager.active, c.executionID)
	c.manager.mu.Unlock()
}

func (c *Capture) persist(e Entry) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if c.fileSize > 0 && c.fileSize+int64(len(data)) > maxFileBytes {
		if err := c.file.Close(); err != nil {
			return err
		}
		c.fileNumber++
		c.file, err = os.OpenFile(filepath.Join(c.dir, fmt.Sprintf("output-%04d.jsonl", c.fileNumber)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		c.fileSize = 0
		if c.fileNumber > maxFiles {
			if err := os.Remove(filepath.Join(c.dir, fmt.Sprintf("output-%04d.jsonl", c.fileNumber-maxFiles))); err != nil {
				return err
			}
		}
	}
	n, err := c.file.Write(data)
	c.fileSize += int64(n)
	return err
}

func (c *Capture) Close() error {
	c.closeOnce.Do(func() {
		c.Stdout.(*streamWriter).close()
		c.Stderr.(*streamWriter).close()
		close(c.queue)
	})
	<-c.done
	return c.writeErr
}

type streamWriter struct {
	mu      sync.Mutex
	capture *Capture
	stream  string
	partial []byte
	closed  bool
}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	n := len(p)
	for len(p) > 0 {
		end := len(p)
		if end > maxChunk-len(w.partial) {
			end = maxChunk - len(w.partial)
		}
		if i := bytes.IndexByte(p[:end], '\n'); i >= 0 {
			end = i + 1
		}
		w.partial = append(w.partial, p[:end]...)
		p = p[end:]
		if len(w.partial) == maxChunk {
			w.emit()
		} else if w.partial[len(w.partial)-1] == '\n' {
			w.capture.enqueue(pending{w.stream, string(w.partial)})
			w.partial = w.partial[:0]
		}
	}
	w.emit()
	return n, nil
}

func (w *streamWriter) emit() {
	cut := len(w.partial)
	for i := cut - 1; i >= 0 && i >= cut-3; i-- {
		if utf8.RuneStart(w.partial[i]) {
			if !utf8.FullRune(w.partial[i:]) {
				cut = i
			}
			break
		}
	}
	if cut > 0 {
		w.capture.enqueue(pending{w.stream, string(w.partial[:cut])})
		w.partial = append(w.partial[:0], w.partial[cut:]...)
	}
}

func (w *streamWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.partial) > 0 {
		w.capture.enqueue(pending{w.stream, string(w.partial)})
		w.partial = nil
	}
	w.closed = true
}

func (m *Manager) prune(serviceID uuid.UUID) error {
	dir := filepath.Join(m.root, serviceID.String())
	info, statErr := os.Lstat(dir)
	if errors.Is(statErr, os.ErrNotExist) {
		return nil
	}
	if statErr != nil || !info.IsDir() {
		return errors.New("service log directory is not a directory")
	}
	items, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	type item struct {
		name     string
		modified time.Time
	}
	var kept []item
	for _, e := range items {
		id, err := uuid.Parse(e.Name())
		if err != nil || !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		if _, active := m.active[id]; !active && time.Since(info.ModTime()) > retention {
			if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
			continue
		}
		kept = append(kept, item{e.Name(), info.ModTime()})
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].modified.After(kept[j].modified) })
	if len(kept) < maxExecutions {
		return nil
	}
	for _, e := range kept[maxExecutions-1:] {
		id, _ := uuid.Parse(e.name)
		if _, active := m.active[id]; !active {
			if err := os.RemoveAll(filepath.Join(dir, e.name)); err != nil {
				return err
			}
		}
	}
	return nil
}

type entryHeap []Entry

func (h entryHeap) Len() int           { return len(h) }
func (h entryHeap) Less(i, j int) bool { return h[i].ID < h[j].ID }
func (h entryHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *entryHeap) Push(x any)        { *h = append(*h, x.(Entry)) }
func (h *entryHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// List returns newest entries first; Before is the ID of the oldest entry on the previous page.
func (m *Manager) List(ctx context.Context, f Filter) ([]Entry, error) {
	if f.Limit < 1 || f.Limit > 500 || len(f.Before) > 128 {
		return nil, errors.New("invalid log query")
	}
	root := m.root
	if f.ServiceID != uuid.Nil {
		root = filepath.Join(root, f.ServiceID.String())
	}
	h := &entryHeap{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil || ctx.Err() != nil {
			return errors.Join(err, ctx.Err())
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64<<10), 1<<20)
		for scanner.Scan() {
			if err := ctx.Err(); err != nil {
				_ = file.Close()
				return err
			}
			var e Entry
			if json.Unmarshal(scanner.Bytes(), &e) != nil || e.ID == "" {
				continue // A crash can leave the final JSONL record incomplete.
			}
			if f.ServiceID != uuid.Nil && e.ServiceID != f.ServiceID || f.WorkspaceID != uuid.Nil && e.WorkspaceID != f.WorkspaceID || f.Before != "" && e.ID >= f.Before {
				continue
			}
			if h.Len() < f.Limit {
				heap.Push(h, e)
			} else if e.ID > (*h)[0].ID {
				(*h)[0] = e
				heap.Fix(h, 0)
			}
		}
		scanErr := scanner.Err()
		return errors.Join(scanErr, file.Close())
	})
	if err != nil {
		return nil, err
	}
	result := make([]Entry, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(Entry)
	}
	return result, nil
}

package traffic_inspector

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
)

var ErrDuplicate = errors.New("inspection already active for this target")
var ErrClosed = errors.New("traffic inspector is shutting down")
var ErrNotFound = errors.New("inspection session not found")

type session struct {
	info       SessionInfo
	listener   net.Listener
	server     *http.Server
	transport  *http.Transport
	records    []Record
	sequence   uint64
	mu         sync.Mutex
	generation uint64
}

type Manager struct {
	mu        sync.RWMutex
	sessions  map[uuid.UUID]*session
	byTarget  map[Target]uuid.UUID
	closed    bool
	broker    *events.Broker
	store     *captureStore
	done      chan struct{}
	closeOnce sync.Once
}

func New(root string, broker *events.Broker) (*Manager, error) {
	m := &Manager{
		sessions: map[uuid.UUID]*session{},
		byTarget: map[Target]uuid.UUID{},
		broker:   broker,
		store:    &captureStore{root: root, limit: captureQuota},
		done:     make(chan struct{}),
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-m.done:
				return
			case <-ticker.C:
				if err := m.prune(); err != nil {
					slog.Error("traffic retention cleanup failed", "error", err)
				}
			}
		}
	}()
	return m, nil
}

func (m *Manager) Start(target Target, upstream *url.URL) (SessionInfo, error) {
	if upstream == nil || upstream.Scheme != "http" || upstream.Hostname() != "127.0.0.1" {
		return SessionInfo{}, errors.New("upstream must be loopback HTTP")
	}
	s := &session{
		info: SessionInfo{
			ID:        uuid.New(),
			Target:    target,
			Upstream:  upstream.String(),
			State:     "starting",
			StartedAt: time.Now().UTC(),
		},
		records: make([]Record, 0, 500),
	}
	// Reserve before opening the listener so concurrent starts cannot duplicate this target.
	if err := m.reserveSession(s); err != nil {
		return SessionInfo{}, err
	}
	if err := os.MkdirAll(m.store.sessionPath(s.info.ID), 0700); err != nil {
		m.remove(s)
		return SessionInfo{}, err
	}
	if err := m.store.put(filepath.Join(m.store.sessionPath(s.info.ID), "session.json"), s.info); err != nil {
		m.remove(s)
		return SessionInfo{}, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		m.remove(s)
		return SessionInfo{}, err
	}
	transport := &http.Transport{
		DisableCompression:    true,
		Proxy:                 nil,
		MaxIdleConnsPerHost:   16,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       60 * time.Second,
	}
	proxy := newReverseProxy(upstream, transport)
	server := &http.Server{
		Handler:           m.recordingHandler(s, proxy),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Stop or Close may have canceled the session while the listener was being created.
	m.mu.Lock()
	if m.closed || m.sessions[s.info.ID] != s {
		m.mu.Unlock()
		_ = listener.Close()
		transport.CloseIdleConnections()
		return SessionInfo{}, ErrClosed
	}
	s.listener = listener
	s.server = server
	s.transport = transport
	s.info.ProxyURL = "http://" + listener.Addr().String()
	s.info.State = "active"
	info := s.info
	m.mu.Unlock()

	go func() {
		err := server.Serve(listener)
		transport.CloseIdleConnections()
		if !errors.Is(err, http.ErrServerClosed) {
			_ = m.archive(s)
		}
	}()
	m.publish(info.Target, events.TrafficSession, map[string]any{"session_id": info.ID, "state": info.State})
	return info, nil
}

func (m *Manager) Get(workspaceID, id uuid.UUID) (SessionInfo, error) {
	m.mu.RLock()
	s := m.sessions[id]
	if s == nil || s.info.Target.WorkspaceID != workspaceID {
		m.mu.RUnlock()
		return SessionInfo{}, ErrNotFound
	}
	info := s.info
	m.mu.RUnlock()
	return info, nil
}

func (m *Manager) List(workspaceID uuid.UUID) []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	items := []SessionInfo{}
	for _, s := range m.sessions {
		if s.info.Target.WorkspaceID == workspaceID && s.info.State != "starting" {
			items = append(items, s.info)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if (items[i].State == "active") != (items[j].State == "active") {
			return items[i].State == "active"
		}
		return items[i].StartedAt.After(items[j].StartedAt)
	})
	return items
}

func (m *Manager) Stop(ctx context.Context, workspaceID, id uuid.UUID) error {
	m.mu.Lock()
	s := m.sessions[id]
	if s == nil || s.info.Target.WorkspaceID != workspaceID {
		m.mu.Unlock()
		return ErrNotFound
	}
	if s.info.State == "stopping" || s.info.State == "stopped" {
		m.mu.Unlock()
		return nil
	}
	s.info.State = "stopping"
	server, transport := s.server, s.transport
	m.mu.Unlock()
	if server == nil {
		return m.archive(s)
	}
	err := server.Shutdown(ctx)
	if err != nil {
		_ = server.Close()
	}
	transport.CloseIdleConnections()
	err = errors.Join(err, m.archive(s))
	m.publish(s.info.Target, events.TrafficSession, map[string]any{"session_id": id, "state": "stopped"})
	return err
}

func (m *Manager) StopWorkspace(ctx context.Context, workspaceID uuid.UUID) {
	for _, item := range m.List(workspaceID) {
		_ = m.Stop(ctx, workspaceID, item.ID)
	}
}

func (m *Manager) StopService(ctx context.Context, workspaceID uuid.UUID, serviceKey string) {
	for _, item := range m.List(workspaceID) {
		if item.Target.Kind == "local" && item.Target.ServiceKey == serviceKey {
			_ = m.Stop(ctx, workspaceID, item.ID)
		}
	}
}

func (m *Manager) Close(ctx context.Context) {
	m.closeOnce.Do(func() { close(m.done) })
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.mu.RLock()
	ids := make([]SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		ids = append(ids, s.info)
	}
	m.mu.RUnlock()
	for _, item := range ids {
		_ = m.Stop(ctx, item.Target.WorkspaceID, item.ID)
	}
}

func (m *Manager) reserveSession(s *session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrClosed
	}
	if _, exists := m.byTarget[s.info.Target]; exists {
		return ErrDuplicate
	}
	for _, existing := range m.sessions {
		if existing.info.State == "active" && existing.info.ProxyURL == s.info.Upstream {
			return errors.New("upstream points to another inspection gateway")
		}
	}
	m.sessions[s.info.ID] = s
	m.byTarget[s.info.Target] = s.info.ID
	return nil
}

func (m *Manager) remove(s *session) {
	m.mu.Lock()
	if m.sessions[s.info.ID] == s {
		delete(m.sessions, s.info.ID)
		delete(m.byTarget, s.info.Target)
	}
	m.mu.Unlock()
	_ = m.store.remove(m.store.sessionPath(s.info.ID))
}

func (m *Manager) archive(s *session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions[s.info.ID] != s {
		return nil
	}
	s.info.State = "stopped"
	if m.byTarget[s.info.Target] == s.info.ID {
		delete(m.byTarget, s.info.Target)
	}
	// Loaded sessions always start stopped; the live state need not be rewritten on disk.
	return nil
}

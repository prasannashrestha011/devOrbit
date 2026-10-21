package traffic_inspector

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
)

var ErrDuplicate = errors.New("inspection already active for this target")
var ErrClosed = errors.New("traffic inspector is shutting down")
var ErrNotFound = errors.New("inspection session not found")

type session struct {
	info      SessionInfo
	listener  net.Listener
	server    *http.Server
	transport *http.Transport
	records   []Record
	sequence  uint64
	mu        sync.Mutex
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]*session
	byTarget map[Target]uuid.UUID
	closed   bool
	broker   *events.Broker
}

func New(broker *events.Broker) *Manager {
	return &Manager{
		sessions: map[uuid.UUID]*session{},
		byTarget: map[Target]uuid.UUID{},
		broker:   broker,
	}
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

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		m.remove(s)
		return SessionInfo{}, err
	}
	transport := &http.Transport{
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
		_ = server.Serve(listener)
		transport.CloseIdleConnections()
		m.remove(s)
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
		if s.info.Target.WorkspaceID == workspaceID && s.info.State == "active" {
			items = append(items, s.info)
		}
	}
	return items
}

func (m *Manager) Stop(ctx context.Context, workspaceID, id uuid.UUID) error {
	m.mu.Lock()
	s := m.sessions[id]
	if s == nil || s.info.Target.WorkspaceID != workspaceID {
		m.mu.Unlock()
		return ErrNotFound
	}
	if s.info.State == "stopping" {
		m.mu.Unlock()
		return nil
	}
	s.info.State = "stopping"
	server, transport := s.server, s.transport
	m.mu.Unlock()
	if server == nil {
		m.remove(s)
		return nil
	}
	err := server.Shutdown(ctx)
	if err != nil {
		_ = server.Close()
	}
	transport.CloseIdleConnections()
	m.remove(s)
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
		if existing.info.ProxyURL == s.info.Upstream {
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
}

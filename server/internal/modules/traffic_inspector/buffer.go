package traffic_inspector

import (
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	"path/filepath"
	"time"
)

func (m *Manager) Recent(workspaceID, id uuid.UUID, after uint64, limit int) ([]Record, uint64, error) {
	m.mu.RLock()
	s := m.sessions[id]
	if s == nil || s.info.Target.WorkspaceID != workspaceID {
		m.mu.RUnlock()
		return nil, 0, ErrNotFound
	}
	m.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit < 1 || limit > 500 {
		limit = 500
	}
	result := make([]Record, 0, limit)
	for _, item := range s.records {
		if item.Sequence > after && len(result) < limit && timeWithinRetention(item) {
			result = append(result, item)
		}
	}
	return result, s.sequence, nil
}

func (m *Manager) Clear(workspaceID, id uuid.UUID) error {
	m.mu.RLock()
	s := m.sessions[id]
	if s == nil || s.info.Target.WorkspaceID != workspaceID {
		m.mu.RUnlock()
		return ErrNotFound
	}
	m.mu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
	for _, record := range s.records {
		if err := m.store.remove(m.store.exchangePath(id, record.ID)); err != nil {
			return err
		}
	}
	s.records = nil
	return nil
}

func (m *Manager) append(s *session, exchange Exchange, generation uint64) {
	s.mu.Lock()
	if generation != s.generation {
		_ = m.store.remove(m.store.exchangePath(exchange.SessionID, exchange.ID))
		s.mu.Unlock()
		return
	}
	s.sequence++
	exchange.Sequence = s.sequence
	if err := m.store.put(filepath.Join(m.store.exchangePath(exchange.SessionID, exchange.ID), "exchange.json"), exchange); err != nil {
		exchange.CaptureIncomplete = true
		exchange.CaptureError = "Capture index could not be saved; this exchange will not survive restart."
	}
	s.records = append(s.records, exchange.Record)
	if len(s.records) > 500 {
		if err := m.store.remove(m.store.exchangePath(exchange.SessionID, s.records[0].ID)); err == nil {
			s.records = s.records[1:]
		}
	}
	s.mu.Unlock()
	m.publish(s.info.Target, events.TrafficCompleted, map[string]any{"session_id": exchange.SessionID})
}

func timeWithinRetention(record Record) bool { return time.Since(record.StartedAt) <= retention }

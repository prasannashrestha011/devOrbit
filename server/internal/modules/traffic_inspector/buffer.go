package traffic_inspector

import (
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
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
		if item.Sequence > after && len(result) < limit {
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
	s.records = s.records[:0]
	s.mu.Unlock()
	return nil
}

func (m *Manager) append(s *session, record Record) {
	s.mu.Lock()
	s.sequence++
	record.Sequence = s.sequence
	if len(s.records) == cap(s.records) {
		copy(s.records, s.records[1:])
		s.records[len(s.records)-1] = record
	} else {
		s.records = append(s.records, record)
	}
	s.mu.Unlock()
	m.publish(s.info.Target, events.TrafficCompleted, record)
}

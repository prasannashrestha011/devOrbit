package traffic_inspector

import (
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
)

func (m *Manager) publish(target Target, kind events.Type, data any) {
	if m.broker == nil {
		return
	}
	serviceID, _ := uuid.Parse(target.ServiceKey)
	m.broker.Publish(events.Event{
		Type:        kind,
		WorkspaceID: target.WorkspaceID,
		ServiceID:   serviceID,
		Data:        data,
	})
}

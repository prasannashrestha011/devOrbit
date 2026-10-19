package events

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBrokerFiltersOverflowAndClose(t *testing.T) {
	b := New()
	wid, sid := uuid.New(), uuid.New()
	workspace, cancelWorkspace := b.Subscribe(Filter{WorkspaceID: wid})
	defer cancelWorkspace()
	service, cancelService := b.Subscribe(Filter{ServiceID: sid})
	defer cancelService()
	other, cancelOther := b.Subscribe(Filter{WorkspaceID: uuid.New()})
	defer cancelOther()
	b.Publish(Event{Type: ProcessStarted, WorkspaceID: wid, ServiceID: sid})
	for _, ch := range []<-chan Event{workspace, service} {
		select {
		case e := <-ch:
			if e.ID == "" || e.Type != ProcessStarted || e.Timestamp.Location() != time.UTC || e.WorkspaceID != wid || e.ServiceID != sid {
				t.Fatalf("bad event: %+v", e)
			}
		default:
			t.Fatal("matching subscriber missed event")
		}
	}
	select {
	case <-other:
		t.Fatal("unmatched subscriber got event")
	default:
	}
	for range 65 {
		b.Publish(Event{Type: LogCreated, WorkspaceID: wid, ServiceID: sid})
	}
	for range 64 {
		<-workspace
	}
	if _, ok := <-workspace; ok {
		t.Fatal("slow subscriber was not disconnected")
	}
	b.Close()
	b.Close()
	if _, ok := <-other; ok {
		t.Fatal("broker close did not disconnect subscriber")
	}
	closed, cancel := b.Subscribe(Filter{})
	defer cancel()
	if _, ok := <-closed; ok {
		t.Fatal("closed broker accepted subscriber")
	}
}

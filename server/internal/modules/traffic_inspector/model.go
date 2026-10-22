package traffic_inspector

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Target struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Kind        string    `json:"kind"`
	// ServiceKey is a DevOrbit service UUID for local targets and a Compose service name for Docker targets.
	ServiceKey string `json:"service_key"`
	PortKey    string `json:"port_key"`
	File       string `json:"file,omitempty"`
}

type SessionInfo struct {
	ID     uuid.UUID `json:"id"`
	Target Target    `json:"target"`
	//actual backend url
	Upstream string `json:"upstream"`
	//gateway url
	ProxyURL  string    `json:"proxy_url"`
	State     string    `json:"state"`
	StartedAt time.Time `json:"started_at"`
}

type Record struct {
	ID                uuid.UUID `json:"id"`
	SessionID         uuid.UUID `json:"session_id"`
	Sequence          uint64    `json:"sequence"`
	StartedAt         time.Time `json:"started_at"`
	Method            string    `json:"method"`
	Path              string    `json:"path"`
	StatusCode        int       `json:"status_code"`
	DurationMS        int64     `json:"duration_ms"`
	RequestBytes      *int64    `json:"request_bytes,omitempty"`
	ResponseBytes     int64     `json:"response_bytes"`
	Outcome           string    `json:"outcome"`
	Error             string    `json:"error,omitempty"`
	CaptureIncomplete bool      `json:"capture_incomplete"`
	CaptureError      string    `json:"capture_error,omitempty"`
}

type Body struct {
	Bytes         int64  `json:"bytes"`
	CapturedBytes int64  `json:"captured_bytes"`
	Complete      bool   `json:"complete"`
	Error         string `json:"error,omitempty"`
}

type Message struct {
	Protocol         string      `json:"protocol"`
	Status           string      `json:"status,omitempty"`
	Headers          http.Header `json:"headers"`
	Trailers         http.Header `json:"trailers"`
	TransferEncoding []string    `json:"transfer_encoding,omitempty"`
	Body             Body        `json:"body"`
}

type Exchange struct {
	Record
	Host     string  `json:"host"`
	RawQuery string  `json:"raw_query"`
	Request  Message `json:"request"`
	Response Message `json:"response"`
}

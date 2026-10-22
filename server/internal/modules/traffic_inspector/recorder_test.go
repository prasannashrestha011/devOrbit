package traffic_inspector

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type recorderTransport func(*http.Request) (*http.Response, error)

func (f recorderTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type interruptedBody struct{ cancel context.CancelFunc }

func (b interruptedBody) Read(p []byte) (int, error) {
	if b.cancel != nil {
		b.cancel()
	}
	return copy(p, "partial"), io.ErrUnexpectedEOF
}

func (interruptedBody) Close() error { return nil }

func TestRecordingHandlerRecordsInterruptedResponses(t *testing.T) {
	for _, test := range []struct {
		name    string
		cancel  bool
		outcome string
		problem string
	}{
		{name: "upstream disconnect", outcome: "upstream_error", problem: "response interrupted"},
		{name: "client disconnect", cancel: true, outcome: "canceled", problem: "client disconnected"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := newTestManager(t, nil)
			s := &session{info: SessionInfo{ID: uuid.New()}, records: make([]Record, 0, 500)}
			request := httptest.NewRequest(http.MethodGet, "/download", nil)
			// ReverseProxy only propagates copy errors as panics in a real server context.
			ctx, cancel := context.WithCancel(context.WithValue(request.Context(), http.ServerContextKey, &http.Server{}))
			defer cancel()
			request = request.WithContext(ctx)
			body := interruptedBody{}
			if test.cancel {
				body.cancel = cancel
			}
			proxy := newReverseProxy(loopbackURL(8080), nil)
			proxy.Transport = recorderTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: body}, nil
			})

			func() {
				defer func() {
					if problem := recover(); problem != http.ErrAbortHandler {
						t.Errorf("panic = %v, want http.ErrAbortHandler", problem)
					}
				}()
				manager.recordingHandler(s, proxy).ServeHTTP(httptest.NewRecorder(), request)
			}()

			if len(s.records) != 1 {
				t.Fatalf("interrupted response produced %d records, want 1", len(s.records))
			}
			record := s.records[0]
			if record.StatusCode != http.StatusOK || record.ResponseBytes != 7 || record.Outcome != test.outcome || record.Error != test.problem {
				t.Fatalf("incorrect interrupted response record: %+v", record)
			}
		})
	}
}

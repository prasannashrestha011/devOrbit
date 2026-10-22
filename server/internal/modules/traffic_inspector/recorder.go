package traffic_inspector

import (
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

type bodySink struct {
	mu     sync.Mutex
	store  *captureStore
	file   *os.File
	body   Body
	eof    bool
	closed bool
}

func (s *captureStore) body(path string) *bodySink {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	sink := &bodySink{store: s, file: f}
	if err != nil {
		sink.body.Error = "Could not open capture file."
	}
	return sink
}

func (s *bodySink) write(p []byte, eof bool, readErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.body.Bytes += int64(len(p))
	s.eof = s.eof || eof
	if s.body.Error == "" && len(p) > 0 {
		s.store.mu.Lock()
		// Reserve space for exchange indexes so a full body quota can still be reported.
		reserve := min(int64(4<<20), s.store.limit/8)
		remaining := max(int64(0), s.store.limit-reserve-s.store.used)
		count := min(int64(len(p)), remaining)
		n, err := s.file.Write(p[:count])
		s.store.used += int64(n)
		s.body.CapturedBytes += int64(n)
		s.store.mu.Unlock()
		if err != nil {
			s.body.Error = "Writing the capture failed."
		} else if int64(n) < int64(len(p)) {
			s.body.Error = "Capture storage quota reached."
		}
	}
	if readErr != nil && readErr != io.EOF && s.body.Error == "" {
		s.body.Error = "Body stream was interrupted."
	}
}

func (s *bodySink) finish(complete bool) Body {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.file != nil {
		if err := s.file.Sync(); err != nil && s.body.Error == "" {
			s.body.Error = "Saving the capture failed."
		}
		if err := s.file.Close(); err != nil && s.body.Error == "" {
			s.body.Error = "Closing the capture failed."
		}
	}
	s.body.Complete = complete && s.body.Error == ""
	if !complete && s.body.Error == "" {
		s.body.Error = "Body stream did not finish."
	}
	return s.body
}

type capturedReader struct {
	io.ReadCloser
	sink *bodySink
}

func (r *capturedReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.sink.write(p[:n], err == io.EOF, err)
	return n, err
}

func (m *Manager) recordingHandler(s *session, proxy *httputil.ReverseProxy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		id := uuid.New()
		s.mu.Lock()
		generation := s.generation
		s.mu.Unlock()
		path := m.store.exchangePath(s.info.ID, id)
		_ = os.MkdirAll(path, 0700) // Opening each body reports a failure without interrupting forwarding.
		requestBody := m.store.body(filepath.Join(path, "request.body"))
		responseBody := m.store.body(filepath.Join(path, "response.body"))
		request := Message{Protocol: r.Proto, Headers: r.Header.Clone(), TransferEncoding: r.TransferEncoding}
		request.Headers.Set("Host", r.Host)
		if r.Body != nil {
			r.Body = &capturedReader{ReadCloser: r.Body, sink: requestBody}
		}
		response := Message{Protocol: r.Proto}
		writer := &observedWriter{ResponseWriter: w, sink: responseBody}
		var upstream *http.Response
		completed, upstreamError := false, false
		// Use a per-request proxy copy; callbacks must never be shared between concurrent exchanges.
		p := *proxy
		p.ModifyResponse = func(res *http.Response) error {
			upstream = res
			response.Protocol = res.Proto
			response.Status = res.Status
			response.TransferEncoding = res.TransferEncoding
			if proxy.ModifyResponse != nil {
				return proxy.ModifyResponse(res)
			}
			return nil
		}
		p.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			upstreamError = true
			proxy.ErrorHandler(w, req, err)
		}
		// Record during panic unwinding without swallowing the proxy's abort.
		defer func() {
			if r.Body != nil {
				_ = r.Body.Close()
			}
			requestBody.mu.Lock()
			requestComplete := requestBody.eof || r.ContentLength >= 0 && requestBody.body.Bytes == r.ContentLength
			requestBody.mu.Unlock()
			request.Body = requestBody.finish(requestComplete)
			if requestComplete {
				request.Trailers = r.Trailer.Clone()
			}
			response.Headers = writer.headers
			if response.Status == "" && writer.status != 0 {
				response.Status = strconv.Itoa(writer.status) + " " + http.StatusText(writer.status)
			}
			response.Body = responseBody.finish(completed && r.Context().Err() == nil && writer.status != http.StatusSwitchingProtocols)
			if upstream != nil {
				response.Trailers = upstream.Trailer.Clone()
			}
			outcome, problem := "completed", ""
			if r.Context().Err() != nil {
				outcome, problem = "canceled", "client disconnected"
			} else if !completed {
				outcome, problem = "upstream_error", "response interrupted"
			} else if upstreamError {
				outcome, problem = "upstream_error", "upstream unavailable"
			}
			requestBytes := request.Body.Bytes
			m.append(s, Exchange{
				Record: Record{ID: id, SessionID: s.info.ID, StartedAt: start, Method: r.Method,
					Path: r.URL.RequestURI(), StatusCode: writer.status, DurationMS: time.Since(start).Milliseconds(),
					RequestBytes: &requestBytes, ResponseBytes: response.Body.Bytes, Outcome: outcome, Error: problem,
					CaptureIncomplete: !request.Body.Complete || !response.Body.Complete},
				Host: r.Host, RawQuery: r.URL.RawQuery, Request: request, Response: response,
			}, generation)
		}()
		p.ServeHTTP(writer, r)
		completed = true
	})
}

type observedWriter struct {
	http.ResponseWriter
	status  int
	headers http.Header
	sink    *bodySink
}

func (w *observedWriter) WriteHeader(code int) {
	if w.status == 0 && (code >= 200 || code == http.StatusSwitchingProtocols) {
		w.status = code
		w.headers = w.Header().Clone()
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *observedWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.sink.write(p[:n], false, err)
	return n, err
}
func (w *observedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

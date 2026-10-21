package traffic_inspector

import (
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (m *Manager) recordingHandler(s *session, proxy *httputil.ReverseProxy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		writer := &observedWriter{ResponseWriter: w}
		var requestBytes *int64
		if r.ContentLength >= 0 {
			n := r.ContentLength
			requestBytes = &n
		}
		proxy.ServeHTTP(writer, r)
		outcome, problem := "completed", ""
		if r.Context().Err() != nil {
			outcome, problem = "canceled", "client disconnected"
		} else if writer.status == http.StatusBadGateway {
			outcome, problem = "upstream_error", "upstream unavailable"
		}
		m.append(s, Record{
			ID:            uuid.New(),
			SessionID:     s.info.ID,
			StartedAt:     start,
			Method:        r.Method,
			Path:          safePath(r.URL.EscapedPath()),
			StatusCode:    writer.status,
			DurationMS:    time.Since(start).Milliseconds(),
			RequestBytes:  requestBytes,
			ResponseBytes: writer.bytes,
			Outcome:       outcome,
			Error:         problem,
		})
	})
}

type observedWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *observedWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *observedWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

func (w *observedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func safePath(path string) string {
	if path == "" {
		return "/"
	}
	parts := strings.Split(path, "/")
	redactNext := false
	for i, part := range parts {
		lower := strings.ToLower(part)
		if redactNext || len(part) > 24 || strings.ContainsAny(part, "%0123456789@=:.+~") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") {
			parts[i] = ":value"
		}
		redactNext = lower == "user" || lower == "users" || lower == "account" || lower == "accounts" || lower == "profile" || lower == "profiles" || lower == "auth" || lower == "reset" || lower == "password" || lower == "key" || lower == "invite" || strings.Contains(lower, "token") || strings.Contains(lower, "secret")
	}
	result := strings.Join(parts, "/")
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

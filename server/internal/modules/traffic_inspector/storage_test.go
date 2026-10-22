package traffic_inspector

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
)

func newTestManager(t *testing.T, broker *events.Broker) *Manager {
	t.Helper()
	m, err := New(t.TempDir(), broker)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close(context.Background()) })
	return m
}

func appendTestRecord(t *testing.T, m *Manager, s *session, record Record) {
	t.Helper()
	if record.ID == uuid.Nil {
		record.ID = uuid.New()
	}
	if record.StartedAt.IsZero() {
		record.StartedAt = time.Now()
	}
	if err := os.MkdirAll(m.store.exchangePath(record.SessionID, record.ID), 0700); err != nil {
		t.Fatal(err)
	}
	m.append(s, Exchange{Record: record}, 0)
}

func TestCompleteExchangeSurvivesRestartAndCanBeCleared(t *testing.T) {
	root := t.TempDir()
	responseBody := []byte(`{"ok":true}`)
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	if _, err := zw.Write(responseBody); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "id=1&id=2&token=secret" || r.Header.Values("X-Repeat")[1] != "second" {
			t.Errorf("request changed: %s %+v", r.URL, r.Header)
		}
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(body, []byte{0, 255, 10}) {
			t.Errorf("forwarded body = %v", body)
		}
		w.Header().Add("Set-Cookie", "a=1; HttpOnly")
		w.Header().Add("Set-Cookie", "b=2; Secure")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Trailer", "X-Finish")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(compressed.Bytes())
		w.Header().Set("X-Finish", "done")
	}))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	manager, err := New(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	workspaceID := uuid.New()
	target := Target{WorkspaceID: workspaceID, Kind: "local", ServiceKey: uuid.NewString(), PortKey: "http"}
	session, err := manager.Start(target, address)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPost, session.ProxyURL+"/users/123?id=1&id=2&token=secret", bytes.NewReader([]byte{0, 255, 10}))
	request.Header.Add("X-Repeat", "first")
	request.Header.Add("X-Repeat", "second")
	request.Header.Set("Cookie", "session=secret")
	client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	forwarded, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if !bytes.Equal(forwarded, compressed.Bytes()) || response.Trailer.Get("X-Finish") != "done" {
		t.Fatalf("response changed: %v %+v", forwarded, response.Trailer)
	}
	records := waitRecords(t, manager, workspaceID, session.ID, 0, 1)
	if records[0].CaptureIncomplete {
		t.Fatalf("incomplete history = %+v", records)
	}
	id := records[0].ID
	if err := manager.Stop(context.Background(), workspaceID, session.ID); err != nil {
		t.Fatal(err)
	}
	manager.Close(context.Background())
	manager, err = New(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close(context.Background())
	if saved, err := manager.Get(workspaceID, session.ID); err != nil || saved.State != "stopped" {
		t.Fatalf("retained session = %+v, %v", saved, err)
	}
	if _, _, err := manager.exchange(uuid.New(), session.ID, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace detail = %v", err)
	}
	s, exchange, err := manager.exchange(workspaceID, session.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	if exchange.Path != "/users/123?id=1&id=2&token=secret" || exchange.RawQuery != "id=1&id=2&token=secret" || exchange.Request.Headers.Get("Cookie") != "session=secret" || len(exchange.Request.Headers.Values("X-Repeat")) != 2 || len(exchange.Response.Headers.Values("Set-Cookie")) != 2 || exchange.Response.Trailers.Get("X-Finish") != "done" {
		t.Fatalf("missing exchange detail: %+v", exchange)
	}
	if !exchange.Request.Body.Complete || exchange.Request.Body.CapturedBytes != 3 || !exchange.Response.Body.Complete || exchange.Response.Body.CapturedBytes != int64(compressed.Len()) {
		t.Fatalf("body sizes: %+v %+v", exchange.Request.Body, exchange.Response.Body)
	}
	if preview := bodyPreview(filepath.Join(manager.store.exchangePath(session.ID, id), "response.body"), exchange.Response.Headers); preview.Text != string(responseBody) {
		t.Fatalf("decoded response = %+v", preview)
	}
	router := chi.NewRouter()
	(&Controller{Manager: manager}).Register(router)
	url := "/workspaces/" + workspaceID.String() + "/traffic/" + session.ID.String() + "/records/" + id.String()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", url, nil))
	if recorder.Code != 200 || !bytes.Contains(recorder.Body.Bytes(), []byte("token=secret")) {
		t.Fatalf("detail endpoint: %d %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", url+"/body/response", nil))
	if recorder.Code != 200 || !bytes.Equal(recorder.Body.Bytes(), compressed.Bytes()) {
		t.Fatalf("download endpoint: %d %v", recorder.Code, recorder.Body.Bytes())
	}
	if err := manager.Clear(workspaceID, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(manager.store.exchangePath(session.ID, id)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("clear retained body files: %v", err)
	}
}

func TestCaptureQuotaDoesNotChangeForwardedRequest(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 950<<10)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !bytes.Equal(body, payload) {
			t.Errorf("forwarding cut off at %d bytes", len(body))
		}
		w.WriteHeader(204)
	}))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	manager := newTestManager(t, nil)
	manager.store.limit = 1 << 20
	target := Target{WorkspaceID: uuid.New(), Kind: "local", ServiceKey: uuid.NewString(), PortKey: "http"}
	session, err := manager.Start(target, address)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(session.ProxyURL, "application/octet-stream", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	records := waitRecords(t, manager, target.WorkspaceID, session.ID, 0, 1)
	if response.StatusCode != 204 || len(records) != 1 || !records[0].CaptureIncomplete {
		t.Fatalf("quota outcome: %d %+v", response.StatusCode, records)
	}
	s, exchange, err := manager.exchange(target.WorkspaceID, session.ID, records[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	if exchange.Request.Body.Bytes != int64(len(payload)) || exchange.Request.Body.CapturedBytes >= int64(len(payload)) || exchange.Request.Body.Error != "Capture storage quota reached." {
		t.Fatalf("quota capture: %+v", exchange.Request.Body)
	}
}

func waitRecords(t *testing.T, m *Manager, workspaceID, sessionID uuid.UUID, after uint64, count int) []Record {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		records, _, err := m.Recent(workspaceID, sessionID, after, 500)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) >= count {
			return records
		}
		if time.Now().After(deadline) {
			t.Fatalf("wanted %d records after %d, got %d", count, after, len(records))
		}
		time.Sleep(time.Millisecond)
	}
}

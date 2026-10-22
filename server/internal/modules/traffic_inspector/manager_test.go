package traffic_inspector

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
)

func TestGatewayForwardingAndRecording(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/users/123" || r.URL.RawQuery != "token=private" || r.Header.Get("X-Test") != "forwarded" {
			t.Errorf("request was not forwarded intact: %s %s", r.Method, r.URL.String())
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "payload" {
			t.Errorf("body = %q", body)
		}
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "accepted")
	}))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	broker := events.New()
	defer broker.Close()
	manager := newTestManager(t, broker)
	defer manager.Close(context.Background())
	workspaceID := uuid.New()
	target := Target{WorkspaceID: workspaceID, Kind: "local", ServiceKey: uuid.NewString(), PortKey: "http"}
	stream, unsubscribe := broker.Subscribe(events.Filter{WorkspaceID: workspaceID})
	defer unsubscribe()
	info, err := manager.Start(target, address)
	if err != nil {
		t.Fatal(err)
	}
	if info.State != "active" || !strings.HasPrefix(info.ProxyURL, "http://127.0.0.1:") {
		t.Fatalf("unexpected gateway: %+v", info)
	}
	request, _ := http.NewRequest(http.MethodPost, info.ProxyURL+"/users/123?token=private", strings.NewReader("payload"))
	request.Header.Set("X-Test", "forwarded")
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != 201 || response.Header.Get("X-Upstream") != "yes" || string(body) != "accepted" {
		t.Fatalf("response not forwarded: %d %q %q", response.StatusCode, response.Header.Get("X-Upstream"), body)
	}
	records := waitRecords(t, manager, workspaceID, info.ID, 0, 1)
	latest := records[0].Sequence
	if latest != 1 {
		t.Fatalf("history = %+v", records)
	}
	record := records[0]
	if record.Path != "/users/123?token=private" || record.Method != "POST" || record.StatusCode != 201 || record.RequestBytes == nil || *record.RequestBytes != 7 || record.ResponseBytes != 8 || record.Outcome != "completed" {
		t.Fatalf("unsafe or incorrect record: %+v", record)
	}

	gotEvent := false
	for range 2 {
		select {
		case event := <-stream:
			if event.Type == events.TrafficCompleted {
				gotEvent = true
			}
		case <-time.After(time.Second):
			t.Fatal("traffic event not published")
		}
	}
	if !gotEvent {
		t.Fatal("traffic event missing")
	}
	if _, err := manager.Get(uuid.New(), info.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace lookup: %v", err)
	}
	if err := manager.Stop(context.Background(), workspaceID, info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(info.ProxyURL); err == nil {
		t.Fatal("stopped gateway still accepts requests")
	}
	if saved, err := manager.Get(workspaceID, info.ID); err != nil || saved.State != "stopped" {
		t.Fatalf("history not retained after stop: %+v, %v", saved, err)
	}
}

func TestConcurrentStartAndBoundedHistory(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	manager := newTestManager(t, nil)
	defer manager.Close(context.Background())
	target := Target{WorkspaceID: uuid.New(), Kind: "local", ServiceKey: uuid.NewString(), PortKey: "http"}
	var wait sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := manager.Start(target, address)
			results <- err
		}()
	}
	wait.Wait()
	close(results)
	success, duplicates := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrDuplicate):
			duplicates++
		default:
			t.Fatalf("unexpected start error: %v", err)
		}
	}
	if success != 1 || duplicates != 7 {
		t.Fatalf("start outcomes: %d successful, %d duplicate", success, duplicates)
	}
	info := manager.List(target.WorkspaceID)[0]
	s := manager.sessions[info.ID]
	for index := 0; index < 520; index++ {
		appendTestRecord(t, manager, s, Record{SessionID: info.ID})
	}
	records, latest, err := manager.Recent(target.WorkspaceID, info.ID, 0, 500)
	if err != nil || len(records) != 500 || records[0].Sequence != 21 || records[499].Sequence != 520 || latest != 520 {
		t.Fatalf("bounded history = %d records, first %d, latest %d, error %v", len(records), records[0].Sequence, latest, err)
	}
	if err := manager.Clear(target.WorkspaceID, info.ID); err != nil {
		t.Fatal(err)
	}
	records, latest, _ = manager.Recent(target.WorkspaceID, info.ID, 0, 500)
	if len(records) != 0 || latest != 520 {
		t.Fatalf("clear reset sequence: %d records, latest %d", len(records), latest)
	}
	manager.Close(context.Background())
	if manager.List(target.WorkspaceID)[0].State != "stopped" {
		t.Fatal("shutdown did not stop session")
	}
	if _, err := manager.Start(target, address); !errors.Is(err, ErrClosed) {
		t.Fatalf("start after shutdown: %v", err)
	}
}

func TestUpstreamFailureAndStreaming(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "first")
		w.(http.Flusher).Flush()
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(w, "second")
	}))
	address, _ := url.Parse(upstream.URL)
	manager := newTestManager(t, nil)
	defer manager.Close(context.Background())
	target := Target{WorkspaceID: uuid.New(), Kind: "local", ServiceKey: uuid.NewString(), PortKey: "http"}
	info, err := manager.Start(target, address)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(info.ProxyURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(body) != "firstsecond" {
		t.Fatalf("stream = %q", body)
	}
	records := waitRecords(t, manager, target.WorkspaceID, info.ID, 0, 1)
	if records[0].DurationMS < 25 {
		t.Fatalf("stream duration missing: %+v", records)
	}
	upstream.Close()
	response, err = http.Get(info.ProxyURL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	records = waitRecords(t, manager, target.WorkspaceID, info.ID, 1, 1)
	if response.StatusCode != 502 || len(records) != 1 || records[0].Outcome != "upstream_error" {
		t.Fatalf("upstream error missing: status %d, records %+v", response.StatusCode, records)
	}
}

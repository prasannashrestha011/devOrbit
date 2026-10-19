package logs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func privateTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCaptureHistoryAndPaging(t *testing.T) {
	root := privateTempDir(t)
	m, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	workspace, service := uuid.New(), uuid.New()
	c, err := m.Begin(workspace, service)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, stream := range []struct {
		writer interface{ Write([]byte) (int, error) }
		text   string
	}{{c.Stdout, "hello\npartial"}, {c.Stderr, "\x1b[31merror\r\n"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := stream.writer.Write([]byte(stream.text)); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if got := m.Recent(service); len(got) != 3 {
		t.Fatalf("recent count = %d", len(got))
	}
	recent := m.Recent(service)
	recent[0].Message = "changed"
	if m.Recent(service)[0].Message == "changed" {
		t.Fatal("snapshot was mutable")
	}
	first, err := m.List(context.Background(), Filter{ServiceID: service, Limit: 2})
	if err != nil || len(first) != 2 {
		t.Fatalf("first page = %+v, %v", first, err)
	}
	second, err := m.List(context.Background(), Filter{WorkspaceID: workspace, Limit: 2, Before: first[1].ID})
	if err != nil || len(second) != 1 {
		t.Fatalf("second page = %+v, %v", second, err)
	}
	if first[0].ID <= first[1].ID || second[0].ID >= first[1].ID {
		t.Fatal("incorrect order")
	}
	for _, e := range append(first, second...) {
		if e.ExecutionID != c.ExecutionID() || e.WorkspaceID != workspace || e.ServiceID != service || e.Timestamp.Location() != time.UTC {
			t.Fatalf("bad metadata: %+v", e)
		}
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	all, err := reopened.List(context.Background(), Filter{ServiceID: service, Limit: 100})
	if err != nil || len(all) != 3 {
		t.Fatalf("persisted = %+v, %v", all, err)
	}
	if _, err := m.Begin(workspace, service); err != ErrClosed {
		t.Fatalf("begin after close: %v", err)
	}
}

func TestRejectsLogDirectorySymlinks(t *testing.T) {
	base := filepath.Join(t.TempDir(), "redirected")
	if err := os.Symlink(t.TempDir(), base); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := New(base); err == nil {
		t.Fatal("accepted redirected configured log directory")
	}
	root := privateTempDir(t)
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "services")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := New(root); err == nil {
		t.Fatal("accepted redirected log root")
	}
	other := privateTempDir(t)
	m, err := New(other)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	id := uuid.New()
	if err := os.Symlink(t.TempDir(), filepath.Join(other, "services", id.String())); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := m.Begin(uuid.New(), id); err == nil {
		t.Fatal("accepted redirected service log directory")
	}
}

func TestChunkBufferRotationAndRetention(t *testing.T) {
	root := privateTempDir(t)
	m, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	workspace, service := uuid.New(), uuid.New()
	c, err := m.Begin(workspace, service)
	if err != nil {
		t.Fatal(err)
	}
	c.fileSize = maxFileBytes - 100
	if _, err := c.Stdout.Write([]byte(strings.Repeat("x", maxChunk*550))); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if got := m.Recent(service); len(got) != maxBytes/maxChunk {
		t.Fatalf("recent count = %d", len(got))
	}
	if m.Recent(service)[0].Message == "" {
		t.Fatal("missing buffered message")
	}
	files, err := os.ReadDir(c.dir)
	if err != nil || len(files) < 2 {
		t.Fatalf("rotation files = %d, %v", len(files), err)
	}
	all, err := m.List(context.Background(), Filter{ServiceID: service, Limit: 500})
	if err != nil || len(all) != 500 {
		t.Fatalf("history = %d, %v", len(all), err)
	}
	stale := filepath.Join(root, "services", service.String(), uuid.NewString())
	if err := os.MkdirAll(stale, 0700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-retention - time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	next, err := m.Begin(workspace, service)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale execution retained: %v", err)
	}
}

func TestExecutionSeparationAndUTF8ChunkBoundary(t *testing.T) {
	m, err := New(privateTempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	wid, sid := uuid.New(), uuid.New()
	first, err := m.Begin(wid, sid)
	if err != nil {
		t.Fatal(err)
	}
	message := strings.Repeat("x", maxChunk-1) + "€"
	if _, err := first.Stdout.Write([]byte(message)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := m.Begin(wid, sid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Stderr.Write([]byte("new run\n")); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := m.List(context.Background(), Filter{ServiceID: sid, Limit: 10})
	if err != nil || len(entries) != 3 {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	if first.ExecutionID() == second.ExecutionID() || entries[0].ExecutionID != second.ExecutionID() {
		t.Fatal("execution IDs mixed")
	}
	if entries[2].Message+entries[1].Message != message {
		t.Fatal("UTF-8 chunk boundary changed content")
	}
}

func TestPartialOutputIsVisibleWhileRunning(t *testing.T) {
	m, err := New(privateTempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	c, err := m.Begin(uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Stdout.Write([]byte("still running")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if entries := m.Recent(c.serviceID); len(entries) == 1 && entries[0].Message == "still running" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if entries := m.Recent(c.serviceID); len(entries) != 1 || entries[0].Message != "still running" {
		t.Fatalf("partial output unavailable: %+v", entries)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}

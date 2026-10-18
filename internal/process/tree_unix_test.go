//go:build !windows

package process

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStopManagedDescendant(t *testing.T) {
	s := New(50 * time.Millisecond)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	id := uuid.New()
	cfg := helperConfig(t, "spawn")
	cfg.Env["DEVORBIT_TEST_READY"] = filepath.Join(t.TempDir(), "ready")
	cfg.Env["DEVORBIT_TEST_BEAT"] = filepath.Join(t.TempDir(), "beat")
	if _, err := s.Start(id, cfg); err != nil {
		t.Fatal(err)
	}
	waitFile(t, cfg.Env["DEVORBIT_TEST_READY"])
	waitFile(t, cfg.Env["DEVORBIT_TEST_BEAT"])
	if _, err := s.Stop(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfg.Env["DEVORBIT_TEST_BEAT"])
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	after, err := os.ReadFile(cfg.Env["DEVORBIT_TEST_BEAT"])
	if err != nil || string(before) != string(after) {
		t.Fatalf("descendant remained active: %q -> %q (%v)", before, after, err)
	}
}

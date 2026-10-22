package traffic_inspector

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const captureQuota int64 = 512 << 20
const retention = 7 * 24 * time.Hour

// All paths are constructed from server-generated UUIDs, never request paths.
type captureStore struct {
	mu    sync.Mutex
	root  string
	used  int64
	limit int64
}

func (s *captureStore) sessionPath(id uuid.UUID) string { return filepath.Join(s.root, id.String()) }
func (s *captureStore) exchangePath(sid, id uuid.UUID) string {
	return filepath.Join(s.sessionPath(sid), id.String())
}

func (s *captureStore) put(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var old int64
	if info, err := os.Stat(path); err == nil {
		old = info.Size()
	}
	if s.used+int64(len(data)) > s.limit {
		return errors.New("capture storage quota reached")
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".capture-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	err = errors.Join(writeErr, f.Close())
	if err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	s.used += int64(len(data)) - old
	return nil
}

func (s *captureStore) remove(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var size int64
	err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	s.used -= size
	return nil
}

func readCaptureJSON(path string, value any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(value)
}

func (m *Manager) load() error {
	if err := os.MkdirAll(m.store.root, 0700); err != nil {
		return err
	}
	if err := os.Chmod(m.store.root, 0700); err != nil {
		return err
	}
	if err := filepath.WalkDir(m.store.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() && len(entry.Name()) > len(".capture-") && entry.Name()[:len(".capture-")] == ".capture-" {
			return os.Remove(path)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("symlink in traffic storage")
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			m.store.used += info.Size()
		}
		return nil
	}); err != nil {
		return err
	}
	dirs, err := os.ReadDir(m.store.root)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		sid, err := uuid.Parse(dir.Name())
		if err != nil || !dir.IsDir() {
			continue
		}
		s := &session{records: make([]Record, 0, 500)}
		if err := readCaptureJSON(filepath.Join(m.store.sessionPath(sid), "session.json"), &s.info); err != nil {
			return err
		}
		if s.info.ID != sid {
			return errors.New("invalid traffic session index")
		}
		s.info.State = "stopped"
		entries, err := os.ReadDir(m.store.sessionPath(sid))
		if err != nil {
			return err
		}
		for _, entry := range entries {
			id, err := uuid.Parse(entry.Name())
			if err != nil || !entry.IsDir() {
				continue
			}
			path := m.store.exchangePath(sid, id)
			var exchange Exchange
			if err := readCaptureJSON(filepath.Join(path, "exchange.json"), &exchange); errors.Is(err, os.ErrNotExist) {
				// A process crash may leave unfinished body files without a committed index.
				if err := m.store.remove(path); err != nil {
					return err
				}
				continue
			} else if err != nil {
				return err
			}
			if exchange.ID != id || exchange.SessionID != sid {
				return errors.New("invalid traffic exchange index")
			}
			s.records = append(s.records, exchange.Record)
			if exchange.Sequence > s.sequence {
				s.sequence = exchange.Sequence
			}
		}
		sort.Slice(s.records, func(i, j int) bool { return s.records[i].Sequence < s.records[j].Sequence })
		m.sessions[sid] = s
	}
	return m.prune()
}

func (m *Manager) prune() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		s.mu.Lock()
		for len(s.records) > 0 && (len(s.records) > 500 || time.Since(s.records[0].StartedAt) > retention) {
			if err := m.store.remove(m.store.exchangePath(id, s.records[0].ID)); err != nil {
				s.mu.Unlock()
				return err
			}
			s.records = s.records[1:]
		}
		empty := len(s.records) == 0
		s.mu.Unlock()
		if empty && s.info.State == "stopped" && time.Since(s.info.StartedAt) > retention {
			if err := m.store.remove(m.store.sessionPath(id)); err != nil {
				return err
			}
			delete(m.sessions, id)
		}
	}
	return nil
}

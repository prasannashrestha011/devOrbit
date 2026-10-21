package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

const maxEnvFileSize = 1 << 20

var ErrEnvFile = errors.New("environment file must be a regular .env file in the service directory")
var ErrEnvConflict = errors.New("environment file changed; reload before saving")
var envWriteMu sync.Mutex

type EnvFile struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Revision string `json:"revision"`
}

func (s *Service) envRoot(ctx context.Context, id uuid.UUID) (*os.Root, string, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return nil, "", err
	}
	workspace, err := s.workspaces.Get(ctx, item.WorkspaceID)
	if err != nil {
		return nil, "", err
	}
	directory, err := workspaceconfig.ResolveDirectory(workspace.RootPath, item.WorkingDirectory)
	if err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(directory)
	return root, directory, err
}

func validEnvName(name string) bool {
	return name == ".env" || strings.HasPrefix(name, ".env.") && name != ".env." && !strings.ContainsAny(name, `/\`) && !strings.ContainsRune(name, 0)
}

func (s *Service) ListEnvFiles(ctx context.Context, id uuid.UUID) ([]string, error) {
	root, _, err := s.envRoot(ctx, id)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	directory, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if !validEnvName(entry.Name()) {
			continue
		}
		info, err := root.Lstat(entry.Name())
		if err == nil && info.Mode().IsRegular() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func readEnvFile(root *os.Root, name string) (EnvFile, os.FileInfo, error) {
	if !validEnvName(name) {
		return EnvFile{}, nil, ErrEnvFile
	}
	info, err := root.Lstat(name)
	if err != nil {
		return EnvFile{}, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxEnvFileSize {
		return EnvFile{}, nil, ErrEnvFile
	}
	file, err := root.Open(name)
	if err != nil {
		return EnvFile{}, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return EnvFile{}, nil, ErrEnvFile
	}
	data, err := io.ReadAll(io.LimitReader(file, maxEnvFileSize+1))
	if err != nil {
		return EnvFile{}, nil, err
	}
	if len(data) > maxEnvFileSize || !utf8.Valid(data) {
		return EnvFile{}, nil, ErrEnvFile
	}
	hash := sha256.Sum256(data)
	return EnvFile{Name: name, Content: string(data), Revision: hex.EncodeToString(hash[:])}, info, nil
}

func (s *Service) ReadEnvFile(ctx context.Context, id uuid.UUID, name string) (EnvFile, error) {
	root, _, err := s.envRoot(ctx, id)
	if err != nil {
		return EnvFile{}, err
	}
	defer root.Close()
	file, _, err := readEnvFile(root, name)
	return file, err
}

func (s *Service) SaveEnvFile(ctx context.Context, id uuid.UUID, name, content, revision string) (EnvFile, error) {
	if !validEnvName(name) || len(content) > maxEnvFileSize || !utf8.ValidString(content) {
		return EnvFile{}, ErrEnvFile
	}
	if revision == "" {
		return EnvFile{}, ErrEnvConflict
	}
	root, directory, err := s.envRoot(ctx, id)
	if err != nil {
		return EnvFile{}, err
	}
	defer root.Close()
	envWriteMu.Lock()
	defer envWriteMu.Unlock()
	current, info, err := readEnvFile(root, name)
	if err != nil {
		return EnvFile{}, err
	}
	if current.Revision != revision {
		return EnvFile{}, ErrEnvConflict
	}
	temp, err := os.CreateTemp(directory, ".env-edit-*")
	if err != nil {
		return EnvFile{}, err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		_ = temp.Close()
		return EnvFile{}, err
	}
	if _, err := temp.WriteString(content); err != nil {
		_ = temp.Close()
		return EnvFile{}, err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return EnvFile{}, err
	}
	if err := temp.Close(); err != nil {
		return EnvFile{}, err
	}
	latest, latestInfo, err := readEnvFile(root, name)
	if err != nil {
		return EnvFile{}, err
	}
	if latest.Revision != revision || !os.SameFile(info, latestInfo) {
		return EnvFile{}, ErrEnvConflict
	}
	if err := os.Rename(temp.Name(), filepath.Join(directory, name)); err != nil {
		return EnvFile{}, err
	}
	if dir, err := os.Open(directory); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	hash := sha256.Sum256([]byte(content))
	return EnvFile{Name: name, Content: content, Revision: hex.EncodeToString(hash[:])}, nil
}

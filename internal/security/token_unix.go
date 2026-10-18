//go:build !windows

package security

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func privateDirectory(path string) error {
	var parts []string
	for part := filepath.Clean(path); ; part = filepath.Dir(part) {
		parts = append(parts, part)
		if part == filepath.Dir(part) {
			break
		}
	}
	protected := false
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		info, err := os.Lstat(part)
		if err != nil || info.Mode()&os.ModeSymlink != 0 && info.Sys().(*syscall.Stat_t).Uid != 0 {
			return errors.New("token directory redirects through an untrusted symlink")
		}
		actual, err := os.Stat(part)
		if err != nil || !actual.IsDir() || !protected && actual.Mode().Perm()&0022 != 0 && actual.Mode()&os.ModeSticky == 0 {
			return errors.New("token directory has a writable shared ancestor")
		}
		if actual.Sys().(*syscall.Stat_t).Uid == uint32(os.Geteuid()) && actual.Mode().Perm()&0077 == 0 {
			protected = true
		}
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return errors.New("token directory is not private")
	}
	return nil
}

func createToken(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

func openToken(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		_ = f.Close()
		return nil, errors.New("token file is not private")
	}
	return f, nil
}

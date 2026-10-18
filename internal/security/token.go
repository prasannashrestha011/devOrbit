package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

func LoadOrCreateToken(path string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", errors.New("cannot create token directory")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Dir(path))
		if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return "", errors.New("token directory must be private")
		}
	}
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", errors.New("cannot generate authentication token")
	}
	token := hex.EncodeToString(bytes[:])
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		if _, err = io.WriteString(f, token); err != nil {
			_ = f.Close()
			_ = os.Remove(path)
			return "", errors.New("cannot write authentication token")
		}
		if err = f.Close(); err != nil {
			_ = os.Remove(path)
			return "", errors.New("cannot close authentication token")
		}
		return token, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return "", errors.New("cannot create authentication token")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return "", errors.New("authentication token file must be private and regular")
	}
	stored, err := os.ReadFile(path)
	if err != nil || len(stored) != 64 {
		return "", errors.New("cannot read valid authentication token")
	}
	if _, err := hex.DecodeString(string(stored)); err != nil {
		return "", errors.New("authentication token has invalid format")
	}
	return string(stored), nil
}

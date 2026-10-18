package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func PrivateDirectory(path string) error { return privateDirectory(path) }

func LoadOrCreateToken(path string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", errors.New("cannot create token directory")
	}
	if err := privateDirectory(filepath.Dir(path)); err != nil {
		return "", errors.New("token directory must be private")
	}
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", errors.New("cannot generate authentication token")
	}
	token := hex.EncodeToString(bytes[:])
	f, err := createToken(path)
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
	f, err = openToken(path)
	if err != nil {
		return "", errors.New("authentication token file must be private and regular")
	}
	stored, err := io.ReadAll(io.LimitReader(f, 65))
	closeErr := f.Close()
	if err != nil || closeErr != nil || len(stored) != 64 {
		return "", errors.New("cannot read valid authentication token")
	}
	if _, err := hex.DecodeString(string(stored)); err != nil {
		return "", errors.New("authentication token has invalid format")
	}
	return string(stored), nil
}

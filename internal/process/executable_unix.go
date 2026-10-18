//go:build !windows

package process

import (
	"os"
)

func executableFile(path string, _ []string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", ErrInvalid
	}
	return path, nil
}

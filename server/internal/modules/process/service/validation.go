package service

import (
	"path/filepath"
	"strings"

	"github.com/prasannashrestha011/devOrbit/internal/process"
)

func resolveDirectory(root, directory string) (string, error) {
	if filepath.IsAbs(directory) {
		return directory, nil
	}
	path := filepath.Join(root, directory)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", process.ErrInvalid
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", process.ErrInvalid
	}
	return resolved, nil
}

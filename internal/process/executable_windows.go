//go:build windows

package process

import (
	"os"
	"path/filepath"
	"strings"
)

func executableFile(path string, env []string) (string, error) {
	extensions := []string{".com", ".exe"}
	for _, pair := range env {
		key, value, ok := strings.Cut(pair, "=")
		if ok && strings.EqualFold(key, "PATHEXT") {
			extensions = nil
			for _, ext := range strings.Split(strings.ToLower(value), ";") {
				if ext == ".exe" || ext == ".com" {
					extensions = append(extensions, ext)
				}
			}
		}
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".bat" || ext == ".cmd" {
		return "", ErrInvalid
	}
	candidates := []string{path}
	if filepath.Ext(path) == "" {
		for _, ext := range extensions {
			candidates = append(candidates, path+ext)
		}
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", ErrInvalid
}

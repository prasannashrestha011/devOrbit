package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Desktop launchers do not read shell startup files where SDKs extend PATH.
func loadShellPath(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, shell, "-ilc", `printf '\000DEVORBIT_PATH\000%s\000' "$PATH"`)
	cmd.Dir = home
	cmd.WaitDelay = 100 * time.Millisecond
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	// Startup banners must not become part of PATH or backend readiness output.
	const marker = "\x00DEVORBIT_PATH\x00"
	_, value, found := strings.Cut(string(output), marker)
	path, _, terminated := strings.Cut(value, "\x00")
	if !found || !terminated || path == "" {
		return errors.New("shell PATH unavailable")
	}
	return os.Setenv("PATH", path)
}

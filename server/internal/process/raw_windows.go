//go:build windows

package process

import "github.com/phuslu/pty"

func makeTerminalRaw(pty.Pty) error { return nil }

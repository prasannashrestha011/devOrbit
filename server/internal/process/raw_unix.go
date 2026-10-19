//go:build !windows

package process

import (
	"github.com/phuslu/pty"
	"golang.org/x/term"
)

func makeTerminalRaw(ptmx pty.Pty) error {
	_, err := term.MakeRaw(int(ptmx.Fd()))
	return err
}

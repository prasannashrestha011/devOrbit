//go:build windows

package process

import "os/exec"

type tmuxSession struct{}

func startTmux(*exec.Cmd, string) (*tmuxSession, *exec.Cmd, error) { return nil, nil, ErrInvalid }
func (*tmuxSession) start() error                                  { return ErrInvalid }
func (*tmuxSession) panePID() (int, error)                         { return 0, ErrInvalid }
func (*tmuxSession) result() (int, bool)                           { return 0, false }
func (*tmuxSession) acknowledge() error                            { return nil }
func (*tmuxSession) interrupt() error                              { return nil }
func (*tmuxSession) kill()                                         {}
func (*tmuxSession) cleanup()                                      {}

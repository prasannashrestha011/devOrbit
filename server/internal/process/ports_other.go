//go:build !linux && !darwin && !windows

package process

func ListeningPorts(int) []int { return nil }

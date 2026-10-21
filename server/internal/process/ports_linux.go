package process

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ListeningPorts returns TCP ports owned by the process or its current children.
func ListeningPorts(pid int) []int {
	if pid <= 0 {
		return nil
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	children := make(map[int][]int)
	for _, entry := range entries {
		child, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		status, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "status"))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(status), "\n") {
			if parent, ok := strings.CutPrefix(line, "PPid:"); ok {
				parentPID, err := strconv.Atoi(strings.TrimSpace(parent))
				if err == nil {
					children[parentPID] = append(children[parentPID], child)
				}
				break
			}
		}
	}
	inodes := make(map[string]bool)
	for queue := []int{pid}; len(queue) > 0; queue = queue[1:] {
		current := queue[0]
		queue = append(queue, children[current]...)
		fds, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(current), "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(current), "fd", fd.Name()))
			if err == nil && strings.HasPrefix(target, "socket:[") {
				inodes[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] = true
			}
		}
	}
	if len(inodes) == 0 {
		return nil
	}
	ports := make(map[int]bool)
	for _, table := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(table)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 10 || fields[3] != "0A" || !inodes[fields[9]] {
				continue
			}
			if _, portHex, ok := strings.Cut(fields[1], ":"); ok {
				port, err := strconv.ParseInt(portHex, 16, 32)
				if err == nil && port > 0 {
					ports[int(port)] = true
				}
			}
		}
	}
	result := make([]int, 0, len(ports))
	for port := range ports {
		result = append(result, port)
	}
	sort.Ints(result)
	return result
}

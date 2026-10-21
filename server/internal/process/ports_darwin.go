package process

import (
	"context"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ListeningPorts returns TCP ports owned by the process or its current children.
func ListeningPorts(pid int) []int {
	if pid <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=").Output()
	if err != nil {
		return nil
	}
	children := make(map[int][]int)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		child, childErr := strconv.Atoi(fields[0])
		parent, parentErr := strconv.Atoi(fields[1])
		if childErr == nil && parentErr == nil {
			children[parent] = append(children[parent], child)
		}
	}
	ids := make([]string, 0, 4)
	for queue := []int{pid}; len(queue) > 0; queue = queue[1:] {
		current := queue[0]
		ids = append(ids, strconv.Itoa(current))
		queue = append(queue, children[current]...)
	}
	output, err = exec.CommandContext(ctx, "lsof", "-nP", "-a", "-p", strings.Join(ids, ","), "-iTCP", "-sTCP:LISTEN", "-Fn").Output()
	if err != nil {
		return nil
	}
	ports := make(map[int]bool)
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		if separator := strings.LastIndex(line, ":"); separator >= 0 {
			portText := line[separator+1:]
			port, err := strconv.Atoi(portText)
			if err == nil && port > 0 && port <= 65535 {
				ports[port] = true
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

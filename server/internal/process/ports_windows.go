package process

import (
	"context"
	"fmt"
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
	script := fmt.Sprintf(`$ids=@(%d); $processes=Get-CimInstance Win32_Process; do { $next=@($processes | Where-Object { $ids -contains $_.ParentProcessId -and $ids -notcontains $_.ProcessId } | Select-Object -ExpandProperty ProcessId); $ids += $next } while ($next.Count -gt 0); Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $ids -contains $_.OwningProcess } | Select-Object -ExpandProperty LocalPort | Sort-Object -Unique`, pid)
	output, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil
	}
	ports := make([]int, 0)
	for _, line := range strings.Fields(string(output)) {
		port, err := strconv.Atoi(line)
		if err == nil && port > 0 && port <= 65535 {
			ports = append(ports, port)
		}
	}
	sort.Ints(ports)
	return ports
}

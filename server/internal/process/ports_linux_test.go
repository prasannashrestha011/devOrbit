package process

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"testing"
)

func TestListeningPorts(t *testing.T) {
	parent, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	parentPort := parent.Addr().(*net.TCPAddr).Port

	cmd := exec.Command(os.Args[0], "-test.run=^TestListeningPortsChild$")
	cmd.Env = append(os.Environ(), "DEVORBIT_PORT_TEST_CHILD=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close(); _ = cmd.Wait() }()
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatalf("child port unavailable: %v", scanner.Err())
	}
	childPort, err := strconv.Atoi(scanner.Text())
	if err != nil {
		t.Fatal(err)
	}
	if got := ListeningPorts(os.Getpid()); !slices.Contains(got, parentPort) || !slices.Contains(got, childPort) {
		t.Fatalf("parent tree ports = %v, want %d and %d", got, parentPort, childPort)
	}
	if got := ListeningPorts(cmd.Process.Pid); slices.Contains(got, parentPort) || !slices.Contains(got, childPort) {
		t.Fatalf("child ports = %v, want only %d", got, childPort)
	}
}

func TestListeningPortsChild(t *testing.T) {
	if os.Getenv("DEVORBIT_PORT_TEST_CHILD") != "1" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	fmt.Println(listener.Addr().(*net.TCPAddr).Port)
	_, _ = io.Copy(io.Discard, os.Stdin)
}

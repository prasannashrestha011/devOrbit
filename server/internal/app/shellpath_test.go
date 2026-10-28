package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLoadShellPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell startup")
	}
	for _, tc := range []struct {
		name, script string
		fail         bool
	}{
		{"startup PATH", `
[ "$1" = "-ilc" ] || exit 1
printf 'startup banner\n'
export PATH="$HOME/sdk bin:$PATH"
/bin/sh -c "$2"
printf 'logout banner\n'
`, false},
		{"failed shell", "exit 1", true},
		{"missing marker", "printf 'banner only'", true},
		{"empty PATH", `printf '\000DEVORBIT_PATH\000\000'`, true},
		{"unterminated PATH", `printf '\000DEVORBIT_PATH\000/bad'`, true},
		{"hung shell", "exec /bin/sleep 10", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			sdk := filepath.Join(home, "sdk bin")
			if err := os.Mkdir(sdk, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sdk, "flutter"), []byte("#!/bin/sh\nprintf 'flutter found'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			shell := filepath.Join(home, "shell")
			if err := os.WriteFile(shell, []byte("#!/bin/sh\n"+tc.script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("SHELL", shell)
			t.Setenv("PATH", "/usr/bin:/bin")
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			err := loadShellPath(ctx)
			if tc.fail {
				if err == nil || os.Getenv("PATH") != "/usr/bin:/bin" {
					t.Fatalf("failed shell changed PATH: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("flutter", "run", "-d", "linux").Output()
			if err != nil || string(output) != "flutter found" {
				t.Fatalf("SDK command lookup failed: %q, %v", output, err)
			}
		})
	}
}

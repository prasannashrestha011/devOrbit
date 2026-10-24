package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, key := range []string{"DEVORBIT_HOST", "DEVORBIT_PORT", "DEVORBIT_DATA_DIR", "DEVORBIT_LOG_DIR", "DEVORBIT_SHUTDOWN_TIMEOUT", "DEVORBIT_AUTH_TOKEN_PATH"} {
		t.Setenv(key, "")
	}
	t.Chdir(t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	base, _ := os.UserConfigDir()
	if cfg.Host != "127.0.0.1" || cfg.Port != 7456 || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.DataDir != filepath.Join(base, "devorbit") || cfg.LogDir != filepath.Join(cfg.DataDir, "logs") || cfg.AuthTokenPath != filepath.Join(cfg.DataDir, "auth-token") {
		t.Fatalf("unexpected paths: %+v", cfg)
	}
}

func TestLoadValidation(t *testing.T) {
	cases := []struct{ key, value string }{
		{"DEVORBIT_HOST", "0.0.0.0"},
		{"DEVORBIT_PORT", "70000"},
		{"DEVORBIT_SHUTDOWN_TIMEOUT", "0s"},
		{"DEVORBIT_DATA_DIR", "relative"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLoadEnvFile(t *testing.T) {
	for _, fromRoot := range []bool{true, false} {
		t.Run(map[bool]string{true: "repository root", false: "server directory"}[fromRoot], func(t *testing.T) {
			root := t.TempDir()
			server := filepath.Join(root, "server")
			if err := os.Mkdir(server, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(server, ".env"), []byte("# local settings\nDEVORBIT_HOST=127.0.0.1\nDEVORBIT_PORT=7654\nDEVORBIT_SHUTDOWN_TIMEOUT='7s'\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if fromRoot {
				t.Chdir(root)
			} else {
				t.Chdir(server)
			}
			for _, key := range []string{"DEVORBIT_PORT", "DEVORBIT_SHUTDOWN_TIMEOUT"} {
				t.Setenv(key, "")
				if err := os.Unsetenv(key); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("DEVORBIT_HOST", "127.0.0.2")
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Host != "127.0.0.2" || cfg.Port != 7654 || cfg.ShutdownTimeout != 7*time.Second {
				t.Fatalf("unexpected config: host=%s port=%d timeout=%s", cfg.Host, cfg.Port, cfg.ShutdownTimeout)
			}
			if os.Getenv("DEVORBIT_SHUTDOWN_TIMEOUT") != "" {
				t.Fatal(".env values should not be exported to child processes")
			}
		})
	}
}

func TestLoadEnvFileInvalid(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile(".env", []byte("DEVORBIT_SHUTDOWN_TIMEOUT='unterminated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid .env error")
	}
}

func TestDesktopChildIgnoresEnvFile(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DEVORBIT_DESKTOP_CHILD", "1")
	if err := os.WriteFile(".env", []byte("DEVORBIT_PORT=invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("desktop backend should use exported settings only: %v", err)
	}
}

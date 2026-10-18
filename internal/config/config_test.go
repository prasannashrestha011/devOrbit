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
	t.Setenv("DEVORBIT_DATABASE_URL", "postgres://user:secret@localhost/test")
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
		{"DEVORBIT_DATABASE_URL", ""},
		{"DEVORBIT_HOST", "0.0.0.0"},
		{"DEVORBIT_PORT", "70000"},
		{"DEVORBIT_SHUTDOWN_TIMEOUT", "0s"},
		{"DEVORBIT_DATA_DIR", "relative"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			t.Setenv("DEVORBIT_DATABASE_URL", "postgres://localhost/test")
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

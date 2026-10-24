package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Host            string
	Port            int
	DataDir         string
	LogDir          string
	ShutdownTimeout time.Duration
	AuthTokenPath   string
}

func Load() (Config, error) {
	fileValues, err := loadEnv()
	if err != nil {
		return Config{}, err
	}
	get := func(key string) string {
		if val, set := os.LookupEnv(key); set {
			return val
		}
		return fileValues[key]
	}
	value := func(key, fallback string) string {
		if val := get(key); val != "" {
			return val
		}
		return fallback
	}
	dataDir := get("DEVORBIT_DATA_DIR")
	if dataDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return Config{}, fmt.Errorf("locate user config directory: %w", err)
		}
		dataDir = filepath.Join(base, "devorbit")
	}
	port, err := strconv.Atoi(value("DEVORBIT_PORT", "7456"))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("DEVORBIT_PORT must be between 1 and 65535")
	}
	timeout, err := time.ParseDuration(value("DEVORBIT_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("DEVORBIT_SHUTDOWN_TIMEOUT must be a positive duration")
	}
	c := Config{
		Host:            value("DEVORBIT_HOST", "127.0.0.1"),
		Port:            port,
		DataDir:         dataDir,
		LogDir:          value("DEVORBIT_LOG_DIR", filepath.Join(dataDir, "logs")),
		ShutdownTimeout: timeout,
		AuthTokenPath:   value("DEVORBIT_AUTH_TOKEN_PATH", filepath.Join(dataDir, "auth-token")),
	}
	if ip := net.ParseIP(c.Host); ip == nil || !ip.IsLoopback() {
		return Config{}, fmt.Errorf("DEVORBIT_HOST must be a loopback IP address")
	}
	for _, path := range []string{c.DataDir, c.LogDir, c.AuthTokenPath} {
		if !filepath.IsAbs(path) {
			return Config{}, fmt.Errorf("data, log, and token paths must be absolute")
		}
	}
	return c, nil
}

func loadEnv() (map[string]string, error) {
	if os.Getenv("DEVORBIT_DESKTOP_CHILD") == "1" {
		return nil, nil
	}
	for _, path := range []string{filepath.Join("server", ".env"), ".env"} {
		values, err := godotenv.Read(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		return values, nil
	}
	return nil, nil
}

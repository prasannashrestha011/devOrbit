package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/prasannashrestha011/devOrbit/internal/app"
	"github.com/prasannashrestha011/devOrbit/internal/config"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if os.Getenv("DEVORBIT_DESKTOP_CHILD") == "1" {
		go func() {
			_, _ = io.Copy(io.Discard, os.Stdin)
			stop()
		}()
	}
	if err := app.Run(ctx, cfg, log); err != nil {
		log.Error("DevOrbit stopped", "error", err)
		os.Exit(1)
	}
}

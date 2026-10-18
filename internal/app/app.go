package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	"github.com/prasannashrestha011/devOrbit/internal/config"
	"github.com/prasannashrestha011/devOrbit/internal/database"
	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"github.com/prasannashrestha011/devOrbit/internal/routes"
	"github.com/prasannashrestha011/devOrbit/internal/security"
)

func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	for _, dir := range []string{cfg.DataDir, cfg.LogDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("initialize application directory: %w", err)
		}
	}
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	db, err := database.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("database pool unavailable")
	}
	defer sqlDB.Close()
	if err := database.Migrate(db, &model.Workspace{}, &serviceModel.Service{}); err != nil {
		return err
	}
	token, err := security.LoadOrCreateToken(cfg.AuthTokenPath)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	server := &http.Server{
		Addr:              address,
		Handler:           api.Router(log, token, "http://"+address, sqlDB.PingContext, func(r chi.Router) { routes.Register(r, db) }),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	log.Info("DevOrbit API listening", "address", address)
	return serve(ctx, server, listener, cfg.ShutdownTimeout)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			<-done
			return fmt.Errorf("HTTP shutdown failed: %w", err)
		}
		<-done
		return nil
	}
}

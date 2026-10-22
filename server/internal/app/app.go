package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/api"
	"github.com/prasannashrestha011/devOrbit/internal/config"
	"github.com/prasannashrestha011/devOrbit/internal/database"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"github.com/prasannashrestha011/devOrbit/internal/modules/traffic_inspector"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/routes"
	"github.com/prasannashrestha011/devOrbit/internal/security"
	"github.com/prasannashrestha011/devOrbit/internal/shellterminal"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
	"gorm.io/gorm"
)

func Run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	if ip := net.ParseIP(cfg.Host); ip == nil || !ip.IsLoopback() || cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("API listen address must be a loopback IP and valid port")
	}
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
	if err := database.Migrate(db, &model.Workspace{}, &serviceModel.Identity{}); err != nil {
		return err
	}
	if db.Migrator().HasTable("services") {
		if err := db.Migrator().DropTable("services"); err != nil {
			return fmt.Errorf("remove legacy services table: %w", err)
		}
	}
	if err := initializeWorkspaceYAML(db, log); err != nil {
		return fmt.Errorf("initialize workspace YAML: %w", err)
	}
	token, err := security.LoadOrCreateToken(cfg.AuthTokenPath)
	if err != nil {
		return err
	}
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	broker := events.New()
	defer broker.Close()
	traffic, err := traffic_inspector.New(filepath.Join(cfg.DataDir, "traffic"), broker)
	if err != nil {
		return fmt.Errorf("initialize traffic captures: %w", err)
	}
	defer traffic.Close(context.Background())
	stopEvents := context.AfterFunc(ctx, broker.Close)
	defer stopEvents()
	manager, err := logs.New(cfg.LogDir, broker)
	if err != nil {
		return fmt.Errorf("initialize process logs: %w", err)
	}
	supervisor := process.New(cfg.ShutdownTimeout, manager)
	supervisor.SetBroker(broker)
	terminals := shellterminal.New(workspaceService.New(workspaceRepo.New(db), cfg.DataDir), cfg.ShutdownTimeout)
	dockerTerminals := process.New(cfg.ShutdownTimeout)
	server := &http.Server{
		Addr: address,
		Handler: localHost(address, api.Router(log, token, "http://"+address, sqlDB.PingContext, func(r chi.Router) {
			routes.Register(r, db, supervisor, manager, broker, cfg.DataDir, dockerTerminals, traffic, address, terminals)
		})),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		_ = manager.Close()
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	log.Info("DevOrbit API listening", "address", address)
	serveErr := serve(ctx, server, listener, cfg.ShutdownTimeout)
	shutdownCtx, stop := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer stop()
	traffic.Close(shutdownCtx)
	processErr := errors.Join(supervisor.Shutdown(shutdownCtx), terminals.Shutdown(shutdownCtx), dockerTerminals.Shutdown(shutdownCtx))
	logErr := manager.Close()
	return errors.Join(serveErr, processErr, logErr)
}

func initializeWorkspaceYAML(db *gorm.DB, log *slog.Logger) error {
	var workspaces []model.Workspace
	if err := db.Find(&workspaces).Error; err != nil {
		return err
	}
	for _, workspace := range workspaces {
		if err := ensureWorkspaceYAML(workspace.RootPath); err != nil {
			log.Warn("workspace YAML unavailable", "workspace_id", workspace.ID, "error", err)
		}
	}
	return nil
}

func ensureWorkspaceYAML(root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace root %s is not a directory", root)
	}
	if snapshot, err := workspaceconfig.ReadRaw(root); err == nil {
		return workspaceconfig.RemoveServicePorts(root, snapshot)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_, err = workspaceconfig.Write(root, []byte("services: {}\n"), "")
	if errors.Is(err, workspaceconfig.ErrConflict) {
		return nil
	}
	return err
}

func localHost(address string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != address {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":"host_forbidden","message":"host is not allowed"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
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

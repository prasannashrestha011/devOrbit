package routes

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/dockercompose"
	processService "github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
	configRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	"github.com/prasannashrestha011/devOrbit/internal/modules/traffic_inspector"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/orchestration"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/shellterminal"
	"gorm.io/gorm"
	"net/http"
	"time"
)

func RegisterRoutes(r chi.Router, db *gorm.DB, supervisor *process.Supervisor, dataDir string, dockerTerminals *process.Supervisor, traffic *traffic_inspector.Manager, terminals ...*shellterminal.Manager) {
	workspaces := repository.New(db)
	services := configRepo.New(db)
	configs := configService.New(services, workspaces, supervisor)
	c := controller.New(service.New(workspaces, dataDir), supervisor, terminals...)
	c.OnDelete = func(id uuid.UUID) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		traffic.StopWorkspace(ctx, id)
	}
	lifecycle := processService.New(configs, service.New(workspaces, dataDir), supervisor)
	o := controller.NewOrchestration(orchestration.New(workspaces, configs, lifecycle, supervisor))
	docker := dockercompose.NewController(workspaces, dockerTerminals)
	r.Post("/workspaces", c.Create)
	r.Get("/workspaces", c.List)
	r.Get("/workspaces/{id}", c.Get)
	r.Patch("/workspaces/{id}", c.Update)
	r.Delete("/workspaces/{id}", func(w http.ResponseWriter, request *http.Request) {
		id, _ := uuid.Parse(chi.URLParam(request, "id"))
		if docker.Active(id) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":{"code":"docker_busy","message":"finish Docker operations and close container terminals before deleting the workspace"}}`))
			return
		}
		c.Delete(w, request)
	})
	r.Get("/workspaces/{id}/docker", docker.Snapshot)
	r.Get("/workspaces/{id}/docker/services/{service}/logs", docker.Logs)
	r.Post("/workspaces/{id}/docker/terminals", docker.CreateTerminal)
	r.Get("/workspaces/{id}/docker/terminals/{terminalID}/stream", docker.StreamTerminal)
	r.Delete("/workspaces/{id}/docker/terminals/{terminalID}", docker.DeleteTerminal)
	r.Post("/workspaces/{id}/docker/operations", docker.Start)
	r.Get("/workspaces/{id}/docker/operations/{operationID}", docker.Operation)
	r.Delete("/workspaces/{id}/docker/operations/{operationID}", docker.Operation)
	r.Post("/workspaces/{id}/start", o.Start)
	r.Post("/workspaces/{id}/stop", o.Stop)
	r.Get("/workspaces/{id}/status", o.Status)
}

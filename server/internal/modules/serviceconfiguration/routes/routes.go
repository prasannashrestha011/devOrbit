package routes

import (
	"context"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	"github.com/prasannashrestha011/devOrbit/internal/modules/traffic_inspector"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"gorm.io/gorm"
)

func RegisterRoutes(r chi.Router, db *gorm.DB, supervisor *process.Supervisor, traffic *traffic_inspector.Manager) {
	services := service.New(repository.New(db), workspaceRepository.New(db), supervisor)
	c := controller.New(services)
	c.OnDelete = func(workspaceID uuid.UUID, serviceID uuid.UUID) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		traffic.StopService(ctx, workspaceID, serviceID.String())
	}
	c.OnChange = func(workspaceID uuid.UUID) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, session := range traffic.List(workspaceID) {
			if session.Target.Kind != "local" {
				continue
			}
			id, err := uuid.Parse(session.Target.ServiceKey)
			if err != nil {
				_ = traffic.Stop(ctx, workspaceID, session.ID)
				continue
			}
			item, err := services.Get(ctx, id)
			if err != nil || item == nil || item.Ports[session.Target.PortKey] == 0 {
				_ = traffic.Stop(ctx, workspaceID, session.ID)
				continue
			}
			if session.Upstream != "http://127.0.0.1:"+strconv.Itoa(item.Ports[session.Target.PortKey]) {
				_ = traffic.Stop(ctx, workspaceID, session.ID)
			}
		}
	}
	r.Get("/workspaces/{id}/configuration", c.Configuration)
	r.Post("/workspaces/{id}/configuration/validate", c.Validate)
	r.Put("/workspaces/{id}/configuration", c.Save)
	r.Post("/workspaces/{id}/services", c.Create)
	r.Get("/workspaces/{id}/services", c.List)
	r.Get("/services/{id}", c.Get)
	r.Patch("/services/{id}", c.Update)
	r.Delete("/services/{id}", c.Delete)
	r.Get("/services/{id}/environment", c.ListEnvironment)
	r.Get("/services/{id}/environment/{name}", c.ReadEnvironment)
	r.Put("/services/{id}/environment/{name}", c.SaveEnvironment)
}

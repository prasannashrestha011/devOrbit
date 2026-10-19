package routes

import (
	"github.com/go-chi/chi/v5"
	processService "github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
	configRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/orchestration"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"gorm.io/gorm"
)

func RegisterRoutes(r chi.Router, db *gorm.DB, supervisor *process.Supervisor) {
	workspaces := repository.New(db)
	services := configRepo.New(db)
	c := controller.New(service.New(workspaces))
	lifecycle := processService.New(configService.New(services, workspaces), service.New(workspaces), supervisor)
	o := controller.NewOrchestration(orchestration.New(workspaces, services, lifecycle, supervisor))
	r.Post("/workspaces", c.Create)
	r.Get("/workspaces", c.List)
	r.Get("/workspaces/{id}", c.Get)
	r.Patch("/workspaces/{id}", c.Update)
	r.Delete("/workspaces/{id}", c.Delete)
	r.Post("/workspaces/{id}/start", o.Start)
	r.Post("/workspaces/{id}/stop", o.Stop)
	r.Get("/workspaces/{id}/status", o.Status)
}

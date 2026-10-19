package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/modules/process/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
	serviceconfig "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"gorm.io/gorm"
)

func RegisterRoutes(r chi.Router, db *gorm.DB, supervisor *process.Supervisor) {
	workspaces := workspaceService.New(workspaceRepo.New(db))
	configs := configService.New(serviceconfig.New(db), workspaceRepo.New(db))
	c := controller.New(service.New(configs, workspaces, supervisor))
	r.Post("/services/{id}/start", c.Start)
	r.Post("/services/{id}/stop", c.Stop)
	r.Post("/services/{id}/restart", c.Restart)
	r.Get("/services/{id}/status", c.Status)
	r.Get("/services/{id}/terminal/{execution}", c.Terminal)
}

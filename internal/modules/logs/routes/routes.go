package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
	"github.com/prasannashrestha011/devOrbit/internal/modules/logs/controller"
	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	serviceService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"gorm.io/gorm"
)

func RegisterRoutes(r chi.Router, db *gorm.DB, manager *logs.Manager) {
	workspaces := workspaceService.New(workspaceRepo.New(db))
	configs := serviceService.New(serviceRepo.New(db), workspaceRepo.New(db))
	c := controller.New(manager, configs, workspaces)
	r.Get("/services/{id}/logs", c.Service)
	r.Get("/workspaces/{id}/logs", c.Workspace)
}

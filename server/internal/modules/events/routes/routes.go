package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	"github.com/prasannashrestha011/devOrbit/internal/modules/events/controller"
	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	serviceService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"gorm.io/gorm"
)

func RegisterRoutes(r chi.Router, db *gorm.DB, broker *events.Broker) {
	workspaces := workspaceService.New(workspaceRepo.New(db))
	services := serviceService.New(serviceRepo.New(db), workspaceRepo.New(db))
	r.Get("/events", controller.New(broker, services, workspaces).Stream)
}

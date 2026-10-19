package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"gorm.io/gorm"
)

func RegisterRoutes(r chi.Router, db *gorm.DB) {
	c := controller.New(service.New(repository.New(db), workspaceRepository.New(db)))
	r.Post("/workspaces/{id}/services", c.Create)
	r.Get("/workspaces/{id}/services", c.List)
	r.Get("/services/{id}", c.Get)
	r.Patch("/services/{id}", c.Update)
	r.Delete("/services/{id}", c.Delete)
}

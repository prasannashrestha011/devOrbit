package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"gorm.io/gorm"
)

func RegisterRoutes(r chi.Router, db *gorm.DB) {
	c := controller.New(service.New(repository.New(db)))
	r.Post("/workspaces", c.Create)
	r.Get("/workspaces", c.List)
	r.Get("/workspaces/{id}", c.Get)
	r.Patch("/workspaces/{id}", c.Update)
	r.Delete("/workspaces/{id}", c.Delete)
}

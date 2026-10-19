package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/modules/shellterminal/controller"
	"github.com/prasannashrestha011/devOrbit/internal/shellterminal"
)

func RegisterRoutes(r chi.Router, manager *shellterminal.Manager) {
	c := controller.New(manager)
	r.Get("/workspaces/{workspace}/terminals", c.List)
	r.Post("/workspaces/{workspace}/terminals", c.Create)
	r.Delete("/workspaces/{workspace}/terminals/{terminal}", c.Delete)
	r.Get("/workspaces/{workspace}/terminals/{terminal}/stream", c.Stream)
}

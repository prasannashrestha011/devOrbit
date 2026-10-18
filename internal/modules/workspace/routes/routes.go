package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/database"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/controller"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"gorm.io/gorm"
)

func New(db *gorm.DB) http.Handler {
	c := controller.New(service.New(service.NewRepository))
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(database.WithDB(req.Context(), db)))
		})
	})
	r.Post("/", c.Create)
	r.Get("/", c.List)
	r.Get("/{id}", c.Get)
	r.Patch("/{id}", c.Update)
	r.Delete("/{id}", c.Delete)
	return r
}

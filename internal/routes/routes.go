package routes

import (
	"github.com/go-chi/chi/v5"
	workspaceRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/routes"
	"gorm.io/gorm"
)

// Register passes the authenticated API router to each module.
func Register(r chi.Router, db *gorm.DB) {
	workspaceRoutes.RegisterRoutes(r, db)
}

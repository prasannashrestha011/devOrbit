package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
	eventRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/events/routes"
	logRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/logs/routes"
	processRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/process/routes"
	serviceRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/routes"
	workspaceRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/routes"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"gorm.io/gorm"
)

// Register passes the authenticated API router to each module.
func Register(r chi.Router, db *gorm.DB, supervisor *process.Supervisor, manager *logs.Manager, broker *events.Broker) {
	workspaceRoutes.RegisterRoutes(r, db, supervisor)
	serviceRoutes.RegisterRoutes(r, db)
	processRoutes.RegisterRoutes(r, db, supervisor)
	logRoutes.RegisterRoutes(r, db, manager)
	eventRoutes.RegisterRoutes(r, db, broker)
}

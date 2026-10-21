package routes

import (
	"github.com/go-chi/chi/v5"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
	"github.com/prasannashrestha011/devOrbit/internal/modules/dockercompose"
	eventRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/events/routes"
	logRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/logs/routes"
	processRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/process/routes"
	serviceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/repository"
	serviceRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/routes"
	serviceService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	shellRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/shellterminal/routes"
	"github.com/prasannashrestha011/devOrbit/internal/modules/traffic_inspector"
	workspaceRepo "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	workspaceRoutes "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/routes"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/shellterminal"
	"gorm.io/gorm"
)

// Register passes the authenticated API router to each module.
func Register(r chi.Router, db *gorm.DB, supervisor *process.Supervisor, manager *logs.Manager, broker *events.Broker, dataDir string, dockerTerminals *process.Supervisor, traffic *traffic_inspector.Manager, apiHost string, terminals ...*shellterminal.Manager) {
	workspaceRoutes.RegisterRoutes(r, db, supervisor, dataDir, dockerTerminals, traffic, terminals...)
	serviceRoutes.RegisterRoutes(r, db, supervisor, traffic)
	processRoutes.RegisterRoutes(r, db, supervisor)
	logRoutes.RegisterRoutes(r, db, manager)
	eventRoutes.RegisterRoutes(r, db, broker)
	(&traffic_inspector.Controller{Manager: traffic, Resolver: traffic_inspector.Resolver{Workspaces: workspaceRepo.New(db), Services: serviceService.New(serviceRepo.New(db), workspaceRepo.New(db)), Processes: supervisor, Docker: dockercompose.New(), APIHost: apiHost}}).Register(r)
	if len(terminals) > 0 {
		shellRoutes.RegisterRoutes(r, terminals[0])
	}
}

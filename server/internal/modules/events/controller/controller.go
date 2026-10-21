package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/events"
	serviceModel "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	serviceconfig "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
)

type serviceLookup interface {
	Identity(context.Context, uuid.UUID) (*serviceModel.Identity, error)
}

type workspaceLookup interface {
	Get(context.Context, uuid.UUID) (*workspaceModel.Workspace, error)
}

type Controller struct {
	broker     *events.Broker
	services   serviceLookup
	workspaces workspaceLookup
}

func New(broker *events.Broker, services serviceLookup, workspaces workspaceLookup) *Controller {
	return &Controller{broker: broker, services: services, workspaces: workspaces}
}

func (c *Controller) Stream(w http.ResponseWriter, r *http.Request) {
	if err := validateFilterQuery(r.URL.Query()); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	filter := events.Filter{}
	if value := r.URL.Query().Get("workspace_id"); value != "" {
		id, err := parseFilterID(value, "workspace_id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if _, err := c.workspaces.Get(r.Context(), id); err != nil {
			writeLookupError(w, workspace.IsNotFound(err), "workspace")
			return
		}
		filter.WorkspaceID = id
	}
	if value := r.URL.Query().Get("service_id"); value != "" {
		id, err := parseFilterID(value, "service_id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		service, err := c.services.Identity(r.Context(), id)
		if err != nil {
			writeLookupError(w, serviceconfig.IsNotFound(err), "service")
			return
		}
		if filter.WorkspaceID != uuid.Nil && service.WorkspaceID != filter.WorkspaceID {
			writeError(w, http.StatusBadRequest, "invalid_request", "service is outside workspace")
			return
		}
		filter.ServiceID = id
	}
	stream, unsubscribe := c.broker.Subscribe(filter)
	defer unsubscribe()
	response := http.NewResponseController(w)
	// A fresh deadline for each frame keeps the stream live without pinning shutdown on a stalled reader.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	if err := response.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return
	}
	if _, err := io.WriteString(w, ": connected\n\n"); err != nil || response.Flush() != nil {
		return
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case event, ok := <-stream:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				return
			}
			if err := response.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, data); err != nil || response.Flush() != nil {
				return
			}
		case <-heartbeat.C:
			if err := response.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
				return
			}
			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil || response.Flush() != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func writeLookupError(w http.ResponseWriter, missing bool, name string) {
	if missing {
		writeError(w, http.StatusNotFound, "not_found", name+" not found")
	} else {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}

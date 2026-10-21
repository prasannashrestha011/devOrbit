package controller

import (
	"encoding/json"
	"net/http"

	"github.com/prasannashrestha011/devOrbit/internal/logs"
	"github.com/prasannashrestha011/devOrbit/internal/modules/logs/dto"
	configService "github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	workspaceService "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
)

type Controller struct {
	logs       *logs.Manager
	configs    *configService.Service
	workspaces *workspaceService.Service
}

func New(manager *logs.Manager, configs *configService.Service, workspaces *workspaceService.Service) *Controller {
	return &Controller{logs: manager, configs: configs, workspaces: workspaces}
}

func (c *Controller) Service(w http.ResponseWriter, r *http.Request) {
	id, filter, err := parseQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, err := c.configs.Identity(r.Context(), id); err != nil {
		if configService.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "service not found")
		} else {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}
	filter.ServiceID = id
	entries, err := c.logs.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, dto.NewListResponse(entries, c.logs.Diagnostics(id)))
}

func (c *Controller) Workspace(w http.ResponseWriter, r *http.Request) {
	id, filter, err := parseQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if _, err := c.workspaces.Get(r.Context(), id); err != nil {
		if workspaceService.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "workspace not found")
		} else {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}
	filter.WorkspaceID = id
	entries, err := c.logs.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, dto.NewListResponse(entries, logs.Diagnostics{}))
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, name, message string) {
	writeJSON(w, code, map[string]any{"error": map[string]string{"code": name, "message": message}})
}

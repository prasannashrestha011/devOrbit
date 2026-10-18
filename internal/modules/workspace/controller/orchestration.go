package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/orchestration"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

type OrchestrationController struct{ orchestrator *orchestration.Orchestrator }

func NewOrchestration(o *orchestration.Orchestrator) *OrchestrationController {
	return &OrchestrationController{orchestrator: o}
}

func (c *OrchestrationController) Start(w http.ResponseWriter, r *http.Request) {
	id, ok := orchestrationID(w, r)
	if !ok {
		return
	}
	result, err := c.orchestrator.StartWorkspace(r.Context(), id)
	if err != nil {
		orchestrationError(w, err)
		return
	}
	orchestrationJSON(w, http.StatusOK, result)
}

func (c *OrchestrationController) Stop(w http.ResponseWriter, r *http.Request) {
	id, ok := orchestrationID(w, r)
	if !ok {
		return
	}
	result, err := c.orchestrator.StopWorkspace(r.Context(), id)
	if err != nil {
		orchestrationError(w, err)
		return
	}
	orchestrationJSON(w, http.StatusOK, result)
}

func (c *OrchestrationController) Status(w http.ResponseWriter, r *http.Request) {
	id, ok := orchestrationID(w, r)
	if !ok {
		return
	}
	result, err := c.orchestrator.GetWorkspaceStatus(r.Context(), id)
	if err != nil {
		orchestrationError(w, err)
		return
	}
	orchestrationJSON(w, http.StatusOK, result)
}

func orchestrationID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || id == uuid.Nil {
		orchestrationError(w, orchestration.ErrInvalidID)
		return uuid.Nil, false
	}
	return id, true
}

func orchestrationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, orchestration.ErrInvalidID):
		orchestrationErrorJSON(w, http.StatusBadRequest, "invalid_request", "invalid id")
	case service.IsNotFound(err):
		orchestrationErrorJSON(w, http.StatusNotFound, "not_found", "workspace not found")
	case errors.Is(err, orchestration.ErrConflict), errors.Is(err, process.ErrClosed):
		orchestrationErrorJSON(w, http.StatusConflict, "conflict", "workspace lifecycle operation is already active or shutting down")
	default:
		orchestrationErrorJSON(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func orchestrationJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func orchestrationErrorJSON(w http.ResponseWriter, status int, code, message string) {
	orchestrationJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

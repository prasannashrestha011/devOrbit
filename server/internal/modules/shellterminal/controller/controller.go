package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/shellterminal"
	"github.com/prasannashrestha011/devOrbit/internal/terminalstream"
)

type Controller struct{ manager *shellterminal.Manager }

func New(manager *shellterminal.Manager) *Controller { return &Controller{manager: manager} }

func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseID(w, chi.URLParam(r, "workspace"))
	if !ok {
		return
	}
	sessions, err := c.manager.List(r.Context(), workspaceID)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseID(w, chi.URLParam(r, "workspace"))
	if !ok {
		return
	}
	session, err := c.manager.Create(r.Context(), workspaceID)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID, id, ok := parseSessionIDs(w, r)
	if !ok {
		return
	}
	if err := c.manager.Delete(r.Context(), workspaceID, id); err != nil {
		writeOperationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) Stream(w http.ResponseWriter, r *http.Request) {
	workspaceID, id, ok := parseSessionIDs(w, r)
	if !ok {
		return
	}
	after, err := terminalstream.After(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid sequence")
		return
	}
	terminal, err := c.manager.Terminal(workspaceID, id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	terminalstream.Serve(w, r, terminal, after,
		func(data []byte) error { return c.manager.Input(workspaceID, id, data) },
		func(cols, rows uint16) error { return c.manager.Resize(workspaceID, id, cols, rows) },
	)
}

func parseID(w http.ResponseWriter, raw string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func parseSessionIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	workspaceID, ok := parseID(w, chi.URLParam(r, "workspace"))
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, ok := parseID(w, chi.URLParam(r, "terminal"))
	return workspaceID, id, ok
}

func writeOperationError(w http.ResponseWriter, err error) {
	switch {
	case workspace.IsNotFound(err), errors.Is(err, shellterminal.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "workspace or terminal not found")
	case errors.Is(err, shellterminal.ErrLimit):
		writeError(w, http.StatusConflict, "terminal_limit", "workspace terminal limit reached")
	case errors.Is(err, process.ErrInvalid):
		writeError(w, http.StatusUnprocessableEntity, "terminal_unavailable", "shell or home directory unavailable")
	case errors.Is(err, process.ErrClosed):
		writeError(w, http.StatusServiceUnavailable, "shutting_down", "terminal manager is shutting down")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "terminal operation failed")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

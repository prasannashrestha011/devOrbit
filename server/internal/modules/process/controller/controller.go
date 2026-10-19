package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
)

type Controller struct{ service *service.Service }

func New(s *service.Service) *Controller { return &Controller{service: s} }

func (c *Controller) Start(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	status, err := c.service.Start(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, status)
}

func (c *Controller) Stop(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	status, err := c.service.Stop(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (c *Controller) Restart(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	status, err := c.service.Restart(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (c *Controller) Status(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	status, err := c.service.Status(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

func writeOperationError(w http.ResponseWriter, err error) {
	switch {
	case service.IsNotFound(err):
		writeError(w, http.StatusNotFound, "not_found", "workspace or service not found")
	case errors.Is(err, process.ErrActive), errors.Is(err, service.ErrDisabled), errors.Is(err, process.ErrClosed):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, process.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_request", "service execution configuration is invalid")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, code int, name, message string) {
	writeJSON(w, code, map[string]any{"error": map[string]string{"code": name, "message": message}})
}

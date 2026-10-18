package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/httpinput"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/dto"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
)

type Controller struct{ service *service.Service }

func New(s *service.Service) *Controller { return &Controller{service: s} }

func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseID(w, r)
	if !ok {
		return
	}
	input, err := dto.DecodeCreate(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := c.service.Create(r.Context(), workspaceID, input)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.FromModel(item))
}

func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseID(w, r)
	if !ok {
		return
	}
	limit, offset, err := httpinput.Pagination(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	items, err := c.service.List(r.Context(), workspaceID, limit, offset)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromModels(items))
}

func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	item, err := c.service.Get(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromModel(item))
}

func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	input, err := dto.DecodePatch(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := c.service.Update(r.Context(), id, input)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromModel(item))
}

func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := c.service.Delete(r.Context(), id); err != nil {
		writeOperationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	var validation *service.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, "invalid_request", validation.Error())
	case service.IsNotFound(err):
		writeError(w, http.StatusNotFound, "not_found", "workspace or service not found")
	case service.IsDuplicate(err):
		writeError(w, http.StatusConflict, "conflict", "service name already exists in workspace")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
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

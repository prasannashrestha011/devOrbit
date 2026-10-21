package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/httpinput"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/dto"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/shellterminal"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type Controller struct {
	service   *service.Service
	terminals *shellterminal.Manager
	runtime   *process.Supervisor
	OnDelete  func(uuid.UUID)
}

func New(s *service.Service, runtime *process.Supervisor, terminals ...*shellterminal.Manager) *Controller {
	c := &Controller{service: s, runtime: runtime}
	if len(terminals) > 0 {
		c.terminals = terminals[0]
	}
	return c
}

func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	input, err := dto.DecodeCreate(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
		return
	}
	workspace, err := c.service.Create(r.Context(), input)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.FromModel(workspace))
}

func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpinput.Pagination(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	workspaces, err := c.service.List(r.Context(), limit, offset)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromModels(workspaces))
}

func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	workspace, err := c.service.Get(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromModel(workspace))
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
	workspace, err := c.service.Update(r.Context(), id, input)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.FromModel(workspace))
}

func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if c.runtime != nil {
		for _, status := range c.runtime.WorkspaceStatuses(id) {
			if status.State == process.Running || status.State == process.Starting || status.State == process.Stopping {
				writeError(w, http.StatusConflict, "conflict", "stop workspace services before deleting the workspace")
				return
			}
		}
	}
	var err error
	if c.terminals != nil {
		err = c.terminals.DeleteWorkspace(r.Context(), id)
	} else {
		err = c.service.Delete(r.Context(), id)
	}
	if err != nil {
		writeOperationError(w, err)
		return
	}
	if c.OnDelete != nil {
		c.OnDelete(id)
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid workspace id")
		return uuid.Nil, false
	}
	return id, true
}

func writeOperationError(w http.ResponseWriter, err error) {
	var validation *service.ValidationError
	if errors.As(err, &validation) {
		writeError(w, http.StatusBadRequest, "invalid_request", validation.Error())
		return
	}
	if service.IsNotFound(err) {
		writeError(w, http.StatusNotFound, "not_found", "workspace not found")
		return
	}
	if errors.Is(err, repository.ErrConflict) {
		writeError(w, http.StatusConflict, "conflict", "workspace contains services")
		return
	}
	if errors.Is(err, workspaceconfig.ErrConflict) {
		writeError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	if errors.Is(err, shellterminal.ErrHasSessions) {
		writeError(w, http.StatusConflict, "conflict", "close workspace terminals before deleting the workspace")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

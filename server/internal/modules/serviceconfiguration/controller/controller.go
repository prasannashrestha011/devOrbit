package controller

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/httpinput"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/dto"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
	"github.com/prasannashrestha011/devOrbit/internal/workspaceconfig"
)

type Controller struct {
	service  *service.Service
	OnDelete func(uuid.UUID, uuid.UUID)
	OnChange func(uuid.UUID)
}

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
	if c.OnChange != nil {
		c.OnChange(item.WorkspaceID)
	}
	writeJSON(w, http.StatusOK, dto.FromModel(item))
}

func (c *Controller) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	identity, err := c.service.Identity(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	if err := c.service.Delete(r.Context(), id, r.URL.Query().Get("revision")); err != nil {
		writeOperationError(w, err)
		return
	}
	if c.OnDelete != nil {
		c.OnDelete(identity.WorkspaceID, id)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) Configuration(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	snapshot, err := c.service.RawSnapshot(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (c *Controller) Validate(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
		return
	}
	config, err := c.service.Validate(r.Context(), id, input.Content)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (c *Controller) Save(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content  string `json:"content"`
		Revision string `json:"revision"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
		return
	}
	snapshot, err := c.service.Save(r.Context(), id, input.Content, input.Revision)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	if c.OnChange != nil {
		c.OnChange(id)
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (c *Controller) ListEnvironment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	names, err := c.service.ListEnvFiles(r.Context(), id)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, names)
}

func (c *Controller) ReadEnvironment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	file, err := c.service.ReadEnvFile(r.Context(), id, chi.URLParam(r, "name"))
	if err != nil {
		writeEnvironmentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (c *Controller) SaveEnvironment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content  *string `json:"content"`
		Revision string  `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20+4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid environment file request")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid environment file request")
		return
	}
	if input.Content == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "environment file content is required")
		return
	}
	file, err := c.service.SaveEnvFile(r.Context(), id, chi.URLParam(r, "name"), *input.Content, input.Revision)
	if err != nil {
		writeEnvironmentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func writeEnvironmentError(w http.ResponseWriter, err error) {
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "not_found", "environment file not found")
		return
	}
	writeOperationError(w, err)
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
	case errors.Is(err, service.ErrEnvFile):
		writeError(w, http.StatusBadRequest, "invalid_environment_file", "select a regular UTF-8 .env file (up to 1 MiB) in the service directory")
	case errors.Is(err, service.ErrEnvConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, os.ErrPermission):
		writeError(w, http.StatusForbidden, "permission_denied", "environment file access denied")
	case service.IsDuplicate(err):
		writeError(w, http.StatusConflict, "conflict", "service name already exists in workspace")
	case service.IsConflict(err):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, workspaceconfig.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid_configuration", err.Error())
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

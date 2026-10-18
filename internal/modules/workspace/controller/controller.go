package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/service"
)

type Controller struct{ service *service.Service }

func New(s *service.Service) *Controller { return &Controller{service: s} }

func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RootPath    string `json:"root_path"`
	}
	if err := decode(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
		return
	}
	workspace, err := c.service.Create(r.Context(), service.CreateInput(input))
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, workspace)
}

func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := 50, 0
	values := r.URL.Query()
	if values.Has("limit") {
		parsed, err := strconv.Atoi(values.Get("limit"))
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	if values.Has("offset") {
		parsed, err := strconv.Atoi(values.Get("offset"))
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "offset must be nonnegative")
			return
		}
		offset = parsed
	}
	workspaces, err := c.service.List(r.Context(), limit, offset)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workspaces)
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
	writeJSON(w, http.StatusOK, workspace)
}

func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var fields map[string]json.RawMessage
	if err := decode(r, &fields); err != nil || fields == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
		return
	}
	var input service.PatchInput
	for field, raw := range fields {
		if bytes.Equal(raw, []byte("null")) {
			writeError(w, http.StatusBadRequest, "invalid_request", "null field value is not supported")
			return
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "fields must be strings")
			return
		}
		switch field {
		case "name":
			input.Name = &value
		case "description":
			input.Description = &value
		case "root_path":
			input.RootPath = &value
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", "unsupported workspace field")
			return
		}
	}
	workspace, err := c.service.Update(r.Context(), id, input)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workspace)
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
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid workspace id")
		return uuid.Nil, false
	}
	return id, true
}

func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing JSON value")
	}
	return nil
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

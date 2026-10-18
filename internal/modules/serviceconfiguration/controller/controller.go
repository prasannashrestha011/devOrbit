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
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/service"
)

type Controller struct{ service *service.Service }

func New(s *service.Service) *Controller { return &Controller{service: s} }

func (c *Controller) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseID(w, r)
	if !ok {
		return
	}
	fields, ok := readFields(w, r)
	if !ok {
		return
	}
	var input service.CreateInput
	for field, raw := range fields {
		switch field {
		case "name":
			if !readValue(w, raw, &input.Name) {
				return
			}
		case "command":
			if !readValue(w, raw, &input.Command) {
				return
			}
		case "args":
			if !readValue(w, raw, &input.Args) {
				return
			}
		case "working_directory":
			if !readValue(w, raw, &input.WorkingDirectory) {
				return
			}
		case "env":
			if !readValue(w, raw, &input.Env) {
				return
			}
		case "enabled":
			input.Enabled = new(bool)
			if !readValue(w, raw, input.Enabled) {
				return
			}
		case "auto_restart":
			input.AutoRestart = new(bool)
			if !readValue(w, raw, input.AutoRestart) {
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", "unsupported service field")
			return
		}
	}
	item, err := c.service.Create(r.Context(), workspaceID, input)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseID(w, r)
	if !ok {
		return
	}
	limit, offset, ok := pagination(w, r)
	if !ok {
		return
	}
	items, err := c.service.List(r.Context(), workspaceID, limit, offset)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
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
	writeJSON(w, http.StatusOK, item)
}

func (c *Controller) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	fields, ok := readFields(w, r)
	if !ok {
		return
	}
	var input service.PatchInput
	for field, raw := range fields {
		switch field {
		case "name":
			input.Name = new(string)
			if !readValue(w, raw, input.Name) {
				return
			}
		case "command":
			input.Command = new(string)
			if !readValue(w, raw, input.Command) {
				return
			}
		case "args":
			input.Args = new([]string)
			if !readValue(w, raw, input.Args) {
				return
			}
		case "working_directory":
			input.WorkingDirectory = new(string)
			if !readValue(w, raw, input.WorkingDirectory) {
				return
			}
		case "env":
			input.Env = new(map[string]string)
			if !readValue(w, raw, input.Env) {
				return
			}
		case "enabled":
			input.Enabled = new(bool)
			if !readValue(w, raw, input.Enabled) {
				return
			}
		case "auto_restart":
			input.AutoRestart = new(bool)
			if !readValue(w, raw, input.AutoRestart) {
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", "unsupported service field")
			return
		}
	}
	item, err := c.service.Update(r.Context(), id, input)
	if err != nil {
		writeOperationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
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

func readFields(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request")
		return nil, false
	}
	return fields, true
}

func readValue(w http.ResponseWriter, raw json.RawMessage, target any) bool {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, target) != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid service field value")
		return false
	}
	return true
}

func pagination(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	limit, offset := 50, 0
	values := r.URL.Query()
	if values.Has("limit") {
		parsed, err := strconv.Atoi(values.Get("limit"))
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be between 1 and 100")
			return 0, 0, false
		}
		limit = parsed
	}
	if values.Has("offset") {
		parsed, err := strconv.Atoi(values.Get("offset"))
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid_request", "offset must be nonnegative")
			return 0, 0, false
		}
		offset = parsed
	}
	return limit, offset, true
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

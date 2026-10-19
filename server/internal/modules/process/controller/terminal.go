package controller

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/process/service"
	"github.com/prasannashrestha011/devOrbit/internal/process"
	"github.com/prasannashrestha011/devOrbit/internal/terminalstream"
)

func (c *Controller) Terminal(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	executionID, err := uuid.Parse(chi.URLParam(r, "execution"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid execution id")
		return
	}
	after, err := terminalstream.After(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid sequence")
		return
	}
	terminal, workspaceID, err := c.service.Terminal(r.Context(), id, executionID)
	if err != nil {
		if service.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "service not found")
		} else if errors.Is(err, process.ErrStaleTerminal) {
			writeError(w, http.StatusConflict, "stale_execution", "terminal execution is no longer available")
		} else {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		}
		return
	}
	terminalstream.Serve(w, r, terminal, after,
		func(data []byte) error { return c.service.TerminalInput(id, workspaceID, executionID, data) },
		func(cols, rows uint16) error {
			return c.service.TerminalResize(id, workspaceID, executionID, cols, rows)
		},
	)
}

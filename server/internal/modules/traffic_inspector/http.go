package traffic_inspector

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func workspaceID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		replyError(w, 400, "invalid_request", "invalid workspace id")
		return uuid.Nil, false
	}
	return id, true
}

func sessionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		replyError(w, 400, "invalid_request", "invalid session id")
		return uuid.Nil, false
	}
	return id, true
}

func replyJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func replyError(w http.ResponseWriter, status int, code, message string) {
	replyJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

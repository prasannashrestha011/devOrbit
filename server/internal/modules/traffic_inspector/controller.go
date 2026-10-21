package traffic_inspector

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Controller struct {
	Manager  *Manager
	Resolver Resolver
}

func (c *Controller) List(w http.ResponseWriter, r *http.Request) {
	id, ok := workspaceID(w, r)
	if !ok {
		return
	}
	if _, err := c.Resolver.Workspaces.Get(r.Context(), id); err != nil {
		replyError(w, 404, "not_found", "workspace not found")
		return
	}
	replyJSON(w, 200, c.Manager.List(id))
}

func (c *Controller) Start(w http.ResponseWriter, r *http.Request) {
	id, ok := workspaceID(w, r)
	if !ok {
		return
	}
	var input startRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		replyError(w, 400, "invalid_request", "invalid inspection request")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		replyError(w, 400, "invalid_request", "one JSON object is required")
		return
	}
	target, port, err := c.Resolver.Resolve(r.Context(), Target{
		WorkspaceID: id,
		Kind:        input.Kind,
		ServiceKey:  input.ServiceKey,
		PortKey:     input.PortKey,
		File:        input.File,
	})
	if err != nil {
		replyError(w, 400, "invalid_target", err.Error())
		return
	}
	/* setup the reverse proxy, returns proxy_url as response*/
	info, err := c.Manager.Start(target, loopbackURL(port))
	if err != nil {
		if errors.Is(err, ErrDuplicate) {
			replyError(w, 409, "already_active", err.Error())
		} else {
			replyError(w, 503, "inspector_unavailable", "could not start inspection gateway")
		}
		return
	}
	replyJSON(w, 201, info)
}

func (c *Controller) Get(w http.ResponseWriter, r *http.Request) {
	wid, ok := workspaceID(w, r)
	if !ok {
		return
	}
	sid, ok := sessionID(w, r)
	if !ok {
		return
	}
	info, err := c.Manager.Get(wid, sid)
	if err != nil {
		replyError(w, 404, "not_found", "inspection session not found")
		return
	}
	replyJSON(w, 200, info)
}

func (c *Controller) Stop(w http.ResponseWriter, r *http.Request) {
	wid, ok := workspaceID(w, r)
	if !ok {
		return
	}
	sid, ok := sessionID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Manager.Stop(ctx, wid, sid); err != nil && !errors.Is(err, ErrNotFound) {
		replyError(w, 503, "shutdown_failed", "inspection gateway shutdown timed out")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) Recent(w http.ResponseWriter, r *http.Request) {
	wid, ok := workspaceID(w, r)
	if !ok {
		return
	}
	sid, ok := sessionID(w, r)
	if !ok {
		return
	}
	after, err := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
	if r.URL.Query().Get("after") == "" {
		after, err = 0, nil
	}
	if err != nil {
		replyError(w, 400, "invalid_request", "invalid after sequence")
		return
	}
	limit := 500
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
	}
	if err != nil || limit < 1 || limit > 500 {
		replyError(w, 400, "invalid_request", "limit must be between 1 and 500")
		return
	}
	records, latest, err := c.Manager.Recent(wid, sid, after, limit)
	if err != nil {
		replyError(w, 404, "not_found", "inspection session not found")
		return
	}
	gap := len(records) > 0 && after > 0 && records[0].Sequence > after+1
	replyJSON(w, 200, recordsResponse{
		Records:        records,
		LatestSequence: latest,
		Gap:            gap,
	})
}

func (c *Controller) Clear(w http.ResponseWriter, r *http.Request) {
	wid, ok := workspaceID(w, r)
	if !ok {
		return
	}
	sid, ok := sessionID(w, r)
	if !ok {
		return
	}
	if err := c.Manager.Clear(wid, sid); err != nil {
		replyError(w, 404, "not_found", "inspection session not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

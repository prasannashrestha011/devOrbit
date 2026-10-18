package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/logs"
)

func parseQuery(r *http.Request) (uuid.UUID, logs.Filter, error) {
	for key, values := range r.URL.Query() {
		if key != "limit" && key != "before" || len(values) != 1 || values[0] == "" {
			return uuid.Nil, logs.Filter{}, errors.New("invalid log query")
		}
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, logs.Filter{}, errors.New("invalid id")
	}
	limit := 100
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 500 {
			return uuid.Nil, logs.Filter{}, errors.New("limit must be between 1 and 500")
		}
	}
	before := r.URL.Query().Get("before")
	if len(before) > 128 {
		return uuid.Nil, logs.Filter{}, errors.New("invalid cursor")
	}
	return id, logs.Filter{Limit: limit, Before: before}, nil
}

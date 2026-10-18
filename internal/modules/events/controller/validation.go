package controller

import (
	"errors"
	"net/url"

	"github.com/google/uuid"
)

func validateFilterQuery(values url.Values) error {
	for key, items := range values {
		if key != "workspace_id" && key != "service_id" || len(items) != 1 || items[0] == "" {
			return errors.New("invalid event filter")
		}
	}
	return nil
}

func parseFilterID(value, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, errors.New("invalid " + field)
	}
	return id, nil
}

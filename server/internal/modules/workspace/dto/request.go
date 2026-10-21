package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

type CreateInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PatchInput struct {
	Name        *string
	Description *string
}

func DecodeCreate(r *http.Request) (CreateInput, error) {
	var input CreateInput
	if err := decode(r, &input); err != nil {
		return input, err
	}
	return input, nil
}

func DecodePatch(r *http.Request) (PatchInput, error) {
	var input PatchInput
	var fields map[string]json.RawMessage
	if err := decode(r, &fields); err != nil || fields == nil {
		return input, errors.New("invalid JSON request")
	}
	for field, raw := range fields {
		if bytes.Equal(raw, []byte("null")) {
			return input, errors.New("null field value is not supported")
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return input, errors.New("fields must be strings")
		}
		switch field {
		case "name":
			input.Name = &value
		case "description":
			input.Description = &value
		default:
			return input, errors.New("unsupported workspace field")
		}
	}
	return input, nil
}

func decode(r *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("content type must be application/json")
	}
	decoder := json.NewDecoder(r.Body)
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

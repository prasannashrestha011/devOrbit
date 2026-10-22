package dto

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

type CreateInput struct {
	Name             string   `json:"name"`
	Command          string   `json:"command"`
	Args             []string `json:"args"`
	WorkingDirectory string   `json:"working_directory"`
	Revision         string   `json:"revision"`
}

type PatchInput struct {
	Name             *string   `json:"name"`
	Command          *string   `json:"command"`
	Args             *[]string `json:"args"`
	WorkingDirectory *string   `json:"working_directory"`
	Revision         string    `json:"revision"`
}

func DecodeCreate(r *http.Request) (CreateInput, error) {
	var input CreateInput
	err := decode(r, &input)
	return input, err
}

func DecodePatch(r *http.Request) (PatchInput, error) {
	var input PatchInput
	err := decode(r, &input)
	return input, err
}

func decode(r *http.Request, target any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return errors.New("content type must be application/json")
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("invalid JSON request")
	}
	return nil
}

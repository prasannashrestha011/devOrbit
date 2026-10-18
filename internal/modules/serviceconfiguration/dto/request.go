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
	Name             string
	Command          string
	Args             []string
	WorkingDirectory string
	Env              map[string]string
	Enabled          *bool
	AutoRestart      *bool
}

type PatchInput struct {
	Name             *string
	Command          *string
	Args             *[]string
	WorkingDirectory *string
	Env              *map[string]string
	Enabled          *bool
	AutoRestart      *bool
}

func DecodeCreate(r *http.Request) (CreateInput, error) {
	patch, err := DecodePatch(r)
	if err != nil {
		return CreateInput{}, err
	}
	input := CreateInput{Enabled: patch.Enabled, AutoRestart: patch.AutoRestart}
	if patch.Name != nil {
		input.Name = *patch.Name
	}
	if patch.Command != nil {
		input.Command = *patch.Command
	}
	if patch.Args != nil {
		input.Args = *patch.Args
	}
	if patch.WorkingDirectory != nil {
		input.WorkingDirectory = *patch.WorkingDirectory
	}
	if patch.Env != nil {
		input.Env = *patch.Env
	}
	return input, nil
}

func DecodePatch(r *http.Request) (PatchInput, error) {
	var input PatchInput
	fields, err := readFields(r)
	if err != nil {
		return input, err
	}
	for field, raw := range fields {
		switch field {
		case "name":
			input.Name = new(string)
			err = readValue(raw, input.Name)
		case "command":
			input.Command = new(string)
			err = readValue(raw, input.Command)
		case "args":
			input.Args = new([]string)
			err = readValue(raw, input.Args)
		case "working_directory":
			input.WorkingDirectory = new(string)
			err = readValue(raw, input.WorkingDirectory)
		case "env":
			input.Env = new(map[string]string)
			err = readValue(raw, input.Env)
		case "enabled":
			input.Enabled = new(bool)
			err = readValue(raw, input.Enabled)
		case "auto_restart":
			input.AutoRestart = new(bool)
			err = readValue(raw, input.AutoRestart)
		default:
			return input, errors.New("unsupported service field")
		}
		if err != nil {
			return input, err
		}
	}
	return input, nil
}

func readFields(r *http.Request) (map[string]json.RawMessage, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return nil, errors.New("content type must be application/json")
	}
	decoder := json.NewDecoder(r.Body)
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil || fields == nil {
		return nil, errors.New("invalid JSON request")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("invalid JSON request")
	}
	return fields, nil
}

func readValue(raw json.RawMessage, target any) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, target) != nil {
		return errors.New("invalid service field value")
	}
	return nil
}

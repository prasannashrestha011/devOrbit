package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
)

type Response struct {
	ID               uuid.UUID         `json:"id"`
	WorkspaceID      uuid.UUID         `json:"workspace_id"`
	Name             string            `json:"name"`
	Command          string            `json:"command"`
	Args             []string          `json:"args"`
	WorkingDirectory string            `json:"working_directory"`
	GitBranch        string            `json:"git_branch"`
	Env              map[string]string `json:"env"`
	Ports            map[string]int    `json:"ports"`
	Enabled          *bool             `json:"enabled"`
	AutoRestart      bool              `json:"auto_restart"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

func FromModel(item *model.Service) Response {
	return Response{
		ID: item.ID, WorkspaceID: item.WorkspaceID, Name: item.Name,
		Command: item.Command, Args: item.Args, WorkingDirectory: item.WorkingDirectory, GitBranch: item.GitBranch,
		Env: item.Env, Ports: item.Ports, Enabled: item.Enabled, AutoRestart: item.AutoRestart,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func FromModels(items []*model.Service) []Response {
	responses := make([]Response, 0, len(items))
	for _, item := range items {
		responses = append(responses, FromModel(item))
	}
	return responses
}

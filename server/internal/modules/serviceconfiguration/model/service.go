package model

import (
	"time"

	"github.com/google/uuid"
	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
)

// Service is a runtime projection of workspace YAML, not a database table.
type Service struct {
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

// Identity persists only the YAML key to UUID association needed by logs and running processes.
type Identity struct {
	ID          uuid.UUID           `gorm:"type:uuid;primaryKey"`
	WorkspaceID uuid.UUID           `gorm:"type:uuid;not null;uniqueIndex:idx_identity_workspace_key"`
	Workspace   workspace.Workspace `gorm:"foreignKey:WorkspaceID;constraint:OnDelete:CASCADE"`
	Key         string              `gorm:"type:varchar(120);not null;uniqueIndex:idx_identity_workspace_key"`
}

func (Identity) TableName() string { return "service_identities" }

package model

import (
	"time"

	"github.com/google/uuid"
	workspace "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
)

// Workspace keeps GORM Gen's generated relationship methods in this model package.
type Workspace = workspace.Workspace

type Service struct {
	ID               uuid.UUID           `json:"id" gorm:"type:uuid;primaryKey"`
	WorkspaceID      uuid.UUID           `json:"workspace_id" gorm:"type:uuid;not null;index;uniqueIndex:idx_service_workspace_name"`
	Workspace        workspace.Workspace `json:"-" gorm:"foreignKey:WorkspaceID;constraint:OnDelete:RESTRICT"`
	Name             string              `json:"name" gorm:"type:varchar(120);not null;check:name <> '';uniqueIndex:idx_service_workspace_name"`
	Command          string              `json:"command" gorm:"type:text;not null;check:command <> ''"`
	Args             []string            `json:"args" gorm:"type:jsonb;serializer:json;not null"`
	WorkingDirectory string              `json:"working_directory" gorm:"type:text;not null;default:''"`
	Env              map[string]string   `json:"env" gorm:"type:jsonb;serializer:json;not null"`
	Enabled          *bool               `json:"enabled" gorm:"not null;default:true"`
	AutoRestart      bool                `json:"auto_restart" gorm:"not null;default:false"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
}

func (Service) TableName() string { return "services" }

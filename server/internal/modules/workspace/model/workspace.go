package model

import (
	"time"

	"github.com/google/uuid"
)

type Workspace struct {
	ID          uuid.UUID `json:"id" gorm:"type:text;primaryKey;not null"`
	Name        string    `json:"name" gorm:"type:varchar(120);not null;check:name <> ''"`
	Description string    `json:"description" gorm:"type:varchar(1000);not null;default:''"`
	RootPath    string    `json:"root_path" gorm:"type:text;not null;check:root_path <> ''"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Workspace) TableName() string { return "workspaces" }

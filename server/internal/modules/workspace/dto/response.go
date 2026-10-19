package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
)

type Response struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	RootPath    string    `json:"root_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func FromModel(item *model.Workspace) Response {
	return Response{
		ID: item.ID, Name: item.Name, Description: item.Description,
		RootPath: item.RootPath, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func FromModels(items []*model.Workspace) []Response {
	responses := make([]Response, 0, len(items))
	for _, item := range items {
		responses = append(responses, FromModel(item))
	}
	return responses
}

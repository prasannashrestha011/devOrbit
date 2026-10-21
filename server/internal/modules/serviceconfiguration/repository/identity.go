package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) Identity(ctx context.Context, id uuid.UUID) (*model.Identity, error) {
	var item model.Identity
	err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &item, err
}

func (r *Repository) Resolve(ctx context.Context, workspaceID uuid.UUID, key string) (*model.Identity, error) {
	created := model.Identity{ID: uuid.New(), WorkspaceID: workspaceID, Key: key}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&created).Error; err != nil {
		return nil, err
	}
	var item model.Identity
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND key = ?", workspaceID, key).First(&item).Error
	return &item, err
}

package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/dal/query"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("workspace not found")

type Repository struct {
	q *query.Query
}

func New(db *gorm.DB) *Repository {
	return &Repository{q: query.Use(db)}
}

func (r *Repository) Create(ctx context.Context, workspace *model.Workspace) error {
	return r.q.Workspace.WithContext(ctx).Create(workspace)
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*model.Workspace, error) {
	workspace, err := r.q.Workspace.WithContext(ctx).Where(r.q.Workspace.ID.Eq(id)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return workspace, err
}

func (r *Repository) List(ctx context.Context, limit, offset int) ([]*model.Workspace, error) {
	return r.q.Workspace.WithContext(ctx).Order(r.q.Workspace.CreatedAt.Asc(), r.q.Workspace.ID.Asc()).Limit(limit).Offset(offset).Find()
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, changes map[string]any) (*model.Workspace, error) {
	changes["updated_at"] = time.Now().UTC()
	result, err := r.q.Workspace.WithContext(ctx).Where(r.q.Workspace.ID.Eq(id)).Updates(changes)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.q.Workspace.WithContext(ctx).Where(r.q.Workspace.ID.Eq(id)).Delete()
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

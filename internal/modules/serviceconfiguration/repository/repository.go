package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prasannashrestha011/devOrbit/internal/dal/query"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	"gorm.io/gorm"
)

var (
	ErrNotFound          = errors.New("service not found")
	ErrDuplicate         = errors.New("service name already exists in workspace")
	ErrWorkspaceNotFound = errors.New("workspace not found")
)

type Repository struct{ q *query.Query }

func New(db *gorm.DB) *Repository { return &Repository{q: query.Use(db)} }

func (r *Repository) Create(ctx context.Context, item *model.Service) error {
	return constraintError(r.q.Service.WithContext(ctx).Create(item))
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*model.Service, error) {
	item, err := r.q.Service.WithContext(ctx).Where(r.q.Service.ID.Eq(id)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return item, err
}

func (r *Repository) List(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*model.Service, error) {
	return r.q.Service.WithContext(ctx).Where(r.q.Service.WorkspaceID.Eq(workspaceID)).Order(r.q.Service.CreatedAt.Asc(), r.q.Service.ID.Asc()).Limit(limit).Offset(offset).Find()
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, changes map[string]any) (*model.Service, error) {
	changes["updated_at"] = time.Now().UTC()
	result, err := r.q.Service.WithContext(ctx).Where(r.q.Service.ID.Eq(id)).Updates(changes)
	if err != nil {
		return nil, constraintError(err)
	}
	if result.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.q.Service.WithContext(ctx).Where(r.q.Service.ID.Eq(id)).Delete()
	if err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) HasServices(ctx context.Context, workspaceID uuid.UUID) (bool, error) {
	count, err := r.q.Service.WithContext(ctx).Where(r.q.Service.WorkspaceID.Eq(workspaceID)).Limit(1).Count()
	return count > 0, err
}

func (r *Repository) NameExists(ctx context.Context, workspaceID uuid.UUID, name string, except uuid.UUID) (bool, error) {
	q := r.q.Service.WithContext(ctx).Where(r.q.Service.WorkspaceID.Eq(workspaceID), r.q.Service.Name.Eq(name))
	if except != uuid.Nil {
		q = q.Where(r.q.Service.ID.Neq(except))
	}
	count, err := q.Limit(1).Count()
	return count > 0, err
}

func constraintError(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return ErrDuplicate
		case "23503":
			return ErrWorkspaceNotFound
		}
	}
	return err
}

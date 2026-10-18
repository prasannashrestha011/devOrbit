package database

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type dbContextKey struct{}

// WithDB attaches the connection or current transaction for this operation.
func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, dbContextKey{}, db)
}

func FromContext(ctx context.Context) (*gorm.DB, error) {
	db, ok := ctx.Value(dbContextKey{}).(*gorm.DB)
	if !ok || db == nil {
		return nil, errors.New("database not found in context")
	}
	return db, nil
}

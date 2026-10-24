package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/database"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
)

func TestWorkspaceSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devorbit.db")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	for range 2 {
		if err := database.Migrate(db, &model.Workspace{}); err != nil {
			t.Fatal(err)
		}
	}
	if !db.Migrator().HasTable(&model.Workspace{}) {
		t.Fatal("workspace table not created")
	}
	other, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	otherSQL, _ := other.DB()
	defer otherSQL.Close()
	repo := New(db)
	first := &model.Workspace{ID: uuid.New(), Name: "first", RootPath: t.TempDir()}
	second := &model.Workspace{ID: uuid.New(), Name: "second", RootPath: first.RootPath}
	for _, item := range []*model.Workspace{first, second} {
		if err := repo.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	otherRepo := New(other)
	loaded, err := otherRepo.Get(ctx, first.ID)
	if err != nil || loaded.Name != first.Name {
		t.Fatalf("separate connection read: %+v %v", loaded, err)
	}
	items, err := otherRepo.List(ctx, 1, 0)
	if err != nil || len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("ordered page: %+v %v", items, err)
	}
	updated, err := otherRepo.Update(ctx, first.ID, map[string]any{"description": "new"})
	if err != nil || updated.ID != first.ID || updated.Description != "new" || updated.Name != "first" || !updated.CreatedAt.Equal(loaded.CreatedAt) || !updated.UpdatedAt.After(loaded.UpdatedAt) {
		t.Fatalf("update: %+v %v", updated, err)
	}
	if err := otherRepo.Delete(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted row lookup: %v", err)
	}
	if err := repo.Delete(ctx, second.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing row delete: %v", err)
	}
	transaction := db.Begin()
	if transaction.Error != nil {
		t.Fatal(transaction.Error)
	}
	transactionRepo := New(transaction)
	inside := &model.Workspace{ID: uuid.New(), Name: "inside", RootPath: first.RootPath}
	if err := transactionRepo.Create(ctx, inside); err != nil {
		t.Fatal(err)
	}
	if _, err := transactionRepo.Get(ctx, inside.ID); err != nil {
		t.Fatal(err)
	}
	if err := transaction.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	if _, err := otherRepo.Get(ctx, inside.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("rolled back row visible: %v", err)
	}
}

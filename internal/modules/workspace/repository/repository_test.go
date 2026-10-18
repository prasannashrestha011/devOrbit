package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/database"
	"github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
)

func TestWorkspacePostgres(t *testing.T) {
	dsn := os.Getenv("DEVORBIT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set DEVORBIT_TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	if dsn == os.Getenv("DEVORBIT_DATABASE_URL") {
		t.Fatal("test database URL must differ from the application database URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Skip("test PostgreSQL unavailable")
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	var version int
	if err := db.Raw("SELECT current_setting('server_version_num')::int").Scan(&version).Error; err != nil || version/10000 != 17 {
		t.Fatalf("PostgreSQL 17 required, got version %d (%v)", version, err)
	}
	schema := "devorbit_phase02_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Exec("DROP SCHEMA " + schema + " CASCADE").Error }()
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := database.Migrate(db, &model.Workspace{}); err != nil {
			t.Fatal(err)
		}
	}
	if !db.Migrator().HasTable(&model.Workspace{}) {
		t.Fatal("workspace table not created")
	}
	other, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	otherSQL, _ := other.DB()
	otherSQL.SetMaxOpenConns(1)
	defer otherSQL.Close()
	if err := other.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
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
	if err != nil || updated.ID != first.ID || updated.Description != "new" || updated.Name != "first" || !updated.CreatedAt.Equal(first.CreatedAt) || !updated.UpdatedAt.After(first.UpdatedAt) {
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

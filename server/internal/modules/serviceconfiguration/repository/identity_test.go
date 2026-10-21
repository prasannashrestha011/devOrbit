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
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
)

func TestResolveExistingIdentityAndDeleteWorkspace(t *testing.T) {
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
	schema := "devorbit_identity_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Exec("DROP SCHEMA " + schema + " CASCADE").Error }()
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db, &workspaceModel.Workspace{}, &model.Identity{}); err != nil {
		t.Fatal(err)
	}
	wid := uuid.New()
	if err := workspaceRepository.New(db).Create(ctx, &workspaceModel.Workspace{ID: wid, Name: "project", RootPath: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	first, err := repo.Resolve(ctx, wid, "api")
	if err != nil {
		t.Fatal(err)
	}
	again, err := repo.Resolve(ctx, wid, "api")
	if err != nil || again.ID != first.ID {
		t.Fatalf("existing identity changed: first=%s again=%v error=%v", first.ID, again, err)
	}
	if err := workspaceRepository.New(db).Delete(ctx, wid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Identity(ctx, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("identity remained after workspace deletion: %v", err)
	}
}

package repository

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/database"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
)

func TestServicePostgres(t *testing.T) {
	dsn := os.Getenv("DEVORBIT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set DEVORBIT_TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	if dsn == os.Getenv("DEVORBIT_DATABASE_URL") {
		t.Fatal("test database URL must differ from application database URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	var version int
	if err := db.Raw("SELECT current_setting('server_version_num')::int").Scan(&version).Error; err != nil || version/10000 != 17 {
		t.Fatalf("PostgreSQL 17 required, got %d: %v", version, err)
	}
	schema := "devorbit_phase03_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Exec("DROP SCHEMA " + schema + " CASCADE").Error }()
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := database.Migrate(db, &workspaceModel.Workspace{}, &model.Service{}); err != nil {
			t.Fatal(err)
		}
	}
	if !db.Migrator().HasTable(&model.Service{}) || !db.Migrator().HasConstraint(&model.Service{}, "Workspace") {
		t.Fatal("service table or workspace foreign key missing")
	}
	workspaces := workspaceRepository.New(db)
	wid, otherID := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{wid, otherID} {
		if err := workspaces.Create(ctx, &workspaceModel.Workspace{ID: id, Name: "workspace", RootPath: t.TempDir()}); err != nil {
			t.Fatal(err)
		}
	}
	repo := New(db)
	falseValue := false
	newItem := func(id, workspaceID uuid.UUID, name string) *model.Service {
		return &model.Service{ID: id, WorkspaceID: workspaceID, Name: name, Command: "go", Args: []string{"", "run", "./cmd/api"}, Env: map[string]string{"APP_ENV": "dev"}, Enabled: &falseValue}
	}
	item := newItem(uuid.New(), wid, "api")
	if err := repo.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, item.ID)
	if err != nil || *got.Enabled || !reflect.DeepEqual(got.Args, item.Args) || !reflect.DeepEqual(got.Env, item.Env) || got.AutoRestart {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if err := repo.Create(ctx, newItem(uuid.New(), uuid.New(), "missing")); !errors.Is(err, ErrWorkspaceNotFound) {
		t.Fatalf("foreign key: %v", err)
	}
	if err := repo.Create(ctx, newItem(uuid.New(), wid, "api")); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}
	other := newItem(uuid.New(), otherID, "api")
	if err := repo.Create(ctx, other); err != nil {
		t.Fatalf("same name in another workspace: %v", err)
	}
	items, err := repo.List(ctx, wid, 50, 0)
	if err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("workspace list: %+v %v", items, err)
	}
	if has, err := repo.HasServices(ctx, wid); err != nil || !has {
		t.Fatalf("has services: %t %v", has, err)
	}
	if exists, err := repo.NameExists(ctx, wid, "api", item.ID); err != nil || exists {
		t.Fatalf("exclude current: %t %v", exists, err)
	}
	updated, err := repo.Update(ctx, item.ID, map[string]any{"args": "[]", "env": "{}", "enabled": false, "auto_restart": false})
	if err != nil || len(updated.Args) != 0 || len(updated.Env) != 0 || *updated.Enabled || !updated.CreatedAt.Equal(got.CreatedAt) || !updated.UpdatedAt.After(got.UpdatedAt) {
		t.Fatalf("zero-value update: %+v %v", updated, err)
	}
	if err := workspaces.Delete(ctx, wid); !errors.Is(err, workspaceRepository.ErrConflict) {
		t.Fatalf("workspace deletion with services: %v", err)
	}
	if err := repo.Delete(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing service: %v", err)
	}
	if err := repo.Delete(ctx, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delete: %v", err)
	}
	if err := workspaces.Delete(ctx, wid); err != nil {
		t.Fatal(err)
	}
	const racers = 2
	var wg sync.WaitGroup
	successes := make(chan error, racers)
	for range racers {
		wg.Add(1)
		go func() { defer wg.Done(); successes <- repo.Create(ctx, newItem(uuid.New(), otherID, "race")) }()
	}
	wg.Wait()
	close(successes)
	created, duplicates := 0, 0
	for err := range successes {
		if err == nil {
			created++
		} else if errors.Is(err, ErrDuplicate) {
			duplicates++
		} else {
			t.Fatalf("concurrent create: %v", err)
		}
	}
	if created != 1 || duplicates != 1 {
		t.Fatalf("concurrent create: created=%d duplicates=%d", created, duplicates)
	}
}

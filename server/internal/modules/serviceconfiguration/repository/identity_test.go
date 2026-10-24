package repository

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prasannashrestha011/devOrbit/internal/database"
	"github.com/prasannashrestha011/devOrbit/internal/modules/serviceconfiguration/model"
	workspaceModel "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/model"
	workspaceRepository "github.com/prasannashrestha011/devOrbit/internal/modules/workspace/repository"
	"gorm.io/gorm"
)

func TestResolveExistingIdentityAndDeleteWorkspace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devorbit.db")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
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
	if err := database.Migrate(db, &workspaceModel.Workspace{}, &model.Identity{}); err != nil {
		t.Fatal(err)
	}
	again, err := repo.Resolve(ctx, wid, "api")
	if err != nil || again.ID != first.ID {
		t.Fatalf("existing identity changed: first=%s again=%v error=%v", first.ID, again, err)
	}
	if _, err := repo.Resolve(ctx, uuid.New(), "orphan"); !errors.Is(err, gorm.ErrForeignKeyViolated) {
		t.Fatalf("expected foreign key violation, got %v", err)
	}
	duplicate := model.Identity{ID: uuid.New(), WorkspaceID: wid, Key: "api"}
	if err := db.Create(&duplicate).Error; !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("expected duplicate key violation, got %v", err)
	}
	other, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	otherSQL, _ := other.DB()
	defer otherSQL.Close()
	repos := []*Repository{repo, New(other)}
	var wg sync.WaitGroup
	type result struct {
		item *model.Identity
		err  error
	}
	results := make(chan result, 16)
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, err := repos[i%2].Resolve(ctx, wid, "worker")
			results <- result{item, err}
		}()
	}
	wg.Wait()
	close(results)
	var concurrentID uuid.UUID
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if concurrentID == uuid.Nil {
			concurrentID = result.item.ID
		}
		if result.item.ID != concurrentID {
			t.Fatal("concurrent creates returned different identities")
		}
	}
	if err := workspaceRepository.New(db).Delete(ctx, wid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Identity(ctx, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("identity remained after workspace deletion: %v", err)
	}
}

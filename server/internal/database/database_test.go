package database

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type migrationRecord struct {
	ID uint `gorm:"primaryKey"`
}

func TestOpenFailureDoesNotExposePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-project", "missing", "devorbit.db")
	_, err := Open(context.Background(), path)
	if err == nil || strings.Contains(err.Error(), "private-project") {
		t.Fatalf("expected safe connection error, got %v", err)
	}
	if _, err := Open(context.Background(), "relative.db"); err == nil {
		t.Fatal("relative database path accepted")
	}
}

func TestMigrateRepeatableAndPersistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devorbit # 50%.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Migrate(db, &migrationRecord{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&migrationRecord{ID: 7}).Error; err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ = db.DB()
	defer sqlDB.Close()
	var row migrationRecord
	if err := db.First(&row).Error; err != nil || row.ID != 7 {
		t.Fatalf("record did not survive reopening: %+v, %v", row, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("database permissions = %o", info.Mode().Perm())
	}
}

func TestConnectionSettingsSurviveReconnect(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "devorbit.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if sqlDB.Stats().MaxOpenConnections != 1 {
		t.Fatal("SQLite connections must be serialized")
	}
	// Force a fresh connection for each query to check the driver's connection options.
	sqlDB.SetMaxIdleConns(0)
	for range 2 {
		var foreignKeys, busyTimeout int
		if err := db.Raw("PRAGMA foreign_keys").Scan(&foreignKeys).Error; err != nil || foreignKeys != 1 {
			t.Fatalf("foreign_keys = %d, error = %v", foreignKeys, err)
		}
		if err := db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil || busyTimeout != 5000 {
			t.Fatalf("busy_timeout = %d, error = %v", busyTimeout, err)
		}
	}
}

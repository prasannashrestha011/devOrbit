package database

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type migrationRecord struct {
	ID uint `gorm:"primaryKey"`
}

func TestOpenFailureDoesNotExposeDSN(t *testing.T) {
	secret := "very-secret-password"
	_, err := Open(context.Background(), "postgres://user:"+secret+"@invalid host/db")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("expected safe connection error, got %v", err)
	}
}

func TestMigrateRepeatable(t *testing.T) {
	dsn := os.Getenv("DEVORBIT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set DEVORBIT_TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	if dsn == os.Getenv("DEVORBIT_DATABASE_URL") {
		t.Fatal("test database URL must differ from the application database URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	schema := "devorbit_phase01_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		_ = sqlDB.Close()
		t.Fatal(err)
	}
	defer func() {
		_ = db.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = sqlDB.Close()
	}()
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Migrate(db, &migrationRecord{}); err != nil {
			t.Fatal(err)
		}
	}
	if !db.Migrator().HasTable(&migrationRecord{}) {
		t.Fatal("registered model table was not created")
	}
}

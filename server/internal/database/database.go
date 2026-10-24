package database

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, path string) (*gorm.DB, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("database path must be absolute")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("database file initialization failed")
	}
	if err := file.Close(); err != nil {
		return nil, errors.New("database file close failed")
	}
	// URI escaping keeps path characters out of connection options, including on Windows.
	dsn := url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(filepath.ToSlash(path), "/"), RawQuery: "_foreign_keys=on&_busy_timeout=5000"}
	db, err := gorm.Open(sqlite.Open(dsn.String()), &gorm.Config{DisableAutomaticPing: true, TranslateError: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("database connection failed")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.New("database pool initialization failed")
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, errors.New("database ping failed")
	}
	return db, nil
}

// Migrate only the models explicitly passed by the application.
func Migrate(db *gorm.DB, models ...any) error {
	if err := db.AutoMigrate(models...); err != nil {
		return errors.New("database schema initialization failed")
	}
	return nil
}

package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	projectdb "photo-backup-manager/database"
)

const defaultDatabasePath = "database/photo_backup.db"

// NewDbConnection opens the project SQLite database. Set PHOTO_BACKUP_DB to
// override the path when running the app or generator from another location.
func NewDbConnection() (*gorm.DB, error) {
	dbPath := os.Getenv("PHOTO_BACKUP_DB")
	if dbPath == "" {
		dbPath = defaultDatabasePath
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory for %q: %w", dbPath, err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open SQLite database %q: %w", dbPath, err)
	}

	// Keep one SQLite connection so the foreign_keys setting applies consistently.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get SQLite connection %q: %w", dbPath, err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return nil, fmt.Errorf("enable SQLite foreign keys: %w", err)
	}

	if err := projectdb.Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate SQLite database %q: %w", dbPath, err)
	}
	return db, nil
}

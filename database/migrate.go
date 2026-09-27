package database

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

//go:embed schema.sql
var schemaFiles embed.FS

// Migrate creates the application's tables and indexes if they do not exist.
// It is safe to call each time the application opens the database.
func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate database: db is nil")
	}

	schema, err := schemaFiles.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read database schema: %w", err)
	}

	// Remove SQL comment lines before splitting the embedded statements.
	var sqlLines []string
	for _, line := range strings.Split(string(schema), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			sqlLines = append(sqlLines, line)
		}
	}

	return db.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		for _, statement := range strings.Split(strings.Join(sqlLines, "\n"), ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if err := tx.Exec(statement).Error; err != nil {
				return fmt.Errorf("execute schema statement: %w", err)
			}
		}
		return nil
	})
}

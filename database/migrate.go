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


func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate database: db is nil")
	}

	schema, err := schemaFiles.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read database schema: %w", err)
	}

	
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

//go:build production

package main

import (
	"os"
	"path/filepath"
)

// Packaged builds keep their DB in a writable user directory, not beside the exe.
func configureAppStorage() error {
	if os.Getenv("PHOTO_BACKUP_DB") != "" {
		return nil
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	return os.Setenv("PHOTO_BACKUP_DB", filepath.Join(root, "photo-backup-manager", "photo_backup.db"))
}

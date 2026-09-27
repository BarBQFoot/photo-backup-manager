package main

import (
	"context"
	"fmt"

	"photo-backup-manager/repository"
	"photo-backup-manager/service"

	"gorm.io/gorm"
)

// App struct
type App struct {
	ctx          context.Context
	db           *gorm.DB
	driveService *service.DriveService
}

// NewApp creates a new App application struct
func NewApp(db *gorm.DB) *App {
	return &App{
		db: db,
		driveService: service.NewDriveServiceWithRepositories(
			repository.NewFileRecordRepository(db),
			repository.NewBackupJobRepository(db),
		),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.driveService.SetWailsContext(ctx)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

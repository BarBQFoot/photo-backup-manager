package main

import (
	"context"
	"fmt"

	"photo-backup-manager/repository"
	"photo-backup-manager/service"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

// SelectFolder opens the native folder picker for the frontend.
func (a *App) SelectFolder(title string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application is not ready")
	}
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: title, CanCreateDirectories: true})
}

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
	aiSetup      *service.AISetup
}

// NewApp creates a new App application struct
func NewApp(db *gorm.DB) *App {
	return &App{
		db:      db,
		aiSetup: service.NewAISetup(),
		driveService: service.NewDriveServiceWithRepositories(
			repository.NewFileRecordRepository(db),
			repository.NewBackupJobRepository(db),
		),
	}
}

func (a *App) shutdown(context.Context) {
	if a.aiSetup != nil {
		a.aiSetup.Cancel()
	}
	if a.driveService != nil {
		_ = a.driveService.Close()
	}
}

func (a *App) GetAISetupStatus() service.AISetupStatus { return a.aiSetup.GetStatus() }

func (a *App) StartAISetup() (service.AISetupStatus, error) {
	if a.ctx == nil {
		return a.aiSetup.GetStatus(), fmt.Errorf("application is not ready")
	}
	return a.aiSetup.Start(a.ctx), nil
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

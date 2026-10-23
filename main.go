package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"photo-backup-manager/repository"
	"photo-backup-manager/service"

	"github.com/joho/godotenv"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	if err := godotenv.Load(); err != nil {
		log.Printf("ไม่พบไฟล์ .env: %v", err)
	}
	if executable, err := os.Executable(); err == nil {
		// An exported app can be launched with any working directory.
		for _, path := range appEnvPaths(executable, runtime.GOOS) {
			_ = godotenv.Load(path)
		}
	}
	if config, err := os.UserConfigDir(); err == nil {
		_ = godotenv.Load(filepath.Join(config, "photo-backup-manager", ".env"))
	}
	if err := configureAppStorage(); err != nil {
		log.Fatalf("prepare app storage: %v", err)
	}

	db, err := repository.NewDbConnection()
	if err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("get database connection: %v", err)
	}
	defer sqlDB.Close()

	app := NewApp(db)

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "photo-backup-manager",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: service.PhotoPreviewMiddleware(app.driveService),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
			app.driveService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

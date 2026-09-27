package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"photo-backup-manager/database"
	"photo-backup-manager/internal/backup"
	dbmodel "photo-backup-manager/model/model"
	"photo-backup-manager/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDriveServiceScanDrive(t *testing.T) {
	root := t.TempDir()
	photoPath := filepath.Join(root, "photo.jpg")
	if err := os.WriteFile(photoPath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := NewDriveService().ScanDrive(root)
	if err != nil {
		t.Fatalf("ScanDrive() error = %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("ScanDrive() returned %d files, want 1", len(files))
	}
	if files[0].Path != photoPath {
		t.Fatalf("Path = %q, want %q", files[0].Path, photoPath)
	}
}

func TestDriveServiceScanDrivePreservesScannerError(t *testing.T) {
	_, err := NewDriveService().ScanDrive("")
	var scanErr *backup.ScanError
	if !errors.As(err, &scanErr) {
		t.Fatalf("error = %v, want backup.ScanError", err)
	}
	if scanErr.Code != "INVALID_PATH" {
		t.Fatalf("error code = %q, want INVALID_PATH", scanErr.Code)
	}
}

func TestDriveServiceStartBackupMovesSelectedFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(source, "photo.jpg")
	if err := os.WriteFile(photo, []byte("photo fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewDriveService().StartBackup(BackupRequest{
		SourcePath:      source,
		DestinationPath: destination,
		FilePaths:       []string{photo},
	})
	if err != nil {
		t.Fatalf("StartBackup() error = %v", err)
	}
	if result.SuccessCount != 1 || result.FailedCount != 0 || result.SkippedCount != 0 {
		t.Fatalf("unexpected result counts: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(destination, "photo.jpg")); err != nil {
		t.Fatalf("destination file was not created: %v", err)
	}
}

func TestDriveServiceStartBackupSkipsDuplicate(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(source, "photo.jpg")
	if err := os.WriteFile(photo, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	duplicate := filepath.Join(destination, "photo.jpg")
	if err := os.WriteFile(duplicate, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := NewDriveService().StartBackup(BackupRequest{
		SourcePath:      source,
		DestinationPath: destination,
		FilePaths:       []string{photo},
	})
	if err != nil {
		t.Fatalf("StartBackup() error = %v", err)
	}
	if result.SkippedCount != 1 || result.SuccessCount != 0 || result.FailedCount != 0 {
		t.Fatalf("unexpected result counts: %#v", result)
	}
	if content, err := os.ReadFile(photo); err != nil || string(content) != "source" {
		t.Fatalf("source content = %q, error = %v", content, err)
	}
	if content, err := os.ReadFile(duplicate); err != nil || string(content) != "existing" {
		t.Fatalf("destination content = %q, error = %v", content, err)
	}
}

func TestDriveServiceStartBackupRejectsSelectionOutsideSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	outside := filepath.Join(root, "outside.jpg")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := NewDriveService().StartBackup(BackupRequest{
		SourcePath:      source,
		DestinationPath: destination,
		FilePaths:       []string{outside},
	})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != "INVALID_SELECTION" {
		t.Fatalf("error = %v, want INVALID_SELECTION", err)
	}
}

func TestDriveServiceReportsCompletedProgress(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(source, "photo.jpg")
	if err := os.WriteFile(photo, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	service := NewDriveService()
	if _, err := service.StartBackup(BackupRequest{SourcePath: source, DestinationPath: destination, FilePaths: []string{photo}}); err != nil {
		t.Fatalf("StartBackup() error = %v", err)
	}
	progress := service.GetBackupProgress()
	if progress.Status != "completed" || progress.Completed != 1 || progress.Total != 1 {
		t.Fatalf("unexpected progress: %#v", progress)
	}
}

func TestDriveServiceSQLiteBackupPersistsJobAndFileRecord(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(source, "photo.jpg")
	if err := os.WriteFile(photo, []byte("sqlite fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	fileStore, jobStore, closeDB := newSQLiteStores(t)
	defer closeDB()
	service := NewDriveServiceWithRepositories(fileStore, jobStore)
	result, err := service.StartBackup(BackupRequest{SourcePath: source, DestinationPath: destination, FilePaths: []string{photo}})
	if err != nil {
		t.Fatalf("StartBackup() error = %v", err)
	}
	if result.JobID <= 0 || result.SuccessCount != 1 {
		t.Fatalf("unexpected backup result: %#v", result)
	}

	record, err := fileStore.GetByPath(testContext(), filepath.Join(destination, "photo.jpg"))
	if err != nil {
		t.Fatalf("GetByPath() error = %v", err)
	}
	if record.Status != repository.FileRecordStatusActive || record.MimeType != "image/jpeg" {
		t.Fatalf("unexpected file record: %#v", record)
	}
	history, err := jobStore.ListByDestination(testContext(), destination)
	if err != nil || len(history) != 1 || history[0].ID != int32(result.JobID) {
		t.Fatalf("unexpected history: %v %#v", err, history)
	}
}

func TestDriveServiceSkipsActiveRecordDuplicate(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(source, "photo.jpg")
	destinationFile := filepath.Join(destination, "photo.jpg")
	if err := os.WriteFile(photo, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destinationFile, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	fileStore, jobStore, closeDB := newSQLiteStores(t)
	defer closeDB()
	if err := fileStore.SaveByPath(testContext(), &dbmodel.FileRecord{
		FileName: filepath.Base(destinationFile), Path: destinationFile, SizeBytes: 8,
		MimeType: "image/jpeg", ModifiedAt: time.Now(), AiStatus: repository.AIStatusUnanalyzed,
		Status: repository.FileRecordStatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := NewDriveServiceWithRepositories(fileStore, jobStore).StartBackup(BackupRequest{
		SourcePath: source, DestinationPath: destination, FilePaths: []string{photo},
	})
	if err != nil {
		t.Fatalf("StartBackup() error = %v", err)
	}
	if result.SkippedCount != 1 || result.SuccessCount != 0 {
		t.Fatalf("unexpected duplicate result: %#v", result)
	}
	if content, err := os.ReadFile(photo); err != nil || string(content) != "source" {
		t.Fatalf("source content = %q, error = %v", content, err)
	}
}

func TestDriveServiceReturnsMissingRecordFromSQLite(t *testing.T) {
	destination := t.TempDir()
	missingPath := filepath.Join(destination, "missing.jpg")
	fileStore, jobStore, closeDB := newSQLiteStores(t)
	defer closeDB()
	if err := fileStore.SaveByPath(testContext(), &dbmodel.FileRecord{
		FileName: filepath.Base(missingPath), Path: missingPath, SizeBytes: 12,
		MimeType: "image/jpeg", ModifiedAt: time.Now(), Description: "missing fixture",
		AiStatus: repository.AIStatusAnalyzed, Status: repository.FileRecordStatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	files, err := NewDriveServiceWithRepositories(fileStore, jobStore).ScanDrive(destination)
	if err != nil {
		t.Fatalf("ScanDrive() error = %v", err)
	}
	if len(files) != 1 || files[0].FileStatus != "missing" || files[0].Description != "missing fixture" {
		t.Fatalf("unexpected missing result: %#v", files)
	}
}

func newSQLiteStores(t *testing.T) (repository.FileRecordStore, repository.BackupJobStore, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	fileStore := repository.NewFileRecordRepository(db)
	jobStore := repository.NewBackupJobRepository(db)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	return fileStore, jobStore, func() { _ = sqlDB.Close() }
}

func testContext() context.Context {
	return context.Background()
}

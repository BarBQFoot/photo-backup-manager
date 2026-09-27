package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"photo-backup-manager/internal/backup"
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

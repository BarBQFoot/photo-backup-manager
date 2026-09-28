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

func TestDriveServiceStartBackupReportsFileRecordSaveFailure(t *testing.T) {
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

	_, jobStore, closeDB := newSQLiteStores(t)
	defer closeDB()
	result, err := NewDriveServiceWithRepositories(failingFileRecordStore{}, jobStore).StartBackup(BackupRequest{
		SourcePath: source, DestinationPath: destination, FilePaths: []string{photo},
	})
	if err != nil {
		t.Fatalf("StartBackup() error = %v", err)
	}
	if result.SuccessCount != 0 || result.FailedCount != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Items[0].Error == nil || result.Items[0].Error.Code != "DATABASE_UPDATE_FAILED" {
		t.Fatalf("item error = %#v, want DATABASE_UPDATE_FAILED", result.Items[0].Error)
	}
	if _, err := os.Stat(filepath.Join(destination, "photo.jpg")); err != nil {
		t.Fatalf("file should have moved before metadata save failed: %v", err)
	}
	history, err := jobStore.ListByDestination(testContext(), destination)
	if err != nil || len(history) != 1 || history[0].Status != "failed" || history[0].FailedCount != 1 {
		t.Fatalf("backup history after metadata failure: %v %#v", err, history)
	}
}

func TestDriveServiceDeleteFileRemovesFileAndMarksRecordDeleted(t *testing.T) {
	destination := t.TempDir()
	filePath := filepath.Join(destination, "photo.jpg")
	if err := os.WriteFile(filePath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileStore, jobStore, closeDB := newSQLiteStores(t)
	defer closeDB()
	if err := fileStore.SaveByPath(testContext(), &dbmodel.FileRecord{
		FileName: filepath.Base(filePath), Path: filePath, SizeBytes: 7,
		MimeType: "image/jpeg", ModifiedAt: time.Now(), AiStatus: repository.AIStatusUnanalyzed,
		Status: repository.FileRecordStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	record, err := fileStore.GetByPath(testContext(), filePath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := NewDriveServiceWithRepositories(fileStore, jobStore).DeleteFile(int64(record.ID), destination)
	if err != nil {
		t.Fatalf("DeleteFile() error = %v", err)
	}
	if result.Status != "deleted" || result.FileID != int64(record.ID) || result.DeletedAt == "" {
		t.Fatalf("unexpected delete result: %#v", result)
	}
	if _, err := os.Stat(filePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still exists or unexpected stat error: %v", err)
	}
	updated, err := fileStore.GetByID(testContext(), record.ID)
	if err != nil || updated.Status != repository.FileRecordStatusDeleted {
		t.Fatalf("record after delete: %v %#v", err, updated)
	}
}

func TestDriveServiceEmitsWailsBackupProgressEvent(t *testing.T) {
	service := NewDriveService()
	ctx := context.WithValue(context.Background(), "test", "wails context")
	service.SetWailsContext(ctx)

	var emittedContext context.Context
	var eventName string
	var eventData []interface{}
	service.emitEvent = func(ctx context.Context, name string, data ...interface{}) {
		emittedContext = ctx
		eventName = name
		eventData = data
	}
	service.setProgress(BackupProgress{Completed: 1, Total: 2, CurrentPath: "photo.jpg", Status: "moving"})

	if emittedContext != ctx || eventName != "backup:progress" || len(eventData) != 1 {
		t.Fatalf("unexpected event: context=%v name=%q data=%#v", emittedContext, eventName, eventData)
	}
	progress, ok := eventData[0].(BackupProgress)
	if !ok || progress.Completed != 1 || progress.Total != 2 || progress.Status != "moving" {
		t.Fatalf("event progress = %#v", eventData[0])
	}
}

func TestDriveServiceScanDriveReturnsMissingDatabaseRecordAlongsideDiskFiles(t *testing.T) {
	destination := t.TempDir()
	diskPath := filepath.Join(destination, "present.jpg")
	missingPath := filepath.Join(destination, "missing.jpg")
	if err := os.WriteFile(diskPath, []byte("present"), 0o600); err != nil {
		t.Fatal(err)
	}
	fileStore, jobStore, closeDB := newSQLiteStores(t)
	defer closeDB()
	if err := fileStore.SaveByPath(testContext(), &dbmodel.FileRecord{
		FileName: filepath.Base(missingPath), Path: missingPath, SizeBytes: 7,
		MimeType: "image/jpeg", ModifiedAt: time.Now(), Description: "file disappeared",
		AiStatus: repository.AIStatusAnalyzed, Status: repository.FileRecordStatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	files, err := NewDriveServiceWithRepositories(fileStore, jobStore).ScanDrive(destination)
	if err != nil {
		t.Fatalf("ScanDrive() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ScanDrive() returned %d files, want 2: %#v", len(files), files)
	}
	for _, file := range files {
		if file.Path == missingPath {
			if file.FileStatus != "missing" || file.Description != "file disappeared" {
				t.Fatalf("missing file result = %#v", file)
			}
			return
		}
	}
	t.Fatalf("missing database record was not returned: %#v", files)
}

type failingFileRecordStore struct{}

func (failingFileRecordStore) SaveByPath(context.Context, *dbmodel.FileRecord) error {
	return errors.New("database write failed")
}

func (failingFileRecordStore) GetByID(context.Context, int32) (*dbmodel.FileRecord, error) {
	return nil, gorm.ErrRecordNotFound
}

func (failingFileRecordStore) GetByPath(context.Context, string) (*dbmodel.FileRecord, error) {
	return nil, gorm.ErrRecordNotFound
}

func (failingFileRecordStore) ListByDestinationPath(context.Context, string) ([]*dbmodel.FileRecord, error) {
	return nil, nil
}

func (failingFileRecordStore) UpdateDescription(context.Context, int32, string, string) error {
	return nil
}
func (failingFileRecordStore) UpdateStatus(context.Context, int32, string) error { return nil }
func (failingFileRecordStore) SearchByDescription(context.Context, string, string) ([]*dbmodel.FileRecord, error) {
	return nil, nil
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

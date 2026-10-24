package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	dbmodel "photo-backup-manager/model/model"
	"photo-backup-manager/repository"
)

func copyBackupFixture(t *testing.T) (BackupRequest, string) {
	t.Helper()
	source, destination := t.TempDir(), t.TempDir()
	photo := filepath.Join(source, "photo.jpg")
	if err := os.WriteFile(photo, []byte("copy fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	return BackupRequest{Mode: "copy", SourcePath: source, DestinationPath: destination, FilePaths: []string{photo}}, filepath.Join(destination, "photo.jpg")
}

func TestCopyBackupPreservesTrackedSourceAndPersistsDestination(t *testing.T) {
	request, destination := copyBackupFixture(t)
	files, jobs, closeDB := newSQLiteStores(t)
	defer closeDB()
	source := &dbmodel.FileRecord{
		FileName: "photo.jpg", Path: request.FilePaths[0], SizeBytes: 12,
		MimeType: "image/jpeg", ModifiedAt: time.Now(),
		AiStatus: repository.AIStatusAnalyzed, Description: "original description",
		Status: repository.FileRecordStatusActive,
	}
	if err := files.SaveByPath(testContext(), source); err != nil {
		t.Fatal(err)
	}
	before, err := files.GetByID(testContext(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewDriveServiceWithRepositories(files, jobs)
	svc.SetWailsContext(context.Background())
	var events []BackupProgress
	svc.emitEvent = func(_ context.Context, name string, values ...interface{}) {
		if name == "backup:progress" {
			events = append(events, values[0].(BackupProgress))
		}
	}
	result, err := svc.StartBackup(request)
	if err != nil || result.SuccessCount != 1 || result.FailedCount != 0 || result.SkippedCount != 0 || len(result.Items) != 1 || result.Items[0].Status != "copied" {
		t.Fatalf("unexpected copy result: %#v, %v", result, err)
	}
	for _, path := range []string{request.FilePaths[0], destination} {
		if data, err := os.ReadFile(path); err != nil || string(data) != "copy fixture" {
			t.Fatalf("unexpected content at %s: %q, %v", path, data, err)
		}
	}
	after, err := files.GetByID(testContext(), source.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("source metadata changed: before=%#v after=%#v error=%v", before, after, err)
	}
	copied, err := files.GetByPath(testContext(), destination)
	if err != nil || copied.ID == source.ID || copied.BackupJobID != int32(result.JobID) || copied.Status != repository.FileRecordStatusActive || copied.AiStatus != repository.AIStatusUnanalyzed {
		t.Fatalf("unexpected destination metadata: %#v, %v", copied, err)
	}
	history, err := svc.GetBackupHistory(request.DestinationPath)
	if err != nil || len(history) != 1 || history[0].ID != result.JobID || history[0].SuccessCount != 1 || history[0].Status != "completed" {
		t.Fatalf("unexpected history: %#v, %v", history, err)
	}
	if len(events) < 2 || events[0].Status != "copying" || events[0].JobID != result.JobID || events[len(events)-1].Status != "completed" {
		t.Fatalf("unexpected progress events: %#v", events)
	}
	// A repeated request skips the destination and leaves both records intact.
	repeated, err := svc.StartBackup(request)
	if err != nil || repeated.SkippedCount != 1 || repeated.FailedCount != 0 || repeated.SuccessCount != 0 || repeated.Items[0].Status != "skipped" {
		t.Fatalf("unexpected repeated copy: %#v, %v", repeated, err)
	}
	unchanged, err := files.GetByPath(testContext(), destination)
	if err != nil || !reflect.DeepEqual(copied, unchanged) {
		t.Fatalf("duplicate copy changed metadata: %#v, %v", unchanged, err)
	}
}

func TestCopyBackupUntrackedSourceAndExplicitMove(t *testing.T) {
	for _, mode := range []string{"copy", "move"} {
		t.Run(mode, func(t *testing.T) {
			request, destination := copyBackupFixture(t)
			request.Mode = mode
			files, jobs, closeDB := newSQLiteStores(t)
			defer closeDB()
			result, err := NewDriveServiceWithRepositories(files, jobs).StartBackup(request)
			wantStatus := "copied"
			if mode == "move" {
				wantStatus = "moved"
			}
			if err != nil || result.SuccessCount != 1 || result.Items[0].Status != wantStatus {
				t.Fatalf("unexpected result: %#v, %v", result, err)
			}
			_, sourceErr := os.Stat(request.FilePaths[0])
			if (mode == "copy" && sourceErr != nil) || (mode == "move" && !errors.Is(sourceErr, os.ErrNotExist)) {
				t.Fatalf("unexpected source state in %s mode: %v", mode, sourceErr)
			}
			if _, err := files.GetByPath(testContext(), destination); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBackupRejectsUnknownModeBeforeCreatingJobOrChangingFiles(t *testing.T) {
	request, destination := copyBackupFixture(t)
	request.Mode = "invalid"
	files, jobs, closeDB := newSQLiteStores(t)
	defer closeDB()
	result, err := NewDriveServiceWithRepositories(files, jobs).StartBackup(request)
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != "INVALID_REQUEST" || result.JobID != 0 {
		t.Fatalf("unexpected result: %#v, %v", result, err)
	}
	if _, err := os.Stat(request.FilePaths[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected destination state: %v", err)
	}
	history, err := jobs.ListByDestination(testContext(), request.DestinationPath)
	if err != nil || len(history) != 0 {
		t.Fatalf("unexpected history: %#v, %v", history, err)
	}
}

func TestCopyBackupMetadataFailureKeepsBothFiles(t *testing.T) {
	request, destination := copyBackupFixture(t)
	_, jobs, closeDB := newSQLiteStores(t)
	defer closeDB()
	result, err := NewDriveServiceWithRepositories(failingFileRecordStore{}, jobs).StartBackup(request)
	if err != nil || result.FailedCount != 1 || result.SuccessCount != 0 || result.Items[0].Error == nil || result.Items[0].Error.Code != "DATABASE_UPDATE_FAILED" {
		t.Fatalf("unexpected result: %#v, %v", result, err)
	}
	for _, path := range []string{request.FilePaths[0], destination} {
		if data, err := os.ReadFile(path); err != nil || string(data) != "copy fixture" {
			t.Fatalf("unexpected file after metadata failure: %q, %v", data, err)
		}
	}
}

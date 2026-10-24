package repository

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dbmodel "photo-backup-manager/model/model"
)

func TestRepositoriesPersistAndScopeByDestination(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "photo_backup.db")
	t.Setenv("PHOTO_BACKUP_DB", dbPath)
	db, err := NewDbConnection()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	jobs := NewBackupJobRepository(db)
	files := NewFileRecordRepository(db)
	destinationA := filepath.Join(t.TempDir(), "photos")
	destinationB := filepath.Join(t.TempDir(), "photos")
	started := time.Now().UTC().Truncate(time.Second)
	jobA := &dbmodel.BackupJob{Source: "source", Destination: destinationA, StartedAt: started, Status: "running"}
	jobB := &dbmodel.BackupJob{Source: "source", Destination: destinationB, StartedAt: started, Status: "running"}
	for _, job := range []*dbmodel.BackupJob{jobA, jobB} {
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatal(err)
		}
	}
	jobA.CompletedAt = started.Add(time.Second)
	jobA.TotalFiles, jobA.SuccessCount, jobA.DurationMs, jobA.Status = 1, 1, 1000, "completed"
	if err := jobs.Finish(ctx, jobA); err != nil {
		t.Fatal(err)
	}
	pathA := filepath.Join(destinationA, "photo.jpg")
	pathB := filepath.Join(destinationB, "photo.jpg")
	for _, item := range []struct {
		path string
		job  int32
	}{{pathA, jobA.ID}, {pathB, jobB.ID}} {
		if err := files.SaveByPath(ctx, &dbmodel.FileRecord{
			FileName: "photo.jpg", Path: item.path, SizeBytes: 5, ModifiedAt: started,
			BackupJobID: item.job, Description: "แมว", AiStatus: AIStatusAnalyzed, Status: FileRecordStatusActive,
		}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := files.GetByPath(ctx, pathA)
	if err != nil {
		t.Fatal(err)
	}
	if err := files.SaveByPath(ctx, &dbmodel.FileRecord{
		FileName: "photo.jpg", Path: pathA, SizeBytes: 8, ModifiedAt: started,
		BackupJobID: jobA.ID, Description: "แมวตัวใหม่", AiStatus: AIStatusAnalyzed, Status: FileRecordStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := files.GetByPath(ctx, pathA)
	if err != nil || updated.ID != first.ID || updated.SizeBytes != 8 {
		t.Fatalf("SaveByPath should update same record: first=%#v updated=%#v err=%v", first, updated, err)
	}
	if err := files.UpdateStatus(ctx, updated.ID, FileRecordStatusMissing); err != nil {
		t.Fatal(err)
	}
	if err := files.UpdateStatus(ctx, updated.ID, FileRecordStatusActive); err != nil {
		t.Fatal(err)
	}

	closeDB := func() {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatal(err)
		}
	}
	closeDB()
	db, err = NewDbConnection()
	if err != nil {
		t.Fatalf("reopen and migrate database: %v", err)
	}
	defer func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() }()
	jobs = NewBackupJobRepository(db)
	files = NewFileRecordRepository(db)
	history, err := jobs.ListByDestination(ctx, destinationA)
	if err != nil || len(history) != 1 || history[0].ID != jobA.ID || history[0].Status != "completed" || history[0].SuccessCount != 1 {
		t.Fatalf("destination A history after reopen = %#v, %v", history, err)
	}
	otherHistory, err := jobs.ListByDestination(ctx, destinationB)
	if err != nil || len(otherHistory) != 1 || otherHistory[0].Status != "interrupted" || otherHistory[0].CompletedAt.IsZero() {
		t.Fatalf("running job should be interrupted after reopen: %#v, %v", otherHistory, err)
	}
	results, err := files.SearchByDescription(ctx, destinationA, "แมว")
	if err != nil || len(results) != 1 || results[0].Path != pathA || results[0].Description != "แมวตัวใหม่" {
		t.Fatalf("destination A search after reopen = %#v, %v", results, err)
	}
	if err := files.UpdateStatus(ctx, results[0].ID, FileRecordStatusMissing); err != nil {
		t.Fatal(err)
	}
	results, err = files.SearchByDescription(ctx, destinationA, "แมว")
	if err != nil || len(results) != 0 {
		t.Fatalf("missing file should not appear in search: %#v, %v", results, err)
	}
	listed, err := files.ListByDestinationPath(ctx, destinationA)
	if err != nil || len(listed) != 1 || listed[0].Status != FileRecordStatusMissing {
		t.Fatalf("missing file should remain in destination listing: %#v, %v", listed, err)
	}
}

func TestDatabaseRejectsDuplicateFilePaths(t *testing.T) {
	t.Setenv("PHOTO_BACKUP_DB", filepath.Join(t.TempDir(), "photo_backup.db"))
	db, err := NewDbConnection()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { sqlDB, _ := db.DB(); _ = sqlDB.Close() }()
	path := filepath.Join(t.TempDir(), "photo.jpg")
	record := dbmodel.FileRecord{FileName: "photo.jpg", Path: path, AiStatus: AIStatusUnanalyzed, Status: FileRecordStatusActive}
	if err := db.Omit("BackupJobID").Create(&record).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := dbmodel.FileRecord{FileName: "photo.jpg", Path: path, AiStatus: AIStatusUnanalyzed, Status: FileRecordStatusActive}
	if err := db.Omit("BackupJobID").Create(&duplicate).Error; err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Fatalf("duplicate path should violate unique index, got %v", err)
	}
}

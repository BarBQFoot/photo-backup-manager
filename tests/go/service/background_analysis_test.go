package service

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"photo-backup-manager/repository"
)

// Exercise actual SQLite records and file operations while the worker is paused.
func TestBackgroundAnalysisAllowsCompletedMoveAndProtectsPending(t *testing.T) {
	fileStore, jobStore, closeDB := newSQLiteStores(t)
	defer closeDB()
	s := NewDriveServiceWithRepositories(fileStore, jobStore)
	root := t.TempDir()
	source, destination, copied := filepath.Join(root, "source"), filepath.Join(root, "destination"), filepath.Join(root, "copy")
	for _, dir := range []string{source, destination, copied} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ids := []int64{}
	paths := []string{}
	for _, name := range []string{"ready.jpg", "pending.jpg"} {
		path := filepath.Join(source, name)
		if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := s.saveFileRecord(path, 0); err != nil {
			t.Fatal(err)
		}
		record, err := fileStore.GetByPath(testContext(), path)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, int64(record.ID))
		paths = append(paths, path)
	}
	ready, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	finish := func() { releaseOnce.Do(func() { close(release) }) }
	var out AIBatchResult
	var analysisErr error
	s.setAIAnalyzer(batchMock{run: func(in []AIInput, cb func(AIEvent)) error {
		cb(AIEvent{Type: "result", FileID: in[0].FileID, Caption: "saved first caption"})
		close(ready)
		<-release
		cb(AIEvent{Type: "result", FileID: in[1].FileID, Caption: "saved second caption"})
		return nil
	}})
	go func() { defer close(done); out, analysisErr = s.AnalyzeImages(ids) }()
	defer func() {
		finish()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("worker did not finish")
		}
	}()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not start")
	}

	// Browse and rescan without recovering a live job as failed.
	files, err := s.ScanDrive(source)
	if err != nil || len(files) != 2 {
		t.Fatalf("scan: %v, %v", files, err)
	}
	metadata, err := s.GetFileMetadata(ids[1])
	if err != nil || metadata.AIStatus != repository.AIStatusAnalyzing {
		t.Fatalf("pending: %+v %v", metadata, err)
	}
	_, err = s.DeleteFile(ids[1], source)
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Code != "AI_FILE_BUSY" {
		t.Fatalf("pending delete: %v", err)
	}

	// The user's existing copy API can copy a pending source without moving it.
	copyResult, err := s.StartBackup(BackupRequest{Mode: "copy", SourcePath: source, DestinationPath: copied, FilePaths: paths[1:]})
	if err != nil || copyResult.SuccessCount != 1 {
		t.Fatalf("copy: %+v %v", copyResult, err)
	}
	result, err := s.StartBackup(BackupRequest{SourcePath: source, DestinationPath: destination, FilePaths: paths})
	if err != nil || result.SuccessCount != 1 || result.FailedCount != 1 || result.Items[1].Error.Code != "AI_FILE_BUSY" {
		t.Fatalf("move: %+v %v", result, err)
	}
	if _, err := os.Stat(paths[1]); err != nil {
		t.Fatal("pending source moved", err)
	}
	moved, err := fileStore.GetByID(testContext(), int32(ids[0]))
	if err != nil || moved.Path != filepath.Join(destination, "ready.jpg") || moved.Description != "saved first caption" {
		t.Fatalf("moved: %+v %v", moved, err)
	}
	if _, err := s.DeleteFile(ids[0], destination); err != nil {
		t.Fatal("completed file cannot be deleted", err)
	}

	// A second analysis must be rejected while the first is still alive.
	_, err = s.AnalyzeImages([]int64{ids[1]})
	if !errors.As(err, &appErr) || appErr.Code != "AI_BUSY" {
		t.Fatalf("second analysis: %v", err)
	}
	// Finish explicitly, leaving the deferred cleanup safe on early failures.
	finish()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("analysis blocked by file operations")
	}
	if analysisErr != nil || len(out.Results) != 2 || len(out.Failures) != 0 {
		t.Fatalf("analysis: %+v %v", out, analysisErr)
	}
	pending, err := fileStore.GetByID(testContext(), int32(ids[1]))
	if err != nil || pending.Path != paths[1] || pending.Description != "saved second caption" || pending.AiStatus != repository.AIStatusAnalyzed {
		t.Fatalf("final file_%d: %+v %v", ids[1], pending, err)
	}
}

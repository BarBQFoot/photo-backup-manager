package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbmodel "photo-backup-manager/model/model"
	"photo-backup-manager/repository"
)

func TestWorkerEnvironmentSeparatesWailsLogLevel(t *testing.T) {
	base := []string{"loglevel=Info", "LOGLEVEL=Debug", "OTHER_SETTING=preserved", "PYTHONUTF8=0"}
	env := aiWorkerEnvironment(base)
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(name, "loglevel") {
			t.Fatal("Wails loglevel leaked into Python")
		}
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "OTHER_SETTING=preserved") || !strings.Contains(joined, "PYTHONUTF8=1") || strings.Contains(joined, "PYTHONUTF8=0") {
		t.Fatalf("bad environment: %v", env)
	}
	if base[0] != "loglevel=Info" {
		t.Fatal("parent environment was changed")
	}
}

type batchStore struct {
	fakeFileStore
	records map[int32]*dbmodel.FileRecord
}

func (f *batchStore) GetByID(_ context.Context, id int32) (*dbmodel.FileRecord, error) {
	if r := f.records[id]; r != nil {
		return r, nil
	}
	return nil, os.ErrNotExist
}

func (f *batchStore) UpdateDescription(_ context.Context, id int32, description, status string) error {
	f.records[id].Description, f.records[id].AiStatus = description, status
	return nil
}

type batchMock struct {
	run func([]AIInput, func(AIEvent)) error
}

func (b batchMock) Analyze(context.Context, string, string) (string, error) {
	panic("must use one batch")
}
func (b batchMock) AnalyzeBatch(_ context.Context, in []AIInput, cb func(AIEvent)) error {
	return b.run(in, cb)
}

func setupBatch(t *testing.T) (*DriveService, *batchStore) {
	t.Helper()
	dir := t.TempDir()
	store := &batchStore{records: map[int32]*dbmodel.FileRecord{}}
	for id := int32(1); id <= 2; id++ {
		path := filepath.Join(dir, string(rune('a'+id))+".jpg")
		if err := os.WriteFile(path, []byte("photo"), 0o600); err != nil {
			t.Fatal(err)
		}
		store.records[id] = &dbmodel.FileRecord{ID: id, Path: path, MimeType: "image/jpeg", Status: repository.FileRecordStatusActive, Description: "old"}
	}
	return NewDriveServiceWithRepositories(store, nil), store
}

func TestBatchSavesByIDWhenResultsAreReordered(t *testing.T) {
	s, store := setupBatch(t)
	s.setAIAnalyzer(batchMock{run: func(in []AIInput, cb func(AIEvent)) error {
		if len(in) != 2 {
			t.Fatalf("inputs: %#v", in)
		}
		cb(AIEvent{Type: "result", FileID: "file_2", Caption: "second"})
		cb(AIEvent{Type: "result", FileID: "file_1", Caption: "first"})
		return nil
	}})
	out, err := s.AnalyzeImages([]int64{1, 2, 1})
	if err != nil || len(out.Results) != 2 || len(out.Failures) != 0 {
		t.Fatalf("%#v %v", out, err)
	}
	if store.records[1].Description != "first" || store.records[2].Description != "second" {
		t.Fatal("wrong identity mapping")
	}
}

func TestBatchTimeoutKeepsSavedResultsAndOldPendingDescription(t *testing.T) {
	s, store := setupBatch(t)
	s.setAIAnalyzer(batchMock{run: func(in []AIInput, cb func(AIEvent)) error {
		cb(AIEvent{Type: "result", FileID: "file_1", Caption: "saved"})
		return errors.New("timeout; saved batches preserved")
	}})
	out, err := s.AnalyzeImages([]int64{1, 2})
	if err != nil || len(out.Results) != 1 || len(out.Failures) != 1 {
		t.Fatalf("%#v %v", out, err)
	}
	if store.records[1].Description != "saved" || store.records[1].AiStatus != repository.AIStatusAnalyzed {
		t.Fatal("completed result lost")
	}
	if store.records[2].Description != "old" || store.records[2].AiStatus != repository.AIStatusFailed {
		t.Fatal("pending description overwritten")
	}
}

func TestBatchRejectsForeignIDAndChangedFile(t *testing.T) {
	s, store := setupBatch(t)
	s.setAIAnalyzer(batchMock{run: func(in []AIInput, cb func(AIEvent)) error {
		cb(AIEvent{Type: "result", FileID: "file_999", Caption: "foreign"})
		if err := os.WriteFile(store.records[1].Path, []byte("changed photo contents"), 0o600); err != nil {
			t.Fatal(err)
		}
		cb(AIEvent{Type: "result", FileID: "file_1", Caption: "stale"})
		return nil
	}})
	out, err := s.AnalyzeImages([]int64{1, 2, 1<<32 + 1})
	if err != nil || len(out.Results) != 0 || len(out.Failures) != 3 {
		t.Fatalf("%#v %v", out, err)
	}
	if store.records[1].Description != "old" || store.records[2].Description != "old" {
		t.Fatal("invalid result saved")
	}
}

func TestEmbeddedWorkerResources(t *testing.T) {
	root, err := aiWorkerResources(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tools/app_caption_worker.py", "tools/translate_captions.py", "docs/GEMINI-TRANSLATE-CAPTIONS.txt"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || len(data) == 0 {
			t.Fatalf("missing resource %s: %v", name, err)
		}
	}
}

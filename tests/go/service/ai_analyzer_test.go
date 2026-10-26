package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	dbmodel "photo-backup-manager/model/model"
	"photo-backup-manager/repository"
)

type mockAnalyzer struct {
	description string
	err         error
}

func (m mockAnalyzer) Analyze(context.Context, string, string) (string, error) {
	return m.description, m.err
}

func TestNewDriveServiceUsesHybridAnalyzer(t *testing.T) {
	service := NewDriveService()
	if _, ok := service.aiAnalyzer.(*HybridAnalyzer); !ok {
		t.Fatalf("default analyzer = %T, want Florence/text-translation pipeline", service.aiAnalyzer)
	}
	if _, ok := service.aiAnalyzer.(BatchAIAnalyzer); !ok {
		t.Fatal("default analyzer must support batch captioning and translation")
	}
}

func TestAnalyzeImageMockSavesDescriptionOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.jpg")
	if err := os.WriteFile(path, []byte{0xff, 0xd8, 0xff, 0xd9}, 0o600); err != nil {
		t.Fatal(err)
	}
	record := &dbmodel.FileRecord{
		ID: 1, FileName: "sample.jpg", Path: path, MimeType: "image/jpeg",
		AiStatus: repository.AIStatusUnanalyzed, Status: repository.FileRecordStatusActive,
	}
	store := &fakeFileStore{record: record}
	service := NewDriveServiceWithRepositories(store, nil)
	service.setAIAnalyzer(mockAnalyzer{description: "A cat sitting indoors"})

	result, err := service.AnalyzeImage(1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Description != "A cat sitting indoors" || result.Status != "success" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if store.record.Description != result.Description || store.record.AiStatus != repository.AIStatusAnalyzed {
		t.Fatalf("record was not updated correctly: %#v", store.record)
	}
}

func TestAnalyzeImageMockErrorMarksFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.jpg")
	if err := os.WriteFile(path, []byte{0xff, 0xd8, 0xff, 0xd9}, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &fakeFileStore{record: &dbmodel.FileRecord{
		ID: 1, Path: path, MimeType: "image/jpeg", Description: "old description",
		AiStatus: repository.AIStatusAnalyzed, Status: repository.FileRecordStatusActive,
	}}
	service := NewDriveServiceWithRepositories(store, nil)
	service.setAIAnalyzer(mockAnalyzer{err: &AIError{Code: "AI_UNAVAILABLE", Err: os.ErrNotExist}})

	_, err := service.AnalyzeImage(1)
	assertAppErrorCode(t, err, "AI_UNAVAILABLE")
	if store.record.Description != "old description" || store.record.AiStatus != repository.AIStatusFailed {
		t.Fatalf("failed analysis changed description/status incorrectly: %#v", store.record)
	}
}

func assertAppErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	appErr, ok := err.(*AppError)
	if !ok || appErr.Code != want {
		t.Fatalf("error = %#v, want app code %q", err, want)
	}
}

type fakeFileStore struct {
	record *dbmodel.FileRecord
}

func (f *fakeFileStore) SaveByPath(context.Context, *dbmodel.FileRecord) error { return nil }
func (f *fakeFileStore) GetByID(context.Context, int32) (*dbmodel.FileRecord, error) {
	if f.record == nil {
		return nil, os.ErrNotExist
	}
	return f.record, nil
}
func (f *fakeFileStore) GetByPath(context.Context, string) (*dbmodel.FileRecord, error) {
	return f.record, nil
}
func (f *fakeFileStore) MovePath(context.Context, int32, string, int32, int32, time.Time) error {
	return nil
}
func (f *fakeFileStore) ListByDestinationPath(context.Context, string) ([]*dbmodel.FileRecord, error) {
	return nil, nil
}
func (f *fakeFileStore) UpdateDescription(_ context.Context, _ int32, description, status string) error {
	f.record.Description = description
	f.record.AiStatus = status
	return nil
}
func (f *fakeFileStore) UpdateStatus(context.Context, int32, string) error { return nil }
func (f *fakeFileStore) SearchByDescription(context.Context, string, string) ([]*dbmodel.FileRecord, error) {
	return nil, nil
}

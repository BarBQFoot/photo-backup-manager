package service

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestReadImageRejectsUnsupportedAndOversizedFiles(t *testing.T) {
	dir := t.TempDir()
	unsupported := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(unsupported, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := readImage(unsupported, "text/plain")
	assertAIErrorCode(t, err, "UNSUPPORTED_IMAGE")

	large := filepath.Join(dir, "large.jpg")
	if err := os.WriteFile(large, make([]byte, maxImageSizeBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = readImage(large, "image/jpeg")
	assertAIErrorCode(t, err, "UNSUPPORTED_IMAGE")
}

func TestGeminiAnalyzerRequiresKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	_, err := NewGeminiAnalyzerFromEnv().Analyze(context.Background(), "unused.jpg", "image/jpeg")
	assertAIErrorCode(t, err, "AI_KEY_MISSING")
}

func TestGeminiAnalyzerRejectsInvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[]}}]}`))
	}))
	defer server.Close()

	analyzer := NewGeminiAnalyzerFromEnv()
	analyzer.apiKey = "test-only"
	analyzer.endpoint = server.URL
	analyzer.httpClient = server.Client()
	image := filepath.Join(t.TempDir(), "sample.jpg")
	if err := os.WriteFile(image, []byte{0xff, 0xd8, 0xff, 0xd9}, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := analyzer.Analyze(context.Background(), image, "image/jpeg")
	assertAIErrorCode(t, err, "AI_RESPONSE_INVALID")
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

func assertAIErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	aiErr, ok := err.(*AIError)
	if !ok || aiErr.Code != want {
		t.Fatalf("error = %#v, want AI code %q", err, want)
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

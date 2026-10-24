package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Optional process smoke check against a separate cache populated with mocked
// translations by the Python smoke test. An empty key prevents real API calls.
func TestHybridCachedProcessSmoke(t *testing.T) {
	cache := os.Getenv("PHOTO_AI_SMOKE_CACHE")
	if cache == "" {
		t.Skip("set PHOTO_AI_SMOKE_CACHE to an isolated prepared smoke cache")
	}
	t.Setenv("PHOTO_AI_CACHE", cache)
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GEMINI_TRANSLATION_MODEL", "smoke-mock-only")
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PHOTO_AI_PYTHON", filepath.Join(root, ".venv", "Scripts", "python.exe"))
	if _, err := os.Stat(os.Getenv("PHOTO_AI_PYTHON")); err != nil {
		t.Skip("Windows smoke environment unavailable")
	}
	items := []AIInput{
		{FileID: "file_101", Path: filepath.Join(root, "test-images", "1000", "000000166478.jpg")},
		{FileID: "file_102", Path: filepath.Join(root, "test-images", "1000", "000000025139.jpg")},
	}
	results := map[string]string{}
	err = NewHybridAnalyzer().AnalyzeBatch(context.Background(), items, func(event AIEvent) {
		if event.Type == "result" {
			results[event.FileID] = event.Caption
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results["file_101"] == "" || results["file_102"] == "" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

// Explicit opt-in: load the real local model through the GUI subprocess path,
// with an empty API key so no generation request can leave the machine.
func TestHybridRealModelSubprocess(t *testing.T) {
	if os.Getenv("PHOTO_AI_REAL_MODEL_SMOKE") != "1" {
		t.Skip("opt-in local model test")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PHOTO_AI_PYTHON", filepath.Join(root, ".venv", "Scripts", "python.exe"))
	t.Setenv("PHOTO_AI_CACHE", t.TempDir())
	t.Setenv("GEMINI_API_KEY", "")
	// Wails dev exports this mixed-case level. Windows environment names are case-insensitive.
	t.Setenv("loglevel", "Info")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	phase := ""
	err = NewHybridAnalyzer().AnalyzeBatch(ctx, []AIInput{{FileID: "file_1", Path: filepath.Join(root, "test-images", "1000", "000000166478.jpg")}}, func(event AIEvent) {
		if event.Type == "progress" {
			phase = event.Phase
		}
		if event.Type == "failure" {
			t.Errorf("photo failed: %s", event.Message)
		}
	})
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY is not configured") || phase != "translating" {
		t.Fatalf("local model did not complete: phase=%s err=%v", phase, err)
	}
}

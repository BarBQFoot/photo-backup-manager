package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHybridResidentRealModelReuse(t *testing.T) {
	if os.Getenv("PHOTO_AI_RESIDENT_SMOKE") != "1" {
		t.Skip("opt-in local GPU model reuse test; no API key")
	}
	root, _ := filepath.Abs("..")
	python := filepath.Join(root, ".venv", "Scripts", "python.exe")
	if _, err := os.Stat(python); err != nil {
		t.Skip("local Windows Python unavailable")
	}
	t.Setenv("PHOTO_AI_PYTHON", python)
	cache := t.TempDir()
	t.Setenv("PHOTO_AI_CACHE", cache)
	t.Setenv("GEMINI_API_KEY", "")
	a := NewHybridAnalyzer()
	defer a.Close()
	var rows []map[string]any
	pid := 0
	for index, file := range []string{"000000025139.jpg", "000000026204.jpg"} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		started := time.Now()
		err := a.AnalyzeBatch(ctx, []AIInput{{FileID: "file_1", Path: filepath.Join(root, "test-images", "50", file)}}, func(AIEvent) {})
		cancel()
		if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY is not configured") {
			t.Fatalf("unexpected model result: %v", err)
		}
		if index == 0 {
			pid = a.worker.cmd.Process.Pid
		} else if pid != a.worker.cmd.Process.Pid {
			t.Fatal("Python restarted between jobs")
		}
		data, err := os.ReadFile(filepath.Join(cache, "captions", "pipeline_summary.json"))
		if err != nil {
			t.Fatal(err)
		}
		var row map[string]any
		if err := json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		if row["model_reused"] != (index == 1) || row["caption_cache_hits"] != float64(0) {
			t.Fatalf("did not generate uncached caption with expected reuse: %#v", row)
		}
		row["go_request_seconds"] = time.Since(started).Seconds()
		row["worker_reused"] = a.lastWorkerStats().Reused
		rows = append(rows, row)
		t.Logf("job %d: %.3fs, model load %v, reused=%v", index+1, row["go_request_seconds"], row["model_load_seconds"], row["model_reused"])
	}
	output := filepath.Join(root, "results", "performance", "resident_worker_smoke.json")
	_ = os.MkdirAll(filepath.Dir(output), 0o700)
	encoded, _ := json.MarshalIndent(rows, "", "  ")
	if err := os.WriteFile(output, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

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
	analyzer := NewHybridAnalyzer()
	defer analyzer.Close()
	err = analyzer.AnalyzeBatch(context.Background(), items, func(event AIEvent) {
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
	analyzer := NewHybridAnalyzer()
	defer analyzer.Close()
	err = analyzer.AnalyzeBatch(ctx, []AIInput{{FileID: "file_1", Path: filepath.Join(root, "test-images", "1000", "000000166478.jpg")}}, func(event AIEvent) {
		if event.Type == "progress" {
			phase = event.Phase
		}
		if event.Type == "failure" {
			t.Errorf("photo failed: %s", event.Message)
		}
	})
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY is not configured") || phase != "pipeline" {
		t.Fatalf("local model did not complete: phase=%s err=%v", phase, err)
	}
}

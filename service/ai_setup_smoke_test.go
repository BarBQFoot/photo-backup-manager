package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in: uses the existing local model, with no credentials and no network downloads.
func TestAISetupRealExistingMachine(t *testing.T) {
	if os.Getenv("PHOTO_AI_SETUP_EXISTING_SMOKE") != "1" {
		t.Skip("opt-in existing machine setup check")
	}
	cache := t.TempDir()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PHOTO_AI_CACHE", cache)
	t.Setenv("PHOTO_AI_PYTHON", filepath.Join(root, ".venv", "Scripts", "python.exe"))
	t.Setenv("GEMINI_API_KEY", "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for attempt := 0; attempt < 2; attempt++ {
		phases := map[string]bool{}
		device := ""
		err := setupLocalAI(ctx, func(status AISetupStatus) {
			phases[status.Phase] = true
			if status.Device != "" {
				device = status.Device
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, phase := range []string{"bootstrap", "python", "pytorch", "packages", "model"} {
			if phases[phase] {
				t.Fatalf("existing machine attempted %s", phase)
			}
		}
		if attempt == 0 && !phases["verify"] {
			t.Fatal("first verification skipped")
		}
		if attempt == 1 && phases["verify"] {
			t.Fatal("second launch repeated verification")
		}
		if device == "" {
			t.Fatal("device missing")
		}
		t.Logf("attempt=%d device=%s phases=%v; no downloads or Gemini", attempt+1, device, phases)
	}
}

// Opt-in: downloads only uv and a clean Python into an isolated temporary directory.
// Does not install PyTorch or download Florence and never calls Gemini.
func TestAISetupFreshPythonBootstrap(t *testing.T) {
	if os.Getenv("PHOTO_AI_SETUP_BOOTSTRAP_SMOKE") != "1" {
		t.Skip("opt-in Python download check")
	}
	cache := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	update := func(AISetupStatus) {}
	uv, err := prepareUV(ctx, cache, update)
	if err != nil {
		t.Fatal(err)
	}
	root, err := aiWorkerResources(cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = runSetupProcess(ctx, cache, root, "python", uv,
		[]string{"venv", "--python", "3.13", "--managed-python", filepath.Join(cache, "python-env")}, update); err != nil {
		t.Fatal(err)
	}
	report, err := runSetupProcess(ctx, cache, root, "checking", managedAIPython(cache),
		[]string{"-u", filepath.Join(root, "tools", "bootstrap_ai.py"), "check"}, update)
	if err != nil {
		t.Fatal(err)
	}
	if !report.PythonReady || report.DependenciesReady {
		t.Fatalf("unexpected fresh environment: %#v", report)
	}
	t.Log("Pinned uv checksum validated; downloaded Python starts and reports missing AI dependencies correctly")
}

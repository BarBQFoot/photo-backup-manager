package service

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAISetupReusesReadyLibrariesAndModelThenSkipsVerification(t *testing.T) {
	cache := t.TempDir()
	python := filepath.Join(cache, "existing", "Scripts", "python.exe")
	report := setupReport{PythonReady: true, DependenciesReady: true, TorchReady: true, ModelReady: true, Signature: "installed-model-signature"}
	calls := []string{}
	operations := aiSetupOperations{
		discover: func(context.Context, string, string, string, func(AISetupStatus)) (string, setupReport) {
			return python, report
		},
		uv: func(context.Context, string, func(AISetupStatus)) (string, error) {
			t.Fatal("ready machine must not download uv/Python")
			return "", nil
		},
		command: func(_ context.Context, _, _, phase, _ string, _ []string, _ func(AISetupStatus)) (setupReport, error) {
			calls = append(calls, phase)
			if phase != "verify" {
				t.Fatalf("unexpected download/install: %s", phase)
			}
			return setupReport{Verified: true, Device: "cuda"}, nil
		},
	}
	for i := 0; i < 2; i++ {
		if err := setupLocalAIWith(context.Background(), cache, cache, operations, func(AISetupStatus) {}); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(calls, []string{"verify"}) {
		t.Fatalf("calls = %v", calls)
	}
	report.Signature = "updated-model-signature"
	if err := setupLocalAIWith(context.Background(), cache, cache, operations, func(AISetupStatus) {}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"verify", "verify"}) {
		t.Fatal("changed model was not rechecked")
	}
}

func TestAISetupReusesSystemPythonWithoutDownloadingAnotherInterpreter(t *testing.T) {
	cache := t.TempDir()
	python := filepath.Join(cache, "system-python", "python.exe")
	calls := []string{}
	operations := aiSetupOperations{
		discover: func(context.Context, string, string, string, func(AISetupStatus)) (string, setupReport) {
			return python, setupReport{PythonReady: true, TorchReady: true}
		},
		uv: func(context.Context, string, func(AISetupStatus)) (string, error) { return "mock-uv", nil },
		command: func(_ context.Context, _, _, phase, _ string, args []string, _ func(AISetupStatus)) (setupReport, error) {
			calls = append(calls, phase)
			if phase == "python" {
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "--managed-python") || !strings.Contains(joined, python) || !strings.Contains(joined, "--system-site-packages") {
					t.Fatalf("did not reuse system Python: %v", args)
				}
			}
			return setupReport{PythonReady: true, DependenciesReady: true, TorchReady: true, ModelReady: true, Signature: "system-ready", Verified: true, Device: "cpu"}, nil
		},
	}
	if err := setupLocalAIWith(context.Background(), cache, cache, operations, func(AISetupStatus) {}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"python", "packages", "checking", "verify"}) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestAISetupDownloadsOnlyMissingModel(t *testing.T) {
	cache := t.TempDir()
	calls := []string{}
	operations := aiSetupOperations{
		discover: func(context.Context, string, string, string, func(AISetupStatus)) (string, setupReport) {
			return filepath.Join(cache, "python"), setupReport{PythonReady: true, DependenciesReady: true, TorchReady: true}
		},
		uv: func(context.Context, string, func(AISetupStatus)) (string, error) {
			t.Fatal("libraries already installed")
			return "", nil
		},
		command: func(_ context.Context, _, _, phase, _ string, _ []string, _ func(AISetupStatus)) (setupReport, error) {
			calls = append(calls, phase)
			if phase == "checking" {
				return setupReport{PythonReady: true, DependenciesReady: true, TorchReady: true, ModelReady: true, Signature: "downloaded"}, nil
			}
			return setupReport{Verified: true, Device: "cpu"}, nil
		},
	}
	if err := setupLocalAIWith(context.Background(), cache, cache, operations, func(AISetupStatus) {}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"model", "checking", "verify"}) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestAISetupPreservesPostDownloadDiagnostics(t *testing.T) {
	for _, probeFails := range []bool{false, true} {
		t.Run(fmt.Sprint(probeFails), func(t *testing.T) {
			cache := t.TempDir()
			operations := aiSetupOperations{
				discover: func(context.Context, string, string, string, func(AISetupStatus)) (string, setupReport) {
					return "python", setupReport{PythonReady: true, DependenciesReady: true, TorchReady: true}
				},
				command: func(_ context.Context, _, _, phase, _ string, _ []string, _ func(AISetupStatus)) (setupReport, error) {
					if phase == "checking" {
						if probeFails {
							return setupReport{}, errors.New("PermissionError: cache access denied")
						}
						return setupReport{ModelPath: "actual-cache", ModelError: "missing tokenizer.json"}, nil
					}
					return setupReport{}, nil
				},
			}
			err := setupLocalAIWith(context.Background(), cache, cache, operations, func(AISetupStatus) {})
			if err == nil {
				t.Fatal("incomplete model must not be marked ready")
			}
			if probeFails && !strings.Contains(err.Error(), "cache access denied") {
				t.Fatal(err)
			}
			if !probeFails && (!strings.Contains(err.Error(), "actual-cache") || !strings.Contains(err.Error(), "tokenizer.json")) {
				t.Fatal(err)
			}
		})
	}
}

func TestAISetupSkipsExistingPyTorchWhenOtherPackagesMissing(t *testing.T) {
	cache := t.TempDir()
	python := managedAIPython(cache)
	if err := os.MkdirAll(filepath.Dir(python), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "python-env", "pyvenv.cfg"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := []string{}
	operations := aiSetupOperations{
		discover: func(context.Context, string, string, string, func(AISetupStatus)) (string, setupReport) {
			return python, setupReport{PythonReady: true, TorchReady: true}
		},
		uv: func(context.Context, string, func(AISetupStatus)) (string, error) { return "mock-uv", nil },
		command: func(_ context.Context, _, _, phase, _ string, _ []string, _ func(AISetupStatus)) (setupReport, error) {
			calls = append(calls, phase)
			return setupReport{PythonReady: true, DependenciesReady: true, TorchReady: true, ModelReady: true, Signature: "ready", Verified: true, Device: "cpu"}, nil
		},
	}
	if err := setupLocalAIWith(context.Background(), cache, cache, operations, func(AISetupStatus) {}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"packages", "checking", "verify"}) {
		t.Fatalf("calls = %v", calls)
	}
}

func TestAISetupDoesNotMarkFailedInstallReady(t *testing.T) {
	t.Setenv("PHOTO_AI_PYTHON", "")
	cache := t.TempDir()
	operations := aiSetupOperations{
		discover: func(context.Context, string, string, string, func(AISetupStatus)) (string, setupReport) {
			return "", setupReport{}
		},
		uv: func(context.Context, string, func(AISetupStatus)) (string, error) { return "mock-uv", nil },
		command: func(_ context.Context, _, _, phase, _ string, args []string, _ func(AISetupStatus)) (setupReport, error) {
			if phase == "python" && !strings.Contains(strings.Join(args, " "), "--managed-python") {
				t.Fatal("missing managed Python request")
			}
			if phase == "pytorch" {
				return setupReport{}, errors.New("test download interrupted")
			}
			return setupReport{}, nil
		},
	}
	if err := setupLocalAIWith(context.Background(), cache, cache, operations, func(AISetupStatus) {}); err == nil {
		t.Fatal("want failed setup")
	}
	if _, err := os.Stat(filepath.Join(cache, "python-env-ready.json")); !os.IsNotExist(err) {
		t.Fatal("failed environment marked ready")
	}
}

func TestAISetupSingleJobAndRetry(t *testing.T) {
	setup := NewAISetup()
	var calls atomic.Int32
	release := make(chan struct{})
	setup.run = func(ctx context.Context, update func(AISetupStatus)) error {
		if calls.Add(1) == 1 {
			<-release
			return errors.New("test interrupted")
		}
		update(AISetupStatus{Phase: "verify", Device: "cuda"})
		return nil
	}
	setup.Start(context.Background())
	setup.Start(context.Background())
	close(release)
	awaitSetup := func(want string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if setup.GetStatus().Status == want {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("status = %#v, want %s", setup.GetStatus(), want)
	}
	awaitSetup("error")
	if calls.Load() != 1 {
		t.Fatal("duplicate setup job")
	}
	setup.Start(context.Background())
	awaitSetup("ready")
	if calls.Load() != 2 || setup.GetStatus().Device != "cuda" {
		t.Fatal("retry did not succeed")
	}
}

func TestAISetupOutputParsesSplitProgressAndDevice(t *testing.T) {
	var status AISetupStatus
	output := &setupOutput{phase: "model", update: func(value AISetupStatus) { status = value }}
	output.Write([]byte(`{"type":"progress","completed":10,`))
	output.Write([]byte("\"total\":20,\"unit\":\"bytes\",\"detail\":\"weights\"}\n"))
	if status.Completed != 10 || status.Total != 20 || status.Phase != "model" {
		t.Fatalf("progress = %#v", status)
	}
	output.Write([]byte("{\"type\":\"verified\",\"device\":\"cuda\"}\n"))
	if output.report.Device != "cuda" || !output.report.Verified {
		t.Fatalf("report = %#v", output.report)
	}
}

func TestAISetupEnvironmentDoesNotPassGeminiKeyOrInheritedUVConfig(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-secret")
	t.Setenv("UV_DEFAULT_INDEX", "https://example.invalid")
	t.Setenv("loglevel", "Info")
	env := strings.Join(setupEnvironment(t.TempDir()), "\n")
	if strings.Contains(env, "test-secret") || strings.Contains(env, "example.invalid") || strings.Contains(env, "loglevel=Info") {
		t.Fatal("setup inherited secret/config")
	}
}

func TestBootstrapDownloadChecksHashAndPreservesDestinationOnFailure(t *testing.T) {
	content := []byte("test bootstrap archive")
	hash := sha256.Sum256(content)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(content) }))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "archive.zip")
	progress := int64(0)
	if err := downloadVerified(context.Background(), server.Client(), server.URL, hex.EncodeToString(hash[:]), destination, func(done, total int64) { progress = done }); err != nil {
		t.Fatal(err)
	}
	if progress != int64(len(content)) {
		t.Fatal("progress not completed")
	}
	if err := downloadVerified(context.Background(), server.Client(), server.URL, strings.Repeat("0", 64), destination, func(int64, int64) {}); err == nil {
		t.Fatal("bad checksum accepted")
	}
	if data, _ := os.ReadFile(destination); string(data) != string(content) {
		t.Fatal("valid file replaced after checksum failure")
	}
}

func TestExtractBootstrapRejectsTraversalAndExtractsOnlyExpectedExecutable(t *testing.T) {
	for _, name := range []string{"../uv.exe", "bundle/uv.exe"} {
		t.Run(name, func(t *testing.T) {
			folder := t.TempDir()
			archive := filepath.Join(folder, "uv.zip")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			entry, _ := writer.Create(name)
			entry.Write([]byte("fake binary"))
			writer.Close()
			file.Close()
			destination := filepath.Join(folder, "uv.exe")
			err = extractUV(archive, destination, "uv.exe")
			if name == "../uv.exe" && err == nil {
				t.Fatal("accepted traversal")
			}
			if name == "bundle/uv.exe" && err != nil {
				t.Fatal(err)
			}
		})
	}
}

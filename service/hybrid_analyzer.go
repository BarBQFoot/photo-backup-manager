package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	aidocs "photo-backup-manager/docs"
	aitools "photo-backup-manager/tools"
)

type AIInput struct {
	FileID string `json:"file_id"`
	Path   string `json:"path"`
}

type AIEvent struct {
	Type      string `json:"type"`
	Phase     string `json:"phase,omitempty"`
	Completed int    `json:"completed,omitempty"`
	Total     int    `json:"total,omitempty"`
	FileID    string `json:"file_id,omitempty"`
	Caption   string `json:"caption,omitempty"`
	Message   string `json:"message,omitempty"`
}

type BatchAIAnalyzer interface {
	AnalyzeBatch(context.Context, []AIInput, func(AIEvent)) error
}

type HybridAnalyzer struct{}

func NewHybridAnalyzer() *HybridAnalyzer { return &HybridAnalyzer{} }

func (a *HybridAnalyzer) Analyze(ctx context.Context, path, _ string) (string, error) {
	var caption string
	err := a.AnalyzeBatch(ctx, []AIInput{{FileID: "file_1", Path: path}}, func(event AIEvent) {
		if event.Type == "result" {
			caption = event.Caption
		}
	})
	if err != nil {
		return "", err
	}
	if caption == "" {
		return "", errors.New("local caption failed; check Python, model cache, and image format")
	}
	return caption, nil
}

func aiCacheDir() (string, error) {
	if path := strings.TrimSpace(os.Getenv("PHOTO_AI_CACHE")); path != "" {
		return filepath.Abs(path)
	}
	path, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(path, "photo-backup-manager", "ai"), nil
}

func aiPython() (string, error) {
	if path := strings.TrimSpace(os.Getenv("PHOTO_AI_PYTHON")); path != "" {
		return exec.LookPath(path)
	}
	if cache, err := aiCacheDir(); err == nil {
		if _, err = os.Stat(filepath.Join(cache, "python-env-ready.json")); err == nil {
			if info, err := os.Stat(managedAIPython(cache)); err == nil && !info.IsDir() {
				return managedAIPython(cache), nil
			}
		}
		if data, err := os.ReadFile(filepath.Join(cache, "existing-python.json")); err == nil {
			var saved existingPythonRecord
			if json.Unmarshal(data, &saved) == nil && filepath.IsAbs(saved.Python) {
				if info, err := os.Stat(saved.Python); err == nil && !info.IsDir() {
					return saved.Python, nil
				}
			}
		}
	}
	cwd, _ := os.Getwd()
	executable, _ := os.Executable()
	roots := aiPythonSearchRoots(cwd, executable, runtime.GOOS)
	for _, root := range roots {
		path := filepath.Join(root, ".venv", "bin", "python")
		if runtime.GOOS == "windows" {
			path = filepath.Join(root, ".venv", "Scripts", "python.exe")
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return filepath.Abs(path)
		}
	}
	return "", &AIError{Code: "AI_SETUP_REQUIRED", Err: errors.New("set PHOTO_AI_PYTHON to the Python executable with PyTorch, Transformers and the cached Florence model")}
}

func aiPythonSearchRoots(cwd, executable, platform string) []string {
	directory := filepath.Dir(executable)
	roots := []string{cwd, directory, filepath.Join(directory, "..", "..")}
	if platform == "darwin" && filepath.Base(directory) == "MacOS" && filepath.Base(filepath.Dir(directory)) == "Contents" {
		bundle := filepath.Dir(filepath.Dir(directory))
		if strings.HasSuffix(strings.ToLower(bundle), ".app") {
			adjacent := filepath.Dir(bundle)
			roots = append(roots, adjacent)
			project := filepath.Clean(filepath.Join(adjacent, "..", ".."))
			if _, err := os.Stat(filepath.Join(project, "go.mod")); err == nil {
				roots = append(roots, project)
			}
		}
	}
	return roots
}

func aiWorkerEnvironment(base []string) []string {
	env := make([]string, 0, len(base)+2)
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		// Wails dev exports loglevel=Info. On Windows PyTorch reads that as
		// LOGLEVEL and rejects the mixed-case value. Keep the parent unchanged.
		if strings.EqualFold(name, "loglevel") || strings.EqualFold(name, "PYTHONUTF8") || strings.EqualFold(name, "PYTHONIOENCODING") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "PYTHONUTF8=1", "PYTHONIOENCODING=utf-8")
}

// Extract embedded resources into a versioned folder, independent of the working directory.
func aiWorkerResources(cache string) (string, error) {
	resources := map[string][]byte{"docs/GEMINI-TRANSLATE-CAPTIONS.txt": []byte(aidocs.TranslationPrompt)}
	hash := sha256.New()
	entries, err := fs.ReadDir(aitools.Files, ".")
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		data, err := aitools.Files.ReadFile(entry.Name())
		if err != nil {
			return "", err
		}
		resources["tools/"+entry.Name()] = data
		hash.Write([]byte(entry.Name()))
		hash.Write(data)
	}
	hash.Write([]byte(aidocs.TranslationPrompt))
	root := filepath.Join(cache, "runtime_"+hex.EncodeToString(hash.Sum(nil))[:20])
	for name, data := range resources {
		path := filepath.Join(root, filepath.FromSlash(name))
		if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return "", err
		}
		// A unique temp file + rename prevents partially written scripts between app instances.
		tmp, err := os.CreateTemp(filepath.Dir(path), ".worker-*")
		if err != nil {
			return "", err
		}
		tmpName := tmp.Name()
		_, writeErr := tmp.Write(data)
		closeErr := tmp.Close()
		if writeErr == nil {
			writeErr = closeErr
		}
		if writeErr == nil {
			writeErr = os.Rename(tmpName, path)
		}
		_ = os.Remove(tmpName)
		if writeErr != nil {
			return "", writeErr
		}
	}
	return root, nil
}

func (a *HybridAnalyzer) AnalyzeBatch(ctx context.Context, items []AIInput, onEvent func(AIEvent)) error {
	python, err := aiPython()
	if err != nil {
		return err
	}
	cache, err := aiCacheDir()
	if err != nil {
		return err
	}
	root, err := aiWorkerResources(cache)
	if err != nil {
		return fmt.Errorf("prepare local AI worker: %w", err)
	}
	model := strings.TrimSpace(os.Getenv("GEMINI_TRANSLATION_MODEL"))
	if model == "" {
		model = "gemini-3.5-flash-lite"
	}
	payload, err := json.Marshal(struct {
		Items []AIInput `json:"items"`
		Cache string    `json:"cache_dir"`
		Model string    `json:"model"`
	}{items, filepath.Join(cache, "captions"), model})
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, python, "-u", filepath.Join(root, "tools", "app_caption_worker.py"))
	configureAICommand(cmd)
	cmd.Dir = root
	cmd.Env = aiWorkerEnvironment(os.Environ())
	cmd.Stdin = bytes.NewReader(payload)
	// Model diagnostics can contain arbitrary provider text. Do not include them in UI errors.
	cmd.Stderr = nil
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start local AI worker: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), 2<<20)
	var workerErr error
	done := false
	for scanner.Scan() {
		var event AIEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			workerErr = errors.New("invalid local AI worker response")
			_ = cmd.Process.Kill()
			break
		}
		switch event.Type {
		case "error":
			workerErr = errors.New(event.Message)
		case "done":
			done = true
		case "progress", "result", "failure":
			onEvent(event)
		default:
			workerErr = errors.New("unknown local AI worker event")
		}
	}
	if err := scanner.Err(); err != nil {
		workerErr = errors.New("could not read local AI worker response")
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if workerErr != nil {
		return workerErr
	}
	if waitErr != nil || !done {
		return errors.New("local AI worker stopped; check Python dependencies and cached Florence model")
	}
	return nil
}

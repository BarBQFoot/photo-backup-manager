package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFileMovesFileWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.jpg")
	destinationDir := filepath.Join(root, "destination")
	destination := filepath.Join(destinationDir, "source.jpg")
	if err := os.Mkdir(destinationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("photo fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	status, err := MoveFile(source, destination)
	if err != nil {
		t.Fatalf("MoveFile() error = %v", err)
	}
	if status != MoveStatusMoved {
		t.Fatalf("status = %q, want %q", status, MoveStatusMoved)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists or returned unexpected error: %v", err)
	}
	if content, err := os.ReadFile(destination); err != nil || string(content) != "photo fixture" {
		t.Fatalf("destination content = %q, error = %v", content, err)
	}
}

func TestMoveFileSkipsDuplicateAndKeepsSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.jpg")
	destination := filepath.Join(root, "source.jpg.copy")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}

	status, err := MoveFile(source, destination)
	if err != nil {
		t.Fatalf("MoveFile() error = %v", err)
	}
	if status != MoveStatusSkipped {
		t.Fatalf("status = %q, want %q", status, MoveStatusSkipped)
	}
	if content, err := os.ReadFile(source); err != nil || string(content) != "source" {
		t.Fatalf("source content = %q, error = %v", content, err)
	}
	if content, err := os.ReadFile(destination); err != nil || string(content) != "existing" {
		t.Fatalf("destination content = %q, error = %v", content, err)
	}
}

func TestMoveFileRejectsMissingSource(t *testing.T) {
	root := t.TempDir()
	_, err := MoveFile(filepath.Join(root, "missing.jpg"), filepath.Join(root, "destination.jpg"))
	var operationErr *FileOperationError
	if !errors.As(err, &operationErr) || operationErr.Code != "INVALID_SELECTION" {
		t.Fatalf("error = %v, want INVALID_SELECTION", err)
	}
}

func TestMoveFileUsesCopyFallbackWhenRenameFails(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	destinationDir := filepath.Join(root, "destination")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "source.jpg")
	destination := filepath.Join(destinationDir, "source.jpg")
	if err := os.WriteFile(source, []byte("copy fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalRenameSourceFile := renameSourceFile
	renameSourceFile = func(_, _ string) error {
		return errors.New("forced rename failure")
	}
	t.Cleanup(func() {
		renameSourceFile = originalRenameSourceFile
	})

	status, err := MoveFile(source, destination)
	if err != nil {
		t.Fatalf("MoveFile() error = %v", err)
	}
	if status != MoveStatusMoved {
		t.Fatalf("status = %q, want %q", status, MoveStatusMoved)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source still exists or returned unexpected error: %v", err)
	}
	if content, err := os.ReadFile(destination); err != nil || string(content) != "copy fixture" {
		t.Fatalf("destination content = %q, error = %v", content, err)
	}
}

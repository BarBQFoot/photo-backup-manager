package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFilePreservesSourceAndCreatesIndependentCopy(t *testing.T) {
	for _, content := range []string{"photo fixture", ""} {
		t.Run(content, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source.jpg")
			destination := filepath.Join(root, "copy.jpg")
			if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}

			status, err := CopyFile(source, destination)
			if err != nil || status != CopyStatusCopied {
				t.Fatalf("CopyFile() = %q, %v, want copied", status, err)
			}
			assertCopyContent(t, source, content)
			assertCopyContent(t, destination, content)
			// Editing the copy must not alter the original (for example via a hard link).
			if err := os.WriteFile(destination, []byte("edited copy"), 0o600); err != nil {
				t.Fatal(err)
			}
			assertCopyContent(t, source, content)
			assertNoCopyTemporaryFiles(t, root)
		})
	}
}

func TestCopyFileSkipsExistingDestination(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.jpg")
	destination := filepath.Join(root, "copy.jpg")
	if err := os.WriteFile(source, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{destination, source, root} {
		status, err := CopyFile(source, target)
		if err != nil || status != CopyStatusSkipped {
			t.Fatalf("CopyFile(%q) = %q, %v, want skipped", target, status, err)
		}
	}
	assertCopyContent(t, source, "original")
	assertCopyContent(t, destination, "existing")
}

func TestCopyFileRejectsInvalidSource(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "copy.jpg")
	for _, source := range []string{filepath.Join(root, "missing.jpg"), root} {
		status, err := CopyFile(source, destination)
		var operationErr *FileOperationError
		if status != "" || !errors.As(err, &operationErr) || operationErr.Code != "INVALID_SELECTION" {
			t.Fatalf("CopyFile(%q) = %q, %v, want INVALID_SELECTION", source, status, err)
		}
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination should not exist: %v", err)
	}
}

func TestCopyFileUnavailableDestinationPreservesSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.jpg")
	if err := os.WriteFile(source, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := CopyFile(source, filepath.Join(root, "missing", "copy.jpg"))
	var operationErr *FileOperationError
	if status != "" || !errors.As(err, &operationErr) || operationErr.Code != "DESTINATION_UNAVAILABLE" {
		t.Fatalf("CopyFile() = %q, %v, want DESTINATION_UNAVAILABLE", status, err)
	}
	assertCopyContent(t, source, "original")
}

func TestCopyFileFinalizeFailureCleansTemporaryFileAndPreservesSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.jpg")
	destination := filepath.Join(root, "copy.jpg")
	if err := os.WriteFile(source, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalFinalize := finalizeCopiedFile
	finalizeCopiedFile = func(_, _ string) error { return errors.New("forced finalize failure") }
	t.Cleanup(func() { finalizeCopiedFile = originalFinalize })

	status, err := CopyFile(source, destination)
	var operationErr *FileOperationError
	if status != "" || !errors.As(err, &operationErr) || operationErr.Code != "DESTINATION_UNAVAILABLE" {
		t.Fatalf("CopyFile() = %q, %v, want DESTINATION_UNAVAILABLE", status, err)
	}
	assertCopyContent(t, source, "original")
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("destination should not exist: %v", err)
	}
	assertNoCopyTemporaryFiles(t, root)
}

func assertCopyContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil || string(content) != want {
		t.Fatalf("ReadFile(%q) = %q, %v, want %q", path, content, err, want)
	}
}

func assertNoCopyTemporaryFiles(t *testing.T, root string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, ".photo-backup-*"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("unexpected temporary files: %v, error: %v", paths, err)
	}
}

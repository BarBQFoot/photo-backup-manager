package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDeleteFileAtPathDeletesFileInsideDestination(t *testing.T) {
	destination := t.TempDir()
	filePath := filepath.Join(destination, "photo.jpg")
	if err := os.WriteFile(filePath, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	deletedAt, err := DeleteFileAtPath(destination, filePath)
	if err != nil {
		t.Fatalf("DeleteFileAtPath() error = %v", err)
	}
	if deletedAt.IsZero() {
		t.Fatal("DeleteFileAtPath() returned zero deletion time")
	}
	if _, err := os.Stat(filePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file still exists or returned unexpected error: %v", err)
	}
}

func TestDeleteFileAtPathRejectsFileOutsideDestination(t *testing.T) {
	destination := t.TempDir()
	outside := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(outside, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := DeleteFileAtPath(destination, outside)
	var operationErr *FileOperationError
	if !errors.As(err, &operationErr) || operationErr.Code != "PATH_SCOPE_MISMATCH" {
		t.Fatalf("error = %v, want PATH_SCOPE_MISMATCH", err)
	}
}

func TestDeleteFileAtPathRejectsMissingFile(t *testing.T) {
	destination := t.TempDir()
	_, err := DeleteFileAtPath(destination, filepath.Join(destination, "missing.jpg"))
	var operationErr *FileOperationError
	if !errors.As(err, &operationErr) || operationErr.Code != "FILE_NOT_FOUND" {
		t.Fatalf("error = %v, want FILE_NOT_FOUND", err)
	}
}

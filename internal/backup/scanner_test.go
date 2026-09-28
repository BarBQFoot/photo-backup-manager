package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanDriveReturnsImagesRecursively(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "photo.JPG"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "photo.heic"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("ignore"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := ScanDrive(root)
	if err != nil {
		t.Fatalf("ScanDrive() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ScanDrive() returned %d files, want 2", len(files))
	}
	for _, file := range files {
		if file.FileID != nil {
			t.Fatalf("FileID = %v, want nil for a new file", file.FileID)
		}
		if file.FileName == "" || file.Path == "" || file.SizeBytes <= 0 {
			t.Fatalf("incomplete file metadata: %#v", file)
		}
		if file.FileStatus != "available" || file.AIStatus != "unanalyzed" {
			t.Fatalf("unexpected statuses: %#v", file)
		}
		if _, err := time.Parse(time.RFC3339, file.ModifiedAt); err != nil {
			t.Fatalf("ModifiedAt = %q is not RFC3339: %v", file.ModifiedAt, err)
		}
		if filepath.Ext(file.Path) == ".heic" && file.MIMEType != "image/heic" {
			t.Fatalf("MIME type = %q, want image/heic", file.MIMEType)
		}
	}
}

func TestScanDriveRejectsFilePath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ScanDrive(file)
	var scanErr *ScanError
	if !errors.As(err, &scanErr) || scanErr.Code != "PATH_NOT_DIRECTORY" {
		t.Fatalf("error = %v, want PATH_NOT_DIRECTORY", err)
	}
}

func TestScanDriveRejectsInvalidPaths(t *testing.T) {
	tests := []struct {
		name string
		path string
		code string
	}{
		{name: "empty path", path: "", code: "INVALID_PATH"},
		{name: "missing path", path: filepath.Join(t.TempDir(), "missing"), code: "INVALID_PATH"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ScanDrive(test.path)
			assertScanErrorCode(t, err, test.code)
		})
	}
}

func TestScanDriveReturnsEmptyListForEmptyDirectory(t *testing.T) {
	files, err := ScanDrive(t.TempDir())
	if err != nil {
		t.Fatalf("ScanDrive() error = %v", err)
	}
	if files == nil {
		t.Fatal("ScanDrive() returned nil, want an empty list")
	}
	if len(files) != 0 {
		t.Fatalf("ScanDrive() returned %d files, want 0", len(files))
	}
}

func assertScanErrorCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	var scanErr *ScanError
	if !errors.As(err, &scanErr) {
		t.Fatalf("error = %v, want ScanError with code %q", err, wantCode)
	}
	if scanErr.Code != wantCode {
		t.Fatalf("error code = %q, want %q", scanErr.Code, wantCode)
	}
}

package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ทดสอบว่า Source และ Destination ที่เป็นโฟลเดอร์แยกกันสามารถใช้งานร่วมกันได้.
func TestValidateBackupPathsAcceptsSeparateDirectories(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ValidateBackupPaths(source, destination); err != nil {
		t.Fatalf("ValidateBackupPaths() error = %v", err)
	}
}

// ทดสอบกรณีที่ Source และ Destination ใช้งานไม่ได้เนื่องจาก Path ซ้ำ ซ้อน หรือไม่มีอยู่จริง.
func TestValidateBackupPathsRejectsInvalidCombinations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	nested := filepath.Join(source, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		destination string
		code        string
	}{
		{name: "same directory", destination: source, code: "SOURCE_EQUALS_DESTINATION"},
		{name: "nested directory", destination: nested, code: "DESTINATION_WITHIN_SOURCE"},
		{name: "missing destination", destination: filepath.Join(root, "missing"), code: "DESTINATION_UNAVAILABLE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateBackupPaths(source, test.destination)
			assertPathErrorCode(t, err, test.code)
		})
	}
}

// ทดสอบว่า Source ที่ไม่มีอยู่จริงต้องคืน INVALID_PATH.
func TestValidateBackupPathsRejectsInvalidSource(t *testing.T) {
	destination := t.TempDir()
	err := ValidateBackupPaths(filepath.Join(destination, "missing"), destination)
	assertPathErrorCode(t, err, "INVALID_PATH")
}

// ทดสอบว่า Source ที่เป็นไฟล์ ไม่ใช่โฟลเดอร์ ต้องถูกปฏิเสธ.
func TestValidateBackupPathsRejectsFileAsSource(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "source.jpg")
	if err := os.WriteFile(file, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := ValidateBackupPaths(file, root)
	assertPathErrorCode(t, err, "INVALID_PATH")
}

func TestValidateSelectedFilesAcceptsFilesUnderSource(t *testing.T) {
	root := t.TempDir()
	photo := filepath.Join(root, "photo.jpg")
	if err := os.WriteFile(photo, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ValidateSelectedFiles(root, []string{photo}); err != nil {
		t.Fatalf("ValidateSelectedFiles() error = %v", err)
	}
}

func TestValidateSelectedFilesRejectsInvalidSelections(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.jpg")
	if err := os.WriteFile(outside, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		files []string
	}{
		{name: "empty selection", files: nil},
		{name: "outside source", files: []string{outside}},
		{name: "missing file", files: []string{filepath.Join(root, "missing.jpg")}},
		{name: "directory", files: []string{root}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateSelectedFiles(root, test.files)
			assertPathErrorCode(t, err, "INVALID_SELECTION")
		})
	}
}

// assertPathErrorCode ตรวจสอบว่า Error ที่ได้มีรหัสตรงกับที่คาดหวัง.
func assertPathErrorCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	var pathErr *PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error = %v, want PathError with code %q", err, wantCode)
	}
	if pathErr.Code != wantCode {
		t.Fatalf("error code = %q, want %q", pathErr.Code, wantCode)
	}
}

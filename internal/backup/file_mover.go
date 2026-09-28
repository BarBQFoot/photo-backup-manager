package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	MoveStatusMoved   = "moved"
	MoveStatusSkipped = "skipped"
)

var renameSourceFile = os.Rename
var finalizeCopiedFile = os.Rename

type FileOperationError struct {
	Code    string
	Message string
	Err     error
}

func (e *FileOperationError) Error() string {
	return e.Message
}

func (e *FileOperationError) Unwrap() error {
	return e.Err
}

// MoveFile ย้ายไฟล์ไปยังปลายทาง โดยไม่เขียนทับไฟล์เดิมและใช้ Copy เป็น Fallback.
func MoveFile(sourcePath, destinationPath string) (string, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return "", &FileOperationError{Code: "INVALID_SELECTION", Message: "source file is unavailable", Err: err}
	}
	if !sourceInfo.Mode().IsRegular() {
		return "", &FileOperationError{Code: "INVALID_SELECTION", Message: "source path is not a regular file"}
	}

	if _, err := os.Stat(destinationPath); err == nil {
		return MoveStatusSkipped, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", &FileOperationError{Code: "DESTINATION_UNAVAILABLE", Message: "destination file cannot be checked", Err: err}
	}

	if err := renameSourceFile(sourcePath, destinationPath); err == nil {
		return MoveStatusMoved, nil
	}
	if _, err := os.Stat(destinationPath); err == nil {
		return MoveStatusSkipped, nil
	}

	if err := copyAndVerify(sourcePath, destinationPath, sourceInfo.Size()); err != nil {
		var operationErr *FileOperationError
		if errors.As(err, &operationErr) && operationErr.Code == "DUPLICATE_FILE" {
			return MoveStatusSkipped, nil
		}
		return "", err
	}
	if err := os.Remove(sourcePath); err != nil {
		return "", &FileOperationError{Code: "SOURCE_DELETE_FAILED", Message: "source file could not be deleted after copy", Err: err}
	}
	return MoveStatusMoved, nil
}

func copyAndVerify(sourcePath, destinationPath string, expectedSize int64) error {
	destinationDir := filepath.Dir(destinationPath)
	temporaryFile, err := os.CreateTemp(destinationDir, ".photo-backup-*")
	if err != nil {
		return &FileOperationError{Code: "DESTINATION_UNAVAILABLE", Message: "temporary destination file could not be created", Err: err}
	}
	temporaryPath := temporaryFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(temporaryPath)
		}
	}()

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		_ = temporaryFile.Close()
		return &FileOperationError{Code: "INVALID_SELECTION", Message: "source file could not be opened", Err: err}
	}
	_, copyErr := io.Copy(temporaryFile, sourceFile)
	closeSourceErr := sourceFile.Close()
	if copyErr != nil {
		_ = temporaryFile.Close()
		return &FileOperationError{Code: "BACKUP_FAILED", Message: "source file could not be copied", Err: copyErr}
	}
	if closeSourceErr != nil {
		_ = temporaryFile.Close()
		return &FileOperationError{Code: "BACKUP_FAILED", Message: "source file could not be closed", Err: closeSourceErr}
	}
	if err := temporaryFile.Sync(); err != nil {
		_ = temporaryFile.Close()
		return &FileOperationError{Code: "BACKUP_FAILED", Message: "temporary destination file could not be flushed", Err: err}
	}
	if err := temporaryFile.Close(); err != nil {
		return &FileOperationError{Code: "BACKUP_FAILED", Message: "temporary destination file could not be closed", Err: err}
	}

	temporaryInfo, err := os.Stat(temporaryPath)
	if err != nil {
		return &FileOperationError{Code: "BACKUP_FAILED", Message: "temporary destination file could not be verified", Err: err}
	}
	if temporaryInfo.Size() != expectedSize {
		return &FileOperationError{Code: "BACKUP_FAILED", Message: fmt.Sprintf("copied file size mismatch: got %d, want %d", temporaryInfo.Size(), expectedSize)}
	}

	if err := finalizeCopiedFile(temporaryPath, destinationPath); err != nil {
		if _, statErr := os.Stat(destinationPath); statErr == nil {
			return &FileOperationError{Code: "DUPLICATE_FILE", Message: "destination file already exists", Err: err}
		}
		return &FileOperationError{Code: "DESTINATION_UNAVAILABLE", Message: "destination file could not be finalized", Err: err}
	}
	cleanup = false

	destinationInfo, err := os.Stat(destinationPath)
	if err != nil {
		return &FileOperationError{Code: "BACKUP_FAILED", Message: "destination file could not be verified", Err: err}
	}
	if destinationInfo.Size() != expectedSize {
		return &FileOperationError{Code: "BACKUP_FAILED", Message: fmt.Sprintf("destination file size mismatch: got %d, want %d", destinationInfo.Size(), expectedSize)}
	}
	return nil
}

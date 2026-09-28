package backup

import (
	"errors"
	"os"
	"time"
)

// DeleteFileAtPath ลบไฟล์จริงเมื่อไฟล์อยู่ภายใน Destination ที่ระบุเท่านั้น.
func DeleteFileAtPath(destinationRoot, filePath string) (time.Time, error) {
	if _, err := os.Stat(destinationRoot); err != nil {
		return time.Time{}, &FileOperationError{Code: "FILE_NOT_FOUND", Message: "destination path is unavailable", Err: err}
	}

	root, err := normalizedPath(destinationRoot)
	if err != nil {
		return time.Time{}, &FileOperationError{Code: "PATH_SCOPE_MISMATCH", Message: "destination path is invalid", Err: err}
	}
	target, err := normalizedPath(filePath)
	if err != nil || !isWithin(root, target) {
		return time.Time{}, &FileOperationError{Code: "PATH_SCOPE_MISMATCH", Message: "file is outside destination"}
	}

	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return time.Time{}, &FileOperationError{Code: "FILE_NOT_FOUND", Message: "file does not exist", Err: err}
		}
		return time.Time{}, &FileOperationError{Code: "DELETE_FAILED", Message: "file cannot be accessed", Err: err}
	}
	if !info.Mode().IsRegular() {
		return time.Time{}, &FileOperationError{Code: "DELETE_FAILED", Message: "path is not a regular file"}
	}
	if err := os.Remove(target); err != nil {
		return time.Time{}, &FileOperationError{Code: "DELETE_FAILED", Message: "file could not be deleted", Err: err}
	}
	return time.Now(), nil
}

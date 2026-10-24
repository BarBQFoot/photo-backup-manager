package backup

import (
	"errors"
	"os"
)

const (
	CopyStatusCopied  = "copied"
	CopyStatusSkipped = "skipped"
)

// CopyFile คัดลอกไฟล์ไปยังปลายทาง โดยเก็บต้นฉบับไว้และข้ามปลายทางที่มีอยู่แล้ว.
// destinationPath ต้องเป็นชื่อไฟล์เต็มในโฟลเดอร์ที่มีอยู่แล้ว เช่นเดียวกับ MoveFile.
// ผู้เรียกต้องตรวจขอบเขตโฟลเดอร์ด้วย ValidateBackupPaths/ValidateSelectedFiles ก่อน.
func CopyFile(sourcePath, destinationPath string) (string, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return "", &FileOperationError{Code: "INVALID_SELECTION", Message: "source file is unavailable", Err: err}
	}
	if !sourceInfo.Mode().IsRegular() {
		return "", &FileOperationError{Code: "INVALID_SELECTION", Message: "source path is not a regular file"}
	}

	if _, err := os.Lstat(destinationPath); err == nil {
		return CopyStatusSkipped, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", &FileOperationError{Code: "DESTINATION_UNAVAILABLE", Message: "destination file cannot be checked", Err: err}
	}

	if err := copyAndVerify(sourcePath, destinationPath, sourceInfo.Size()); err != nil {
		var operationErr *FileOperationError
		if errors.As(err, &operationErr) && operationErr.Code == "DUPLICATE_FILE" {
			return CopyStatusSkipped, nil
		}
		return "", err
	}
	return CopyStatusCopied, nil
}

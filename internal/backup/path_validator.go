package backup

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// PathError คือข้อผิดพลาดจากการตรวจสอบ Source และ Destination พร้อมรหัสสำหรับแปลงเป็น AppError.
type PathError struct {
	Code    string
	Message string
	Err     error
}

// Error คืนข้อความของข้อผิดพลาดสำหรับแสดงผลทั่วไป.
func (e *PathError) Error() string {
	return e.Message
}

// Unwrap คืนข้อผิดพลาดต้นเหตุเพื่อให้ errors.Is หรือ errors.As ตรวจสอบต่อได้.
func (e *PathError) Unwrap() error {
	return e.Err
}

// ValidateBackupPaths ตรวจสอบว่า Source และ Destination ใช้งานได้และไม่ทับซ้อนกัน.
func ValidateBackupPaths(sourcePath, destinationPath string) error {
	sourceInfo, err := validateDirectory(sourcePath, "INVALID_PATH", "source path is invalid")
	if err != nil {
		return err
	}

	destinationInfo, err := validateDirectory(destinationPath, "DESTINATION_UNAVAILABLE", "destination path is unavailable")
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() || !destinationInfo.IsDir() {
		return &PathError{Code: "INVALID_PATH", Message: "source and destination must be directories"}
	}

	source, err := normalizedPath(sourcePath)
	if err != nil {
		return &PathError{Code: "INVALID_PATH", Message: "source path is invalid", Err: err}
	}
	destination, err := normalizedPath(destinationPath)
	if err != nil {
		return &PathError{Code: "DESTINATION_UNAVAILABLE", Message: "destination path is unavailable", Err: err}
	}

	if pathsEqual(source, destination) {
		return &PathError{Code: "SOURCE_EQUALS_DESTINATION", Message: "source and destination must be different"}
	}
	if isWithin(source, destination) {
		return &PathError{Code: "DESTINATION_WITHIN_SOURCE", Message: "destination cannot be inside source"}
	}
	return nil
}

// ValidateSelectedFiles ตรวจสอบว่าไฟล์ที่เลือกมีอยู่จริง เป็นไฟล์ปกติ และอยู่ใต้ Source.
func ValidateSelectedFiles(sourcePath string, filePaths []string) error {
	if len(filePaths) == 0 {
		return &PathError{Code: "INVALID_SELECTION", Message: "at least one file must be selected"}
	}

	source, err := normalizedPath(sourcePath)
	if err != nil {
		return &PathError{Code: "INVALID_SELECTION", Message: "source path is invalid", Err: err}
	}
	for _, filePath := range filePaths {
		selected, pathErr := normalizedPath(filePath)
		if pathErr != nil || !isWithin(source, selected) {
			return &PathError{Code: "INVALID_SELECTION", Message: "selected file is outside source", Err: pathErr}
		}

		info, statErr := os.Stat(selected)
		if statErr != nil {
			return &PathError{Code: "INVALID_SELECTION", Message: "selected file is unavailable", Err: statErr}
		}
		if !info.Mode().IsRegular() {
			return &PathError{Code: "INVALID_SELECTION", Message: "selected path is not a regular file"}
		}
	}
	return nil
}

// validateDirectory ตรวจสอบว่า Path ไม่ว่าง มีอยู่จริง และเป็นโฟลเดอร์.
func validateDirectory(path, code, message string) (os.FileInfo, error) {
	if strings.TrimSpace(path) == "" {
		return nil, &PathError{Code: code, Message: message}
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, &PathError{Code: "ACCESS_DENIED", Message: "path cannot be accessed", Err: err}
		}
		return nil, &PathError{Code: code, Message: message, Err: err}
	}
	if !info.IsDir() {
		return nil, &PathError{Code: code, Message: message}
	}
	return info, nil
}

// normalizedPath แปลง Path ให้เป็น Absolute Path และตัดส่วนที่ไม่จำเป็นออก.
func normalizedPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// pathsEqual เปรียบเทียบ Path โดยไม่แยกตัวพิมพ์เล็กใหญ่บน Windows.
func pathsEqual(first, second string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(first, second)
	}
	return first == second
}

// isWithin ตรวจสอบว่า child อยู่ภายใน parent หรือไม่.
func isWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

package backup

import (
	"errors"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scanner ทำหน้าที่อ่านข้อมูลไฟล์จากระบบไฟล์ และยังไม่รวม Metadata Matching,
// Missing Detection หรือการเชื่อมต่อ Wails.

// DriveFile คือข้อมูลไฟล์ที่ได้จากการสแกนระบบไฟล์ตาม API Contract.
// ข้อมูลจาก Database เช่น Description และ File ID จะเติมภายหลังเมื่อเชื่อม Repository.
type DriveFile struct {
	FileID     *int64 `json:"fileId"`
	FileName   string `json:"fileName"`
	Path       string `json:"path"`
	SizeBytes  int64  `json:"sizeBytes"`
	MIMEType   string `json:"mimeType"`
	ModifiedAt string `json:"modifiedAt"`
	FileStatus string `json:"fileStatus"`
	AIStatus   string `json:"aiStatus"`
}

// ScanError คือข้อผิดพลาดจากการสแกนที่จะแปลงเป็น AppError ที่ Wails Boundary.
type ScanError struct {
	Code    string
	Message string
	Err     error
}

// Error คืนข้อความอธิบายสาเหตุของข้อผิดพลาดจากการสแกน.
func (e *ScanError) Error() string {
	return e.Message
}

// Unwrap คืนข้อผิดพลาดต้นเหตุเพื่อให้ errors.Is หรือ errors.As ตรวจสอบต่อได้.
func (e *ScanError) Unwrap() error {
	return e.Err
}

// ScanDrive สแกนโฟลเดอร์ root และโฟลเดอร์ย่อยแบบ Recursive แล้วคืนเฉพาะไฟล์รูป.
func ScanDrive(root string) ([]DriveFile, error) {
	if strings.TrimSpace(root) == "" {
		return nil, &ScanError{Code: "INVALID_PATH", Message: "path is required"}
	}

	rootInfo, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, &ScanError{Code: "ACCESS_DENIED", Message: "path cannot be accessed", Err: err}
		}
		return nil, &ScanError{Code: "INVALID_PATH", Message: "path does not exist", Err: err}
	}
	if !rootInfo.IsDir() {
		return nil, &ScanError{Code: "PATH_NOT_DIRECTORY", Message: "path is not a directory"}
	}

	files := make([]DriveFile, 0)
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrPermission) {
				return &ScanError{Code: "ACCESS_DENIED", Message: "path cannot be accessed", Err: walkErr}
			}
			return &ScanError{Code: "SCAN_FAILED", Message: "failed to scan path", Err: walkErr}
		}
		if entry.IsDir() {
			return nil
		}
		if !isImageFile(path) {
			return nil
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			if errors.Is(infoErr, os.ErrPermission) {
				return &ScanError{Code: "ACCESS_DENIED", Message: "file cannot be accessed", Err: infoErr}
			}
			return &ScanError{Code: "SCAN_FAILED", Message: "failed to read file metadata", Err: infoErr}
		}
		files = append(files, DriveFile{
			FileName:   info.Name(),
			Path:       path,
			SizeBytes:  info.Size(),
			MIMEType:   imageMIMEType(path),
			ModifiedAt: info.ModTime().Format(time.RFC3339),
			FileStatus: "available",
			AIStatus:   "unanalyzed",
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// isImageFile ตรวจสอบจาก MIME Type ของนามสกุลว่า Path เป็นไฟล์รูปหรือไม่.
func isImageFile(path string) bool {
	return strings.HasPrefix(imageMIMEType(path), "image/")
}

// imageMIMEType แปลงนามสกุลไฟล์เป็น MIME Type ที่ใช้ระบุชนิดไฟล์รูป.
func imageMIMEType(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	switch extension {
	case ".heic":
		return "image/heic"
	case ".heif":
		return "image/heif"
	case ".raw":
		return "image/x-raw"
	}
	if detected := mime.TypeByExtension(extension); strings.HasPrefix(detected, "image/") {
		return detected
	}
	return ""
}

package backup

import (
	"errors"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//Scanner implementation เสร็จแล้ว และ Unit Test ผ่าน 
//ยังทำส่วน Metadata Matching, Missing Detection และ Wails Integration ไม่ได้

// DriveFile is the filesystem portion of the API contract.
// Database fields are left empty until the metadata repository is connected.
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

// ScanError is converted to the shared AppError at the Wails boundary.
type ScanError struct {
	Code    string
	Message string
	Err     error
}

func (e *ScanError) Error() string {
	return e.Message
}

func (e *ScanError) Unwrap() error {
	return e.Err
}

// ScanDrive recursively scans root and returns image files found on disk.
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

func isImageFile(path string) bool {
	return strings.HasPrefix(imageMIMEType(path), "image/")
}

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

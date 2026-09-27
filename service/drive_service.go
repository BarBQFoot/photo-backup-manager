package service

import (
	"path/filepath"
	"sync"
	"time"

	"photo-backup-manager/internal/backup"
)

type DriveService struct {
	progressMu sync.RWMutex
	progress   BackupProgress
}

type AppError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	return e.Message
}

type BackupRequest struct {
	SourcePath      string   `json:"sourcePath"`
	DestinationPath string   `json:"destinationPath"`
	FilePaths       []string `json:"filePaths"`
}

type BackupItemResult struct {
	SourcePath      string    `json:"sourcePath"`
	DestinationPath string    `json:"destinationPath,omitempty"`
	Status          string    `json:"status"`
	Error           *AppError `json:"error,omitempty"`
}

type BackupResult struct {
	JobID        int64              `json:"jobId"`
	TotalFiles   int                `json:"totalFiles"`
	SuccessCount int                `json:"successCount"`
	SkippedCount int                `json:"skippedCount"`
	FailedCount  int                `json:"failedCount"`
	DurationMs   int64              `json:"durationMs"`
	Items        []BackupItemResult `json:"items"`
}

type BackupProgress struct {
	JobID       int64  `json:"jobId"`
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
	CurrentPath string `json:"currentPath,omitempty"`
	Status      string `json:"status"`
}

func NewDriveService() *DriveService {
	return &DriveService{progress: BackupProgress{Status: "idle"}}
}

func (s *DriveService) ScanDrive(path string) ([]backup.DriveFile, error) {
	return backup.ScanDrive(path)
}

// StartBackup ย้ายไฟล์ที่เลือกไปยัง Destination และสรุปผลการทำงานรายไฟล์.
func (s *DriveService) StartBackup(request BackupRequest) (BackupResult, error) {
	if err := backup.ValidateBackupPaths(request.SourcePath, request.DestinationPath); err != nil {
		return BackupResult{}, toAppError(err)
	}
	if err := backup.ValidateSelectedFiles(request.SourcePath, request.FilePaths); err != nil {
		return BackupResult{}, toAppError(err)
	}

	startedAt := time.Now()
	result := BackupResult{
		TotalFiles: len(request.FilePaths),
		Items:      make([]BackupItemResult, 0, len(request.FilePaths)),
	}
	s.setProgress(BackupProgress{Total: result.TotalFiles, Status: "moving"})
	for _, sourcePath := range request.FilePaths {
		destinationPath := filepath.Join(request.DestinationPath, filepath.Base(sourcePath))
		status, err := backup.MoveFile(sourcePath, destinationPath)
		item := BackupItemResult{
			SourcePath:      sourcePath,
			DestinationPath: destinationPath,
			Status:          status,
		}
		if err != nil {
			result.FailedCount++
			item.Status = "failed"
			item.Error = toAppError(err)
		} else if status == backup.MoveStatusSkipped {
			result.SkippedCount++
		} else {
			result.SuccessCount++
		}
		result.Items = append(result.Items, item)
		s.setProgress(BackupProgress{
			Completed:   len(result.Items),
			Total:       result.TotalFiles,
			CurrentPath: sourcePath,
			Status:      "moving",
		})
	}
	result.DurationMs = time.Since(startedAt).Milliseconds()
	s.setProgress(BackupProgress{Completed: result.TotalFiles, Total: result.TotalFiles, Status: "completed"})
	return result, nil
}

// GetBackupProgress คืน Snapshot ความคืบหน้าปัจจุบันของการ Backup.
func (s *DriveService) GetBackupProgress() BackupProgress {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	return s.progress
}

func (s *DriveService) setProgress(progress BackupProgress) {
	s.progressMu.Lock()
	s.progress = progress
	s.progressMu.Unlock()
}

func toAppError(err error) *AppError {
	appError := &AppError{Code: "INTERNAL_ERROR", Message: err.Error()}
	switch typedError := err.(type) {
	case *backup.PathError:
		appError.Code = typedError.Code
		appError.Message = typedError.Message
	case *backup.FileOperationError:
		appError.Code = typedError.Code
		appError.Message = typedError.Message
	}
	return appError
}

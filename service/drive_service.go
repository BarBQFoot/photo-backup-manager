package service

import (
	"context"
	"errors"
	"mime"
	"os"
	"path/filepath"
	"sync"
	"time"

	"photo-backup-manager/internal/backup"
	dbmodel "photo-backup-manager/model/model"
	"photo-backup-manager/repository"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type DriveService struct {
	progressMu sync.RWMutex
	progress   BackupProgress
	fileStore  repository.FileRecordStore
	jobStore   repository.BackupJobStore
	wailsCtx   context.Context
	emitEvent  func(context.Context, string, ...interface{})
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

type FileMetadata struct {
	FileID      int64  `json:"fileId"`
	FileName    string `json:"fileName"`
	Path        string `json:"path"`
	SizeBytes   int64  `json:"sizeBytes"`
	MIMEType    string `json:"mimeType"`
	ModifiedAt  string `json:"modifiedAt"`
	AIStatus    string `json:"aiStatus"`
	Description string `json:"description,omitempty"`
	BackupJobID int64  `json:"backupJobId,omitempty"`
	Status      string `json:"status"`
}

type BackupJob struct {
	ID              int64  `json:"id"`
	SourcePath      string `json:"sourcePath"`
	DestinationPath string `json:"destinationPath"`
	StartedAt       string `json:"startedAt"`
	CompletedAt     string `json:"completedAt,omitempty"`
	TotalFiles      int    `json:"totalFiles"`
	SuccessCount    int    `json:"successCount"`
	FailedCount     int    `json:"failedCount"`
	SkippedCount    int    `json:"skippedCount"`
	Status          string `json:"status"`
	DurationMs      int64  `json:"durationMs"`
}

type DeleteResult struct {
	FileID    int64  `json:"fileId"`
	Status    string `json:"status"`
	DeletedAt string `json:"deletedAt"`
}

func NewDriveService() *DriveService {
	return &DriveService{
		progress:  BackupProgress{Status: "idle"},
		emitEvent: wailsRuntime.EventsEmit,
	}
}

func NewDriveServiceWithRepositories(fileStore repository.FileRecordStore, jobStore repository.BackupJobStore) *DriveService {
	service := NewDriveService()
	service.fileStore = fileStore
	service.jobStore = jobStore
	return service
}

func (s *DriveService) ScanDrive(path string) ([]backup.DriveFile, error) {
	files, err := backup.ScanDrive(path)
	if err != nil || s.fileStore == nil {
		return files, err
	}

	ctx := context.Background()
	seenPaths := make(map[string]bool, len(files))
	for index := range files {
		seenPaths[cleanPath(files[index].Path)] = true
		record, recordErr := s.fileStore.GetByPath(ctx, files[index].Path)
		if errors.Is(recordErr, gorm.ErrRecordNotFound) {
			continue
		}
		if recordErr != nil {
			return nil, &AppError{Code: "METADATA_READ_FAILED", Message: recordErr.Error()}
		}
		fileID := int64(record.ID)
		files[index].FileID = &fileID
		files[index].AIStatus = record.AiStatus
		files[index].Description = record.Description
	}

	// เพิ่มมาเพื่อแสดง Metadata ที่อยู่ใน Database แต่ไฟล์จริงหายจาก Destination.
	records, recordErr := s.fileStore.ListByDestinationPath(ctx, path)
	if recordErr != nil {
		return nil, &AppError{Code: "METADATA_READ_FAILED", Message: recordErr.Error()}
	}
	for _, record := range records {
		if seenPaths[cleanPath(record.Path)] {
			continue
		}
		fileID := int64(record.ID)
		files = append(files, backup.DriveFile{
			FileID: &fileID, FileName: record.FileName, Path: record.Path,
			SizeBytes: int64(record.SizeBytes), MIMEType: record.MimeType,
			ModifiedAt: record.ModifiedAt.Format(time.RFC3339), FileStatus: "missing",
			AIStatus: record.AiStatus, Description: record.Description,
		})
	}
	return files, nil
}

// SetWailsContext ตั้ง Context สำหรับส่ง Event ความคืบหน้าไปยัง Frontend.
func (s *DriveService) SetWailsContext(ctx context.Context) {
	s.progressMu.Lock()
	s.wailsCtx = ctx
	s.progressMu.Unlock()
}

// GetFileMetadata อ่าน Metadata ของไฟล์จาก Repository ด้วย File ID.
func (s *DriveService) GetFileMetadata(fileID int64) (FileMetadata, error) {
	if s.fileStore == nil {
		return FileMetadata{}, &AppError{Code: "METADATA_READ_FAILED", Message: "file repository is unavailable"}
	}
	record, err := s.fileStore.GetByID(context.Background(), int32(fileID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FileMetadata{}, &AppError{Code: "FILE_NOT_FOUND", Message: "file metadata was not found"}
		}
		return FileMetadata{}, &AppError{Code: "METADATA_READ_FAILED", Message: err.Error()}
	}
	return FileMetadata{
		FileID: int64(record.ID), FileName: record.FileName, Path: record.Path,
		SizeBytes: int64(record.SizeBytes), MIMEType: record.MimeType,
		ModifiedAt: record.ModifiedAt.Format(time.RFC3339), AIStatus: record.AiStatus,
		Description: record.Description, BackupJobID: int64(record.BackupJobID), Status: record.Status,
	}, nil
}

// GetBackupHistory คืนประวัติ Backup เฉพาะ Destination ที่ระบุ.
func (s *DriveService) GetBackupHistory(destinationPath string) ([]BackupJob, error) {
	if s.jobStore == nil {
		return nil, &AppError{Code: "HISTORY_READ_FAILED", Message: "backup repository is unavailable"}
	}
	jobs, err := s.jobStore.ListByDestination(context.Background(), destinationPath)
	if err != nil {
		if destinationPath == "" {
			return nil, &AppError{Code: "DESTINATION_REQUIRED", Message: err.Error()}
		}
		return nil, &AppError{Code: "HISTORY_READ_FAILED", Message: err.Error()}
	}
	history := make([]BackupJob, 0, len(jobs))
	for _, job := range jobs {
		history = append(history, BackupJob{
			ID: int64(job.ID), SourcePath: job.Source, DestinationPath: job.Destination,
			StartedAt: job.StartedAt.Format(time.RFC3339), CompletedAt: formatOptionalTime(job.CompletedAt),
			TotalFiles: int(job.TotalFiles), SuccessCount: int(job.SuccessCount), FailedCount: int(job.FailedCount),
			SkippedCount: int(job.SkippedCount), Status: job.Status, DurationMs: int64(job.DurationMs),
		})
	}
	return history, nil
}

// DeleteFile ลบไฟล์จริงก่อน แล้วจึงเปลี่ยนสถานะ Record เป็น deleted.
func (s *DriveService) DeleteFile(fileID int64, destinationPath string) (DeleteResult, error) {
	if s.fileStore == nil {
		return DeleteResult{}, &AppError{Code: "METADATA_READ_FAILED", Message: "file repository is unavailable"}
	}
	record, err := s.fileStore.GetByID(context.Background(), int32(fileID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return DeleteResult{}, &AppError{Code: "FILE_NOT_FOUND", Message: "file metadata was not found"}
		}
		return DeleteResult{}, &AppError{Code: "METADATA_READ_FAILED", Message: err.Error()}
	}
	deletedAt, err := backup.DeleteFileAtPath(destinationPath, record.Path)
	if err != nil {
		return DeleteResult{}, toAppError(err)
	}
	if err := s.fileStore.UpdateStatus(context.Background(), record.ID, repository.FileRecordStatusDeleted); err != nil {
		return DeleteResult{FileID: fileID, Status: "deleted", DeletedAt: deletedAt.Format(time.RFC3339)}, &AppError{Code: "DATABASE_UPDATE_FAILED", Message: err.Error()}
	}
	return DeleteResult{FileID: fileID, Status: "deleted", DeletedAt: deletedAt.Format(time.RFC3339)}, nil
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
	var job *dbmodel.BackupJob
	if s.jobStore != nil {
		job = &dbmodel.BackupJob{
			Source:      request.SourcePath,
			Destination: request.DestinationPath,
			StartedAt:   startedAt,
			TotalFiles:  int32(result.TotalFiles),
			Status:      "running",
		}
		if err := s.jobStore.Create(context.Background(), job); err != nil {
			return BackupResult{}, &AppError{Code: "DATABASE_CREATE_FAILED", Message: err.Error()}
		}
		result.JobID = int64(job.ID)
	}
	s.setProgress(BackupProgress{Total: result.TotalFiles, Status: "moving"})
	for _, sourcePath := range request.FilePaths {
		destinationPath := filepath.Join(request.DestinationPath, filepath.Base(sourcePath))
		status, err := s.moveSelectedFile(sourcePath, destinationPath)
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
			if s.fileStore != nil {
				if saveErr := s.saveFileRecord(destinationPath, result.JobID); saveErr != nil {
					result.FailedCount++
					result.SuccessCount--
					item.Status = "failed"
					item.Error = saveErr
				}
			}
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
	if job != nil {
		job.CompletedAt = time.Now()
		job.SuccessCount = int32(result.SuccessCount)
		job.SkippedCount = int32(result.SkippedCount)
		job.FailedCount = int32(result.FailedCount)
		job.DurationMs = int32(result.DurationMs)
		job.Status = backupJobStatus(result)
		if err := s.jobStore.Finish(context.Background(), job); err != nil {
			return result, &AppError{Code: "DATABASE_UPDATE_FAILED", Message: err.Error()}
		}
	}
	return result, nil
}

// moveSelectedFile เพิ่มมาเพื่อข้าม Active Record แม้ไฟล์ปลายทางจริงจะหายไปแล้ว.
func (s *DriveService) moveSelectedFile(sourcePath, destinationPath string) (string, error) {
	if s.fileStore != nil {
		record, err := s.fileStore.GetByPath(context.Background(), destinationPath)
		if err == nil && record.Status == repository.FileRecordStatusActive {
			return backup.MoveStatusSkipped, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", &AppError{Code: "METADATA_READ_FAILED", Message: err.Error()}
		}
	}
	return backup.MoveFile(sourcePath, destinationPath)
}

func (s *DriveService) saveFileRecord(destinationPath string, jobID int64) *AppError {
	info, err := filepath.Abs(destinationPath)
	if err != nil {
		return &AppError{Code: "DATABASE_UPDATE_FAILED", Message: err.Error()}
	}
	fileInfo, err := os.Stat(info)
	if err != nil {
		return &AppError{Code: "DATABASE_UPDATE_FAILED", Message: err.Error()}
	}
	fileRecord := &dbmodel.FileRecord{
		FileName:    filepath.Base(info),
		Path:        info,
		SizeBytes:   int32(fileInfo.Size()),
		MimeType:    mimeTypeForPath(info),
		ModifiedAt:  fileInfo.ModTime(),
		BackupJobID: int32(jobID),
		AiStatus:    repository.AIStatusUnanalyzed,
		Status:      repository.FileRecordStatusActive,
	}
	if err := s.fileStore.SaveByPath(context.Background(), fileRecord); err != nil {
		return &AppError{Code: "DATABASE_UPDATE_FAILED", Message: err.Error()}
	}
	return nil
}

func backupJobStatus(result BackupResult) string {
	if result.FailedCount == 0 {
		return "completed"
	}
	if result.SuccessCount > 0 || result.SkippedCount > 0 {
		return "partial"
	}
	return "failed"
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
	ctx := s.wailsCtx
	s.progressMu.Unlock()
	if ctx != nil {
		s.emitEvent(ctx, "backup:progress", progress)
	}
}

func mimeTypeForPath(path string) string {
	extension := filepath.Ext(path)
	if extension == ".heic" {
		return "image/heic"
	}
	if extension == ".heif" {
		return "image/heif"
	}
	return mime.TypeByExtension(extension)
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func cleanPath(path string) string {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return filepath.Clean(path)
	}
	return absolute
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

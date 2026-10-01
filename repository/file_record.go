package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dbmodel "photo-backup-manager/model/model"
	"photo-backup-manager/model/query"

	"gorm.io/gorm"
)

const (
	FileRecordStatusActive  = "active"
	FileRecordStatusMissing = "missing"
	FileRecordStatusDeleted = "deleted"

	AIStatusUnanalyzed = "unanalyzed"
	AIStatusAnalyzing  = "analyzing"
	AIStatusAnalyzed   = "analyzed"
	AIStatusFailed     = "failed"
)

// FileRecordStore contains metadata operations; image bytes stay on the drive.
type FileRecordStore interface {
	SaveByPath(ctx context.Context, record *dbmodel.FileRecord) error
	GetByID(ctx context.Context, id int32) (*dbmodel.FileRecord, error)
	GetByPath(ctx context.Context, path string) (*dbmodel.FileRecord, error)
	MovePath(ctx context.Context, id int32, destinationPath string, jobID int32, sizeBytes int32, modifiedAt time.Time) error
	ListByDestinationPath(ctx context.Context, destination string) ([]*dbmodel.FileRecord, error)
	UpdateDescription(ctx context.Context, id int32, description, aiStatus string) error
	UpdateStatus(ctx context.Context, id int32, status string) error
	SearchByDescription(ctx context.Context, destination, keyword string) ([]*dbmodel.FileRecord, error)
}

// ListByDestinationPath เพิ่มมาเพื่อให้ Backup ตรวจ Missing Record ตาม Destination ได้.
func (r *fileRecordRepository) ListByDestinationPath(ctx context.Context, destination string) ([]*dbmodel.FileRecord, error) {
	if strings.TrimSpace(destination) == "" {
		return nil, errors.New("destination is required")
	}

	records, err := r.q.WithContext(ctx).FileRecord.Find()
	if err != nil {
		return nil, fmt.Errorf("list file records for %q: %w", destination, err)
	}
	matching := make([]*dbmodel.FileRecord, 0, len(records))
	for _, record := range records {
		if record.Status != FileRecordStatusDeleted && isPathWithinDestination(destination, record.Path) {
			matching = append(matching, record)
		}
	}
	return matching, nil
}

type fileRecordRepository struct {
	q *query.Query
}

func NewFileRecordRepository(db *gorm.DB) FileRecordStore {
	return &fileRecordRepository{q: query.Use(db)}
}

// SaveByPath creates a record for a new path or updates the existing record.
func (r *fileRecordRepository) SaveByPath(ctx context.Context, record *dbmodel.FileRecord) error {
	if record == nil || strings.TrimSpace(record.Path) == "" {
		return errors.New("file record and path are required")
	}

	return r.q.Transaction(func(tx *query.Query) error {
		fileQuery := tx.WithContext(ctx).FileRecord
		existing, err := fileQuery.Where(tx.FileRecord.Path.Eq(record.Path)).First()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if record.BackupJobID == 0 {
				fileQuery = fileQuery.Omit(tx.FileRecord.BackupJobID)
			}
			if err := fileQuery.Create(record); err != nil {
				return fmt.Errorf("create file record for %q: %w", record.Path, err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("find file record for %q: %w", record.Path, err)
		}

		record.ID = existing.ID
		if record.BackupJobID == 0 {
			record.BackupJobID = existing.BackupJobID
			fileQuery = fileQuery.Omit(tx.FileRecord.BackupJobID)
		}
		if err := fileQuery.Save(record); err != nil {
			return fmt.Errorf("update file record for %q: %w", record.Path, err)
		}
		return nil
	})
}

func (r *fileRecordRepository) GetByID(ctx context.Context, id int32) (*dbmodel.FileRecord, error) {
	if id <= 0 {
		return nil, errors.New("file record ID must be positive")
	}
	record, err := r.q.WithContext(ctx).FileRecord.
		Where(r.q.FileRecord.ID.Eq(id)).
		First()
	if err != nil {
		return nil, fmt.Errorf("get file record %d: %w", id, err)
	}
	return record, nil
}

func (r *fileRecordRepository) GetByPath(ctx context.Context, path string) (*dbmodel.FileRecord, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("file path is required")
	}
	record, err := r.q.WithContext(ctx).FileRecord.
		Where(r.q.FileRecord.Path.Eq(path)).
		First()
	if err != nil {
		return nil, fmt.Errorf("get file record for %q: %w", path, err)
	}
	return record, nil
}

// MovePath keeps a file's identity and metadata when the app moves it to another folder.
func (r *fileRecordRepository) MovePath(ctx context.Context, id int32, destinationPath string, jobID int32, sizeBytes int32, modifiedAt time.Time) error {
	if id <= 0 || strings.TrimSpace(destinationPath) == "" {
		return errors.New("file record ID and destination path are required")
	}
	result, err := r.q.WithContext(ctx).FileRecord.
		Where(r.q.FileRecord.ID.Eq(id)).
		Updates(map[string]interface{}{
			"path": destinationPath, "file_name": filepath.Base(destinationPath),
			"backup_job_id": jobID, "size_bytes": sizeBytes, "modified_at": modifiedAt,
			"status": FileRecordStatusActive,
		})
	if err != nil {
		return fmt.Errorf("move file record %d to %q: %w", id, destinationPath, err)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *fileRecordRepository) UpdateDescription(ctx context.Context, id int32, description, aiStatus string) error {
	if id <= 0 {
		return errors.New("file record ID must be positive")
	}
	if aiStatus != AIStatusUnanalyzed && aiStatus != AIStatusAnalyzing && aiStatus != AIStatusAnalyzed && aiStatus != AIStatusFailed {
		return fmt.Errorf("invalid AI status %q", aiStatus)
	}

	result, err := r.q.WithContext(ctx).FileRecord.
		Where(r.q.FileRecord.ID.Eq(id)).
		Updates(map[string]interface{}{"description": description, "ai_status": aiStatus})
	if err != nil {
		return fmt.Errorf("update description for file record %d: %w", id, err)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *fileRecordRepository) UpdateStatus(ctx context.Context, id int32, status string) error {
	if id <= 0 {
		return errors.New("file record ID must be positive")
	}
	if status != FileRecordStatusActive && status != FileRecordStatusMissing && status != FileRecordStatusDeleted {
		return fmt.Errorf("invalid file status %q", status)
	}

	result, err := r.q.WithContext(ctx).FileRecord.
		Where(r.q.FileRecord.ID.Eq(id)).
		Update(r.q.FileRecord.Status, status)
	if err != nil {
		return fmt.Errorf("update status for file record %d: %w", id, err)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SearchByDescription is scoped to one destination and excludes missing/deleted files.
func (r *fileRecordRepository) SearchByDescription(ctx context.Context, destination, keyword string) ([]*dbmodel.FileRecord, error) {
	if strings.TrimSpace(destination) == "" {
		return nil, errors.New("destination is required")
	}
	if strings.TrimSpace(keyword) == "" {
		return []*dbmodel.FileRecord{}, nil
	}

	file := r.q.FileRecord
	records, err := r.q.WithContext(ctx).FileRecord.
		Where(
			file.Description.Like("%"+keyword+"%"),
			file.Status.Eq(FileRecordStatusActive),
		).
		Find()
	if err != nil {
		return nil, fmt.Errorf("search descriptions in %q: %w", destination, err)
	}

	// Scope by the stored full path so records without a BackupJobID are searchable too.
	// Use path comparison in Go to avoid treating '%' or '_' in folder names as SQL wildcards.
	matching := make([]*dbmodel.FileRecord, 0, len(records))
	for _, record := range records {
		if isPathWithinDestination(destination, record.Path) {
			matching = append(matching, record)
		}
	}
	return matching, nil
}

func isPathWithinDestination(destination, filePath string) bool {
	destinationPath, err := filepath.Abs(filepath.Clean(destination))
	if err != nil {
		return false
	}
	fullPath, err := filepath.Abs(filepath.Clean(filePath))
	if err != nil {
		return false
	}
	relativePath, err := filepath.Rel(destinationPath, fullPath)
	if err != nil || relativePath == "." || relativePath == ".." {
		return false
	}
	return !strings.HasPrefix(relativePath, ".."+string(os.PathSeparator))
}

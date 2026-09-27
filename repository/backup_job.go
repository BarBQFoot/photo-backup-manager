package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dbmodel "photo-backup-manager/model/model"
	"photo-backup-manager/model/query"

	"gorm.io/gorm"
)

// BackupJobStore contains the database operations used for backup history.
type BackupJobStore interface {
	Create(ctx context.Context, job *dbmodel.BackupJob) error
	Finish(ctx context.Context, job *dbmodel.BackupJob) error
	ListByDestination(ctx context.Context, destination string) ([]*dbmodel.BackupJob, error)
}

type backupJobRepository struct {
	q *query.Query
}

func NewBackupJobRepository(db *gorm.DB) BackupJobStore {
	return &backupJobRepository{q: query.Use(db)}
}

// Create records a job when a backup starts.
func (r *backupJobRepository) Create(ctx context.Context, job *dbmodel.BackupJob) error {
	if job == nil {
		return errors.New("backup job is nil")
	}
	return r.q.WithContext(ctx).BackupJob.Create(job)
}

// Finish stores the final counts, status, completion time, and duration.
func (r *backupJobRepository) Finish(ctx context.Context, job *dbmodel.BackupJob) error {
	if job == nil || job.ID == 0 {
		return errors.New("backup job and a valid ID are required")
	}

	result, err := r.q.WithContext(ctx).BackupJob.
		Where(r.q.BackupJob.ID.Eq(job.ID)).
		Updates(map[string]interface{}{
			"completed_at":  job.CompletedAt,
			"total_files":   job.TotalFiles,
			"success_count": job.SuccessCount,
			"failed_count":  job.FailedCount,
			"skipped_count": job.SkippedCount,
			"duration_ms":   job.DurationMs,
			"status":        job.Status,
		})
	if err != nil {
		return fmt.Errorf("finish backup job %d: %w", job.ID, err)
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListByDestination returns only jobs for the requested destination, newest first.
func (r *backupJobRepository) ListByDestination(ctx context.Context, destination string) ([]*dbmodel.BackupJob, error) {
	if strings.TrimSpace(destination) == "" {
		return nil, errors.New("destination is required")
	}

	jobs, err := r.q.WithContext(ctx).BackupJob.
		Where(r.q.BackupJob.Destination.Eq(destination)).
		Order(r.q.BackupJob.StartedAt.Desc()).
		Find()
	if err != nil {
		return nil, fmt.Errorf("list backup history for %q: %w", destination, err)
	}
	return jobs, nil
}

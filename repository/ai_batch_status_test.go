package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
	dbmodel "photo-backup-manager/model/model"
)

func aiStatusFixture(tb testing.TB, count int) (*gorm.DB, FileRecordStore, []int32) {
	tb.Helper()
	tb.Setenv("PHOTO_BACKUP_DB", filepath.Join(tb.TempDir(), "status.db"))
	db, err := NewDbConnection()
	if err != nil {
		tb.Fatal(err)
	}
	sqlDB, _ := db.DB()
	tb.Cleanup(func() { _ = sqlDB.Close() })
	records := make([]dbmodel.FileRecord, count)
	for index := range records {
		records[index] = dbmodel.FileRecord{FileName: fmt.Sprintf("%d.jpg", index), Path: fmt.Sprintf("fixture/%d.jpg", index),
			Description: "original caption", SizeBytes: 10, ModifiedAt: time.Now(), Status: FileRecordStatusActive, AiStatus: AIStatusUnanalyzed}
	}
	if err := db.Omit("BackupJobID").CreateInBatches(&records, 100).Error; err != nil {
		tb.Fatal(err)
	}
	ids := make([]int32, count)
	for index := range records {
		ids[index] = records[index].ID
	}
	return db, NewFileRecordRepository(db), ids
}

func TestMarkAnalyzingScopesActiveRowsAndPreservesCaptions(t *testing.T) {
	_, files, ids := aiStatusFixture(t, 603)
	ctx := context.Background()
	if err := files.UpdateStatus(ctx, ids[601], FileRecordStatusMissing); err != nil {
		t.Fatal(err)
	}
	if err := files.UpdateStatus(ctx, ids[602], FileRecordStatusDeleted); err != nil {
		t.Fatal(err)
	}
	if err := files.UpdateDescription(ctx, ids[0], "user edited", AIStatusAnalyzed); err != nil {
		t.Fatal(err)
	}
	marked, err := files.(AIBatchStatusStore).MarkAnalyzing(ctx, append(ids, 999999))
	if err != nil || len(marked) != 601 {
		t.Fatalf("marked %d: %v", len(marked), err)
	}
	for index, id := range ids {
		record, err := files.GetByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		want := AIStatusAnalyzing
		if index >= 601 {
			want = AIStatusUnanalyzed
		}
		if record.AiStatus != want {
			t.Fatalf("row %d: %s", index, record.AiStatus)
		}
		caption := "original caption"
		if index == 0 {
			caption = "user edited"
		}
		if record.Description != caption || record.SizeBytes != 10 {
			t.Fatalf("metadata overwritten: %#v", record)
		}
	}
}

func TestMarkAnalyzingRollsBackEarlierChunksOnFailure(t *testing.T) {
	db, files, ids := aiStatusFixture(t, 503)
	query := fmt.Sprintf("CREATE TRIGGER reject_ai_mark BEFORE UPDATE OF ai_status ON file_records WHEN NEW.id = %d BEGIN SELECT RAISE(ABORT, 'test failure'); END", ids[501])
	if err := db.Exec(query).Error; err != nil {
		t.Fatal(err)
	}
	marked, err := files.(AIBatchStatusStore).MarkAnalyzing(context.Background(), ids)
	if err == nil || marked != nil {
		t.Fatal("failed transaction returned accepted IDs")
	}
	var count int64
	db.Model(&dbmodel.FileRecord{}).Where("ai_status = ?", AIStatusAnalyzing).Count(&count)
	if count != 0 {
		t.Fatalf("partial transaction committed %d rows", count)
	}
}

func BenchmarkMarkAIStatus(b *testing.B) {
	for _, bulk := range []bool{false, true} {
		b.Run(fmt.Sprintf("bulk_%v", bulk), func(b *testing.B) {
			b.StopTimer()
			_, files, ids := aiStatusFixture(b, 1000)
			ctx := context.Background()
			b.StartTimer()
			for n := 0; n < b.N; n++ {
				if bulk {
					if _, err := files.(AIBatchStatusStore).MarkAnalyzing(ctx, ids); err != nil {
						b.Fatal(err)
					}
				} else {
					for _, id := range ids {
						if err := files.UpdateDescription(ctx, id, "original caption", AIStatusAnalyzing); err != nil {
							b.Fatal(err)
						}
					}
				}
			}
		})
	}
}

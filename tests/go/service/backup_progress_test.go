package service

import (
	"context"
	"testing"
	"time"
)

func TestBackupProgressCoalescesEventsButKeepsLatestSnapshot(t *testing.T) {
	s := NewDriveService()
	s.SetWailsContext(context.Background())
	var events []BackupProgress
	s.emitEvent = func(_ context.Context, name string, data ...interface{}) {
		if name != "backup:progress" {
			t.Fatalf("unexpected event %s", name)
		}
		events = append(events, data[0].(BackupProgress))
	}
	now := time.Now()
	s.setProgressAt(BackupProgress{Status: "moving", Total: 1000}, now)
	for i := 1; i <= 999; i++ {
		s.setProgressAt(BackupProgress{Status: "moving", Completed: i, Total: 1000}, now.Add(10*time.Millisecond))
	}
	if len(events) != 1 || s.GetBackupProgress().Completed != 999 {
		t.Fatal("progress snapshot or event coalescing failed")
	}
	s.setProgressAt(BackupProgress{Status: "moving", Completed: 999, Total: 1000}, now.Add(100*time.Millisecond))
	if len(events) != 2 {
		t.Fatal("periodic update was not emitted")
	}
	s.setProgressAt(BackupProgress{Status: "moving", Completed: 1000, Total: 1000}, now.Add(101*time.Millisecond))
	s.setProgressAt(BackupProgress{Status: "completed", Completed: 1000, Total: 1000}, now.Add(102*time.Millisecond))
	if len(events) != 4 || events[2].Completed != 1000 || events[3].Status != "completed" {
		t.Fatal("last file or completion event was lost")
	}
	s.setProgressAt(BackupProgress{Status: "moving", Total: 2}, now.Add(103*time.Millisecond))
	if len(events) != 5 {
		t.Fatal("new job start was lost")
	}
}

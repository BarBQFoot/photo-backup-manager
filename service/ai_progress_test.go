package service

import (
	"context"
	"testing"
	"time"
)

func TestAIProgressCoalescesCountersWithoutDroppingResultsOrPhases(t *testing.T) {
	s := NewDriveService()
	s.SetWailsContext(context.Background())
	var events []AIEvent
	s.emitEvent = func(_ context.Context, name string, values ...interface{}) {
		if name != "ai:progress" {
			t.Fatalf("unexpected event %s", name)
		}
		events = append(events, values[0].(AIEvent))
	}
	now := time.Now()
	s.emitAIProgressAt(context.Background(), AIEvent{Type: "progress", Phase: "captioning", Total: 1000}, now)
	for i := 1; i < 1000; i++ {
		s.emitAIProgressAt(context.Background(), AIEvent{Type: "progress", Phase: "captioning", Completed: i, Total: 1000}, now.Add(time.Millisecond))
	}
	if len(events) != 1 {
		t.Fatal("progress events were not coalesced")
	}
	s.emitAIProgressAt(context.Background(), AIEvent{Type: "progress", Phase: "captioning", Completed: 999, Total: 1000}, now.Add(100*time.Millisecond))
	s.emitAIProgressAt(context.Background(), AIEvent{Type: "progress", Phase: "translating", Total: 1000}, now.Add(101*time.Millisecond))
	for _, id := range []string{"file_1", "file_2"} {
		s.emitAIProgressAt(context.Background(), AIEvent{Type: "result", FileID: id, Caption: "saved"}, now.Add(102*time.Millisecond))
	}
	s.emitAIProgressAt(context.Background(), AIEvent{Type: "progress", Phase: "done", Total: 1000, Completed: 1000}, now.Add(103*time.Millisecond))
	if len(events) != 6 || events[2].Phase != "translating" || events[3].FileID != "file_1" || events[4].FileID != "file_2" || events[5].Phase != "done" {
		t.Fatalf("phase or result event lost: %#v", events)
	}
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestResidentAIHelper(t *testing.T) {
	if os.Getenv("PHOTO_TEST_RESIDENT") != "1" {
		return
	}
	encoder := json.NewEncoder(os.Stdout)
	_ = encoder.Encode(AIEvent{Type: "ready"})
	decoder := json.NewDecoder(os.Stdin)
	var request struct {
		Items []AIInput `json:"items"`
	}
	count := 0
	for decoder.Decode(&request) == nil {
		count++
		fid := request.Items[0].FileID
		if fid == "file_99" {
			os.Exit(3)
		}
		if fid == "file_88" {
			_ = encoder.Encode(AIEvent{Type: "progress", Phase: "waiting"})
			time.Sleep(time.Minute)
		}
		if fid == "file_77" {
			_ = encoder.Encode(AIEvent{Type: "error", Message: "test request failure"})
		} else {
			_ = encoder.Encode(AIEvent{Type: "result", FileID: fid, Caption: fmt.Sprintf("job_%d", count)})
		}
		_ = encoder.Encode(AIEvent{Type: "done"})
	}
	os.Exit(0)
}

func residentFixture(t *testing.T) *HybridAnalyzer {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PHOTO_AI_PYTHON", executable)
	t.Setenv("PHOTO_AI_CACHE", t.TempDir())
	t.Setenv("PHOTO_TEST_RESIDENT", "1")
	a := NewHybridAnalyzer()
	a.command = func(string, ...string) *exec.Cmd { return exec.Command(executable, "-test.run=^TestResidentAIHelper$") }
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestResidentWorkerReusesProcessAndDrainsFailedRequest(t *testing.T) {
	a := residentFixture(t)
	var captions []string
	cb := func(event AIEvent) {
		if event.Type == "result" {
			captions = append(captions, event.Caption)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := a.AnalyzeBatch(ctx, []AIInput{{FileID: "file_1"}}, cb); err != nil {
		t.Fatal(err)
	}
	pid := a.worker.cmd.Process.Pid
	cancel() // A completed request's cancellation must not kill the next job.
	if err := a.AnalyzeBatch(context.Background(), []AIInput{{FileID: "file_77"}}, cb); err == nil || !strings.Contains(err.Error(), "test request failure") {
		t.Fatal(err)
	}
	if err := a.AnalyzeBatch(context.Background(), []AIInput{{FileID: "file_2"}}, cb); err != nil {
		t.Fatal(err)
	}
	if a.worker.cmd.Process.Pid != pid || strings.Join(captions, ",") != "job_1,job_3" {
		t.Fatalf("process not reused: %v", captions)
	}
}

func TestResidentWorkerCrashDoesNotReplayAndNextCallRestarts(t *testing.T) {
	a := residentFixture(t)
	if err := a.AnalyzeBatch(context.Background(), []AIInput{{FileID: "file_99"}}, func(AIEvent) {}); err == nil {
		t.Fatal("crash was ignored")
	}
	if a.worker != nil {
		t.Fatal("dead process retained")
	}
	caption := ""
	if err := a.AnalyzeBatch(context.Background(), []AIInput{{FileID: "file_1"}}, func(e AIEvent) { caption = e.Caption }); err != nil {
		t.Fatal(err)
	}
	if caption != "job_1" {
		t.Fatal(caption)
	}
}

func TestResidentWorkerCancellationStopsAndRestarts(t *testing.T) {
	a := residentFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := a.AnalyzeBatch(ctx, []AIInput{{FileID: "file_88"}}, func(AIEvent) {}); err != context.DeadlineExceeded {
		t.Fatalf("cancellation: %v", err)
	}
	if err := a.AnalyzeBatch(context.Background(), []AIInput{{FileID: "file_1"}}, func(AIEvent) {}); err != nil {
		t.Fatal(err)
	}
}

func TestResidentWorkerCloseStopsActiveJobAndRejectsNewJobs(t *testing.T) {
	a := residentFixture(t)
	waiting := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- a.AnalyzeBatch(context.Background(), []AIInput{{FileID: "file_88"}}, func(e AIEvent) {
			if e.Phase == "waiting" {
				close(waiting)
			}
		})
	}()
	select {
	case <-waiting:
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not start")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("active job succeeded after close")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("worker remained alive")
	}
	if err := a.AnalyzeBatch(context.Background(), []AIInput{{FileID: "file_1"}}, func(AIEvent) {}); err == nil {
		t.Fatal("closed analyzer accepted a job")
	}
}

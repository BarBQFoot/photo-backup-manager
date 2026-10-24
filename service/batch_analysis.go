package service

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dbmodel "photo-backup-manager/model/model"
	"photo-backup-manager/repository"
)

type AIFailure struct {
	FileID  int64  `json:"fileId"`
	Message string `json:"message"`
}

type AIBatchResult struct {
	Results  []AIResult     `json:"results"`
	Failures []AIFailure    `json:"failures"`
	Timings  AIBatchTimings `json:"timings"`
}

type AIBatchTimings struct {
	PreparationMs   int64 `json:"preparationMs"`
	MarkStatusMs    int64 `json:"markStatusMs"`
	WorkerMs        int64 `json:"workerMs"`
	SaveResultsMs   int64 `json:"saveResultsMs"`
	TotalMs         int64 `json:"totalMs"`
	WorkerStartupMs int64 `json:"workerStartupMs"`
	WorkerReused    bool  `json:"workerReused"`
}

// AnalyzeImages keeps DB identities separate from filenames and saves each completed result.
func (s *DriveService) AnalyzeImages(fileIDs []int64) (out AIBatchResult, resultErr error) {
	started := time.Now()
	out = AIBatchResult{Results: []AIResult{}, Failures: []AIFailure{}}
	if s.fileStore == nil || s.aiAnalyzer == nil {
		return out, &AppError{Code: "AI_UNAVAILABLE", Message: "AI analyzer is unavailable"}
	}
	if !s.aiMu.TryLock() {
		return out, &AppError{Code: "AI_BUSY", Message: "another analysis is running"}
	}
	defer s.aiMu.Unlock()
	recordTimings := false
	var saveDuration time.Duration
	defer func() {
		out.Timings.TotalMs = time.Since(started).Milliseconds()
		out.Timings.SaveResultsMs = saveDuration.Milliseconds()
		if cache, err := aiCacheDir(); err == nil && recordTimings {
			_ = os.MkdirAll(cache, 0o700)
			_ = writeSetupMarker(filepath.Join(cache, "analysis_summary.json"), struct {
				Timings   AIBatchTimings `json:"timings"`
				Requested int            `json:"requested"`
				Completed int            `json:"completed"`
				Failed    int            `json:"failed"`
			}{out.Timings, len(fileIDs), len(out.Results), len(out.Failures)})
		}
	}()
	if len(fileIDs) == 0 || len(fileIDs) > 10000 {
		return out, &AppError{Code: "INVALID_REQUEST", Message: "select 1 to 10000 photos"}
	}
	ctx := context.Background()
	s.progressMu.RLock()
	if s.wailsCtx != nil {
		ctx = s.wailsCtx
	}
	s.progressMu.RUnlock()
	s.emitAIProgress(ctx, AIEvent{Type: "progress", Phase: "preparing", Detail: "กำลังตรวจข้อมูลรูปและเตรียมสถานะ", Total: len(fileIDs)})
	inputs := []AIInput{}
	records := map[string]dbmodel.FileRecord{}
	infos := map[string]os.FileInfo{}
	finished := map[string]bool{}
	seen := map[int64]bool{}
	// Reserve selected records atomically with respect to moves and deletes.
	s.fileMutationMu.Lock()
	for _, id := range fileIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if id < 1 || id > math.MaxInt32 {
			out.Failures = append(out.Failures, AIFailure{id, "invalid file ID"})
			continue
		}
		record, err := s.fileStore.GetByID(ctx, int32(id))
		if err != nil || record == nil || record.ID != int32(id) {
			out.Failures = append(out.Failures, AIFailure{id, "file metadata was not found"})
			continue
		}
		if record.Status != repository.FileRecordStatusActive {
			out.Failures = append(out.Failures, AIFailure{id, "image file is missing or unavailable"})
			continue
		}
		info, err := os.Stat(record.Path)
		if err != nil || !info.Mode().IsRegular() {
			out.Failures = append(out.Failures, AIFailure{id, "image file is missing or unavailable"})
			continue
		}
		fid := "file_" + strconv.FormatInt(id, 10)
		records[fid] = *record
		infos[fid] = info
		inputs = append(inputs, AIInput{FileID: fid, Path: record.Path})
	}
	markStarted := time.Now()
	marked := map[int32]bool{}
	if batchStore, ok := s.fileStore.(repository.AIBatchStatusStore); ok {
		ids := make([]int32, 0, len(inputs))
		for _, input := range inputs {
			ids = append(ids, records[input.FileID].ID)
		}
		if len(ids) > 0 {
			if accepted, err := batchStore.MarkAnalyzing(ctx, ids); err == nil {
				for _, id := range accepted {
					marked[id] = true
				}
			}
		}
	} else {
		for _, input := range inputs {
			record := records[input.FileID]
			if err := s.fileStore.UpdateDescription(ctx, record.ID, record.Description, repository.AIStatusAnalyzing); err == nil {
				marked[record.ID] = true
			}
		}
	}
	acceptedInputs := inputs[:0]
	for _, input := range inputs {
		if marked[records[input.FileID].ID] {
			acceptedInputs = append(acceptedInputs, input)
		} else {
			out.Failures = append(out.Failures, AIFailure{int64(records[input.FileID].ID), "could not mark image as analyzing"})
			delete(records, input.FileID)
		}
	}
	inputs = acceptedInputs
	s.fileMutationMu.Unlock()
	out.Timings.MarkStatusMs = time.Since(markStarted).Milliseconds()
	out.Timings.PreparationMs = time.Since(started).Milliseconds()
	fail := func(fid, message string) {
		record, ok := records[fid]
		if !ok || finished[fid] {
			return
		}
		finished[fid] = true
		// Retain any current user description and do not overwrite a concurrently moved/deleted record.
		if current, err := s.fileStore.GetByID(context.Background(), record.ID); err == nil && current.Path == record.Path && current.Status == repository.FileRecordStatusActive {
			_ = s.fileStore.UpdateDescription(context.Background(), record.ID, current.Description, repository.AIStatusFailed)
		}
		out.Failures = append(out.Failures, AIFailure{int64(record.ID), message})
	}
	protocolErr := false
	onEvent := func(event AIEvent) {
		if event.Type == "progress" {
			s.emitAIProgress(ctx, event)
			return
		}
		s.fileMutationMu.Lock()
		defer s.fileMutationMu.Unlock()
		saveStarted := time.Now()
		defer func() { saveDuration += time.Since(saveStarted) }()
		record, ok := records[event.FileID]
		if !ok || finished[event.FileID] {
			protocolErr = true
			return
		}
		if event.Type == "failure" {
			fail(event.FileID, event.Message)
			return
		}
		caption := strings.TrimSpace(event.Caption)
		if caption == "" {
			fail(event.FileID, "empty AI description")
			return
		}
		current, err := s.fileStore.GetByID(ctx, record.ID)
		info, statErr := os.Stat(record.Path)
		if err != nil || statErr != nil || current.Path != record.Path || current.Status != repository.FileRecordStatusActive ||
			info.Size() != infos[event.FileID].Size() || !info.ModTime().Equal(infos[event.FileID].ModTime()) {
			fail(event.FileID, "image changed during analysis; try again")
			return
		}
		if err := s.fileStore.UpdateDescription(ctx, record.ID, caption, repository.AIStatusAnalyzed); err != nil {
			fail(event.FileID, "could not save AI description; cached result is preserved")
			return
		}
		finished[event.FileID] = true
		out.Results = append(out.Results, AIResult{FileID: int64(record.ID), Description: caption, Status: "success"})
		s.emitAIProgress(ctx, event)
	}
	var workerErr error
	workerStarted := time.Now()
	if len(inputs) > 0 {
		_, recordTimings = s.aiAnalyzer.(*HybridAnalyzer)
		if analyzer, ok := s.aiAnalyzer.(BatchAIAnalyzer); ok {
			workerErr = analyzer.AnalyzeBatch(ctx, inputs, onEvent)
		} else {
			for _, input := range inputs {
				caption, err := s.aiAnalyzer.Analyze(ctx, input.Path, records[input.FileID].MimeType)
				if err != nil {
					workerErr = err
					fail(input.FileID, err.Error())
					continue
				}
				onEvent(AIEvent{Type: "result", FileID: input.FileID, Caption: caption})
			}
		}
	}
	out.Timings.WorkerMs = time.Since(workerStarted).Milliseconds()
	if hybrid, ok := s.aiAnalyzer.(*HybridAnalyzer); ok {
		stats := hybrid.lastWorkerStats()
		out.Timings.WorkerReused, out.Timings.WorkerStartupMs = stats.Reused, stats.StartupMs
	}
	if protocolErr && workerErr == nil {
		workerErr = errors.New("AI returned an unknown or duplicated file ID")
	}
	for _, input := range inputs {
		if !finished[input.FileID] {
			message := "AI did not return a caption"
			if workerErr != nil {
				message = workerErr.Error()
			}
			fail(input.FileID, message)
		}
	}
	s.emitAIProgress(ctx, AIEvent{Type: "progress", Phase: "done", Completed: len(out.Results), Total: len(seen)})
	return out, nil
}

func (s *DriveService) emitAIProgress(ctx context.Context, event AIEvent) {
	s.emitAIProgressAt(ctx, event, time.Now())
}

func (s *DriveService) emitAIProgressAt(ctx context.Context, event AIEvent, now time.Time) {
	s.progressMu.Lock()
	ready := s.wailsCtx != nil
	if ready && event.Type == "progress" {
		ready = s.aiProgressEmittedAt.IsZero() || event.Phase != s.aiProgressPhase ||
			now.Sub(s.aiProgressEmittedAt) >= 100*time.Millisecond ||
			(event.Total > 0 && event.Completed >= event.Total)
		if ready {
			s.aiProgressEmittedAt = now
			s.aiProgressPhase = event.Phase
		}
	}
	s.progressMu.Unlock()
	if ready && s.emitEvent != nil {
		s.emitEvent(ctx, "ai:progress", event)
	}
}

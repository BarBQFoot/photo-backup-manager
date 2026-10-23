package service

import (
	"context"
	"errors"
	"math"
	"os"
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
	Results  []AIResult  `json:"results"`
	Failures []AIFailure `json:"failures"`
}

// AnalyzeImages keeps DB identities separate from filenames and saves each completed result.
func (s *DriveService) AnalyzeImages(fileIDs []int64) (AIBatchResult, error) {
	out := AIBatchResult{Results: []AIResult{}, Failures: []AIFailure{}}
	if s.fileStore == nil || s.aiAnalyzer == nil {
		return out, &AppError{Code: "AI_UNAVAILABLE", Message: "AI analyzer is unavailable"}
	}
	if !s.aiMu.TryLock() {
		return out, &AppError{Code: "AI_BUSY", Message: "another analysis is running"}
	}
	defer s.aiMu.Unlock()
	if len(fileIDs) == 0 || len(fileIDs) > 10000 {
		return out, &AppError{Code: "INVALID_REQUEST", Message: "select 1 to 10000 photos"}
	}
	ctx := context.Background()
	s.progressMu.RLock()
	if s.wailsCtx != nil {
		ctx = s.wailsCtx
	}
	s.progressMu.RUnlock()
	inputs := []AIInput{}
	records := map[string]dbmodel.FileRecord{}
	infos := map[string]os.FileInfo{}
	finished := map[string]bool{}
	seen := map[int64]bool{}
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
		if err := s.fileStore.UpdateDescription(ctx, record.ID, record.Description, repository.AIStatusAnalyzing); err != nil {
			out.Failures = append(out.Failures, AIFailure{id, "could not mark image as analyzing"})
			continue
		}
		records[fid] = *record
		infos[fid] = info
		inputs = append(inputs, AIInput{FileID: fid, Path: record.Path})
	}
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
	if len(inputs) > 0 {
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

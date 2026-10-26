package service

import "context"

// AIAnalyzer is the shared contract used by DriveService. The application uses
// HybridAnalyzer: Florence captions images locally and Gemini translates text.
type AIAnalyzer interface {
	Analyze(ctx context.Context, path, mimeType string) (string, error)
}

// AIError carries a stable error code from an analyzer to the application layer.
type AIError struct {
	Code string
	Err  error
}

func (e *AIError) Error() string { return e.Err.Error() }
func (e *AIError) Unwrap() error { return e.Err }

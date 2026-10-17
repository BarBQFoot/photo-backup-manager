package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	geminiModel       = "gemini-2.5-flash"
	maxImageSizeBytes = 10 << 20
)

type AIAnalyzer interface {
	Analyze(ctx context.Context, path, mimeType string) (string, error)
}

type AIError struct {
	Code string
	Err  error
}

func (e *AIError) Error() string { return e.Err.Error() }
func (e *AIError) Unwrap() error { return e.Err }

type GeminiAnalyzer struct {
	apiKey     string
	httpClient *http.Client
	endpoint   string
}

func NewGeminiAnalyzerFromEnv() *GeminiAnalyzer {
	return &GeminiAnalyzer{
		apiKey:     strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		httpClient: &http.Client{Timeout: 60 * time.Second},
		endpoint:   "https://generativelanguage.googleapis.com/v1beta/models/" + geminiModel + ":generateContent",
	}
}

func (a *GeminiAnalyzer) Analyze(ctx context.Context, path, mimeType string) (string, error) {
	if a == nil || strings.TrimSpace(a.apiKey) == "" {
		return "", &AIError{Code: "AI_KEY_MISSING", Err: errors.New("Gemini API key is not configured")}
	}

	image, detectedMIME, err := readImage(path, mimeType)
	if err != nil {
		return "", err
	}

	requestBody := geminiRequest{
		Contents: []geminiContent{{
			Parts: []geminiPart{
				{Text: "อธิบายภาพนี้เป็นภาษาไทยด้วยประโยคสั้น กระชับ และเป็นธรรมชาติ ตอบเฉพาะคำอธิบายภาพเท่านั้น ห้ามใส่ Markdown หัวข้อ หรือป้ายกำกับ"},
				{InlineData: &geminiInlineData{
					MIMEType: detectedMIME,
					Data:     base64.StdEncoding.EncodeToString(image),
				}},
			},
		}},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", &AIError{Code: "AI_UNAVAILABLE", Err: fmt.Errorf("encode Gemini request: %w", err)}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint+"?key="+a.apiKey, bytes.NewReader(payload))
	if err != nil {
		return "", &AIError{Code: "AI_UNAVAILABLE", Err: fmt.Errorf("create Gemini request: %w", err)}
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := a.httpClient.Do(request)
	if err != nil {
		return "", &AIError{Code: "AI_UNAVAILABLE", Err: fmt.Errorf("Gemini request failed: %w", err)}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
		return "", &AIError{Code: "AI_UNAVAILABLE", Err: fmt.Errorf("Gemini returned HTTP %d", response.StatusCode)}
	}

	var result geminiResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", &AIError{Code: "AI_RESPONSE_INVALID", Err: errors.New("Gemini response was not valid JSON")}
	}
	description := strings.TrimSpace(result.firstText())
	if description == "" {
		return "", &AIError{Code: "AI_RESPONSE_INVALID", Err: errors.New("Gemini response did not contain a description")}
	}
	return description, nil
}

func readImage(path, declaredMIME string) ([]byte, string, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", &AIError{Code: "FILE_MISSING", Err: errors.New("image file is missing")}
		}
		return nil, "", &AIError{Code: "AI_UNAVAILABLE", Err: errors.New("image file could not be opened")}
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImageSizeBytes+1))
	if err != nil {
		return nil, "", &AIError{Code: "AI_UNAVAILABLE", Err: errors.New("image file could not be read")}
	}
	if len(data) > maxImageSizeBytes {
		return nil, "", &AIError{Code: "UNSUPPORTED_IMAGE", Err: errors.New("image exceeds the 10 MB limit")}
	}

	mimeType := strings.ToLower(strings.TrimSpace(declaredMIME))
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	switch mimeType {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		return data, mimeType, nil
	default:
		return nil, "", &AIError{Code: "UNSUPPORTED_IMAGE", Err: errors.New("image type is not supported")}
	}
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inline_data,omitempty"`
}

type geminiInlineData struct {
	MIMEType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (r geminiResponse) firstText() string {
	for _, candidate := range r.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				return part.Text
			}
		}
	}
	return ""
}

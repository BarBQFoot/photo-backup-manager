package service

import (
	"net/http"
	"os"

	"photo-backup-manager/internal/backup"
)

type photoPreview struct {
	info     os.FileInfo
	mimeType string
}

// Only files returned by a successful scan may be requested by the webview.
func (s *DriveService) registerPreviews(files []backup.DriveFile) {
	s.previewMu.Lock()
	defer s.previewMu.Unlock()
	if s.previews == nil {
		s.previews = make(map[string]photoPreview)
	}
	for _, file := range files {
		if file.FileStatus != "available" {
			continue
		}
		info, err := os.Stat(file.Path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		s.previews[cleanPath(file.Path)] = photoPreview{info: info, mimeType: file.MIMEType}
	}
}

// PhotoPreviewMiddleware runs before both embedded assets and the Nuxt dev proxy.
func PhotoPreviewMiddleware(s *DriveService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/photo-preview" {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			path := r.URL.Query().Get("path")
			s.previewMu.RLock()
			preview, ok := s.previews[cleanPath(path)]
			s.previewMu.RUnlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			file, err := os.Open(path)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil || !info.Mode().IsRegular() || !os.SameFile(preview.info, info) {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", preview.mimeType)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Security-Policy", "sandbox")
			w.Header().Set("Cache-Control", "no-store")
			http.ServeContent(w, r, info.Name(), info.ModTime(), file)
		})
	}
}

// Package uploads provides file upload handling for GoliveKit.
package uploads

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Common errors.
var (
	ErrFileTooLarge    = errors.New("file exceeds maximum size")
	ErrInvalidFileType = errors.New("invalid file type")
	ErrMaxFilesReached = errors.New("maximum number of files reached")
	ErrUploadCancelled = errors.New("upload cancelled")
	ErrUploadFailed    = errors.New("upload failed")
)

// UploadConfig configures upload behavior.
type UploadConfig struct {
	// Name identifies this upload configuration.
	Name string

	// Accept is a list of allowed MIME types.
	Accept []string

	// MaxFileSize is the maximum file size in bytes.
	MaxFileSize int64

	// MaxEntries is the maximum number of files.
	MaxEntries int

	// AutoUpload starts upload immediately when files are selected.
	AutoUpload bool

	// ChunkSize for chunked uploads.
	ChunkSize int64

	// External configures external storage (S3/GCS).
	External *ExternalUpload

	// TempDir is the directory for temporary files.
	TempDir string
}

// DefaultUploadConfig returns default upload configuration.
func DefaultUploadConfig() *UploadConfig {
	return &UploadConfig{
		Accept:      []string{"*/*"},
		MaxFileSize: 10 * 1024 * 1024, // 10MB
		MaxEntries:  5,
		AutoUpload:  false,
		ChunkSize:   1024 * 1024, // 1MB
		TempDir:     os.TempDir(),
	}
}

// UploadEntry represents a single file being uploaded.
type UploadEntry struct {
	// UUID is a unique identifier for this upload.
	UUID string `json:"uuid"`

	// FileName is the original file name.
	FileName string `json:"filename"`

	// Size is the file size in bytes.
	Size int64 `json:"size"`

	// ContentType is the MIME type.
	ContentType string `json:"content_type"`

	// Progress is the upload progress (0-100).
	Progress int `json:"progress"`

	// Done indicates if the upload is complete.
	Done bool `json:"done"`

	// URL is the URL where the file is accessible.
	URL string `json:"url,omitempty"`

	// Errors contains any upload errors.
	Errors []string `json:"errors,omitempty"`

	// TempPath is the temporary file path.
	TempPath string `json:"-"`

	// CreatedAt is when the upload started.
	CreatedAt time.Time `json:"created_at"`
}

// Upload manages file uploads for a session.
type Upload struct {
	Config  *UploadConfig
	Entries map[string]*UploadEntry
	mu      sync.RWMutex
}

// NewUpload creates a new upload manager.
func NewUpload(config *UploadConfig) *Upload {
	if config == nil {
		config = DefaultUploadConfig()
	}
	return &Upload{
		Config:  config,
		Entries: make(map[string]*UploadEntry),
	}
}

// AddEntry adds a new upload entry.
func (u *Upload) AddEntry(filename string, size int64, contentType string) (*UploadEntry, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	// Check max entries
	if len(u.Entries) >= u.Config.MaxEntries {
		return nil, ErrMaxFilesReached
	}

	// Check file size
	if size > u.Config.MaxFileSize {
		return nil, ErrFileTooLarge
	}

	// Check content type
	if !u.isAllowedType(contentType) {
		return nil, ErrInvalidFileType
	}

	entry := &UploadEntry{
		UUID:        generateUUID(),
		FileName:    sanitizeFilename(filename),
		Size:        size,
		ContentType: contentType,
		Progress:    0,
		Done:        false,
		CreatedAt:   time.Now(),
	}

	u.Entries[entry.UUID] = entry
	return entry, nil
}

// GetEntry retrieves an entry by UUID.
func (u *Upload) GetEntry(uuid string) (*UploadEntry, bool) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	entry, ok := u.Entries[uuid]
	return entry, ok
}

// RemoveEntry removes an entry.
func (u *Upload) RemoveEntry(uuid string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if entry, ok := u.Entries[uuid]; ok {
		// Clean up temp file
		if entry.TempPath != "" {
			os.Remove(entry.TempPath)
		}
		delete(u.Entries, uuid)
	}
}

// UpdateProgress updates the progress of an entry.
func (u *Upload) UpdateProgress(uuid string, progress int) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if entry, ok := u.Entries[uuid]; ok {
		entry.Progress = progress
		if progress >= 100 {
			entry.Done = true
		}
	}
}

// Complete marks an entry as complete.
func (u *Upload) Complete(uuid string, url string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if entry, ok := u.Entries[uuid]; ok {
		entry.Progress = 100
		entry.Done = true
		entry.URL = url
	}
}

// Error marks an entry as errored.
func (u *Upload) Error(uuid string, err string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if entry, ok := u.Entries[uuid]; ok {
		entry.Errors = append(entry.Errors, err)
	}
}

// AllComplete returns true if all entries are complete.
func (u *Upload) AllComplete() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, entry := range u.Entries {
		if !entry.Done {
			return false
		}
	}
	return true
}

// CompletedEntries returns all completed entries.
func (u *Upload) CompletedEntries() []*UploadEntry {
	u.mu.RLock()
	defer u.mu.RUnlock()

	var completed []*UploadEntry
	for _, entry := range u.Entries {
		if entry.Done && len(entry.Errors) == 0 {
			completed = append(completed, entry)
		}
	}
	return completed
}

func (u *Upload) isAllowedType(contentType string) bool {
	for _, allowed := range u.Config.Accept {
		if allowed == "*/*" {
			return true
		}
		if strings.HasSuffix(allowed, "/*") {
			prefix := strings.TrimSuffix(allowed, "/*")
			if strings.HasPrefix(contentType, prefix) {
				return true
			}
		}
		if allowed == contentType {
			return true
		}
	}
	return false
}

// ExternalUpload configures external storage.
type ExternalUpload struct {
	// Presigner generates presigned URLs for direct upload.
	Presigner func(entry *UploadEntry) (*PresignedURL, error)

	// Completer is called when upload completes.
	Completer func(entry *UploadEntry) (string, error)
}

// PresignedURL contains information for direct upload.
type PresignedURL struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Fields  map[string]string `json:"fields,omitempty"` // For form-based uploads
}

// UploadHandler handles file upload HTTP requests.
type UploadHandler struct {
	config    *UploadConfig
	destDir   string
	onSuccess func(entry *UploadEntry)
	onError   func(entry *UploadEntry, err error)
}

// NewUploadHandler creates a new upload handler.
func NewUploadHandler(config *UploadConfig, destDir string) *UploadHandler {
	if config == nil {
		config = DefaultUploadConfig()
	}
	return &UploadHandler{
		config:  config,
		destDir: destDir,
	}
}

// OnSuccess sets the success callback.
func (h *UploadHandler) OnSuccess(fn func(entry *UploadEntry)) *UploadHandler {
	h.onSuccess = fn
	return h
}

// OnError sets the error callback.
func (h *UploadHandler) OnError(fn func(entry *UploadEntry, err error)) *UploadHandler {
	h.onError = fn
	return h
}

// ServeHTTP handles upload requests.
func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(h.config.MaxFileSize); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		http.Error(w, "No files uploaded", http.StatusBadRequest)
		return
	}

	var results []*UploadEntry

	for _, fileHeader := range files {
		entry, err := h.handleFile(r.Context(), fileHeader)
		if err != nil {
			if h.onError != nil {
				h.onError(entry, err)
			}
			continue
		}

		if h.onSuccess != nil {
			h.onSuccess(entry)
		}
		results = append(results, entry)
	}

	// Return results as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (h *UploadHandler) handleFile(ctx context.Context, header *multipart.FileHeader) (*UploadEntry, error) {
	// Create entry
	entry := &UploadEntry{
		UUID:        generateUUID(),
		FileName:    sanitizeFilename(header.Filename),
		Size:        header.Size,
		ContentType: header.Header.Get("Content-Type"),
		CreatedAt:   time.Now(),
	}

	// Validate
	if entry.Size > h.config.MaxFileSize {
		entry.Errors = append(entry.Errors, "File too large")
		return entry, ErrFileTooLarge
	}

	// Open uploaded file
	src, err := header.Open()
	if err != nil {
		entry.Errors = append(entry.Errors, "Failed to open file")
		return entry, err
	}
	defer src.Close()

	// Create destination
	destPath := filepath.Join(h.destDir, entry.UUID+filepath.Ext(entry.FileName))
	dst, err := os.Create(destPath)
	if err != nil {
		entry.Errors = append(entry.Errors, "Failed to create file")
		return entry, err
	}
	defer dst.Close()

	// Copy
	if _, err := io.Copy(dst, src); err != nil {
		entry.Errors = append(entry.Errors, "Failed to save file")
		return entry, err
	}

	entry.Done = true
	entry.Progress = 100
	entry.TempPath = destPath
	entry.URL = "/uploads/" + filepath.Base(destPath)

	return entry, nil
}

// Helper functions

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func sanitizeFilename(filename string) string {
	// Remove path separators
	filename = filepath.Base(filename)

	// Remove potentially dangerous characters
	filename = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\x00' {
			return '_'
		}
		return r
	}, filename)

	// Limit length
	if len(filename) > 255 {
		ext := filepath.Ext(filename)
		filename = filename[:255-len(ext)] + ext
	}

	return filename
}

// Progress tracks upload progress.
type Progress struct {
	UUID       string `json:"uuid"`
	Uploaded   int64  `json:"uploaded"`
	Total      int64  `json:"total"`
	Percentage int    `json:"percentage"`
}

// ProgressTracker tracks upload progress.
type ProgressTracker struct {
	progresses sync.Map
}

// NewProgressTracker creates a new progress tracker.
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{}
}

// Update updates progress for an upload.
func (pt *ProgressTracker) Update(uuid string, uploaded, total int64) {
	percentage := int(float64(uploaded) / float64(total) * 100)
	pt.progresses.Store(uuid, Progress{
		UUID:       uuid,
		Uploaded:   uploaded,
		Total:      total,
		Percentage: percentage,
	})
}

// Get retrieves progress for an upload.
func (pt *ProgressTracker) Get(uuid string) (Progress, bool) {
	p, ok := pt.progresses.Load(uuid)
	if !ok {
		return Progress{}, false
	}
	return p.(Progress), true
}

// Remove removes progress tracking.
func (pt *ProgressTracker) Remove(uuid string) {
	pt.progresses.Delete(uuid)
}

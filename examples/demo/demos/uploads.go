// Package demos provides demo components for GoliveKit showcase.
package demos

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// FileType represents the type of a file
type FileType int

const (
	FileTypeFolder FileType = iota
	FileTypeDocument
	FileTypeImage
	FileTypeArchive
	FileTypeCode
	FileTypeOther
)

// FileItem represents a file or folder
type FileItem struct {
	ID         string
	Name       string
	Type       FileType
	Size       int64
	ModTime    time.Time
	Children   []string // IDs of children (for folders)
	ParentID   string
	IsSelected bool
}

// UploadJob represents an upload in progress
type UploadJob struct {
	ID       string
	Filename string
	Size     int64
	Progress int
	Status   string // "uploading", "complete", "error"
	Error    string
}

// FileManager is the file manager component.
type FileManager struct {
	core.BaseComponent

	// File system state (simulated)
	Files       map[string]*FileItem
	CurrentPath []string // Path as list of folder IDs
	Selected    map[string]bool

	// View options
	ViewMode   string // "grid" or "list"
	SortBy     string // "name", "size", "date"
	SortAsc    bool
	ShowHidden bool

	// Upload state
	Uploads     []*UploadJob
	IsUploading bool

	// Modal state
	ShowNewFolder bool
	NewFolderName string
	ShowRename    bool
	RenameTarget  string
	RenameValue   string

	// Clipboard
	Clipboard   []string
	ClipboardOp string // "copy" or "cut"
}

// NewFileManager creates a new file manager component.
func NewFileManager() core.Component {
	return &FileManager{
		ViewMode: "grid",
		SortBy:   "name",
		SortAsc:  true,
	}
}

// Name returns the component name.
func (f *FileManager) Name() string {
	return "file-manager"
}

// Mount initializes the file manager.
func (f *FileManager) Mount(ctx context.Context, params core.Params, session core.Session) error {
	// Initialize with demo file structure
	f.Files = make(map[string]*FileItem)
	f.Selected = make(map[string]bool)
	f.CurrentPath = []string{"root"}

	// Root folder
	f.Files["root"] = &FileItem{
		ID:       "root",
		Name:     "Home",
		Type:     FileTypeFolder,
		Children: []string{"folder1", "folder2", "file1", "file2", "file3"},
	}

	// Demo folders
	f.Files["folder1"] = &FileItem{
		ID:       "folder1",
		Name:     "Documents",
		Type:     FileTypeFolder,
		ParentID: "root",
		Children: []string{"doc1", "doc2"},
		ModTime:  time.Now().Add(-24 * time.Hour),
	}
	f.Files["folder2"] = &FileItem{
		ID:       "folder2",
		Name:     "Images",
		Type:     FileTypeFolder,
		ParentID: "root",
		Children: []string{"img1", "img2", "img3"},
		ModTime:  time.Now().Add(-48 * time.Hour),
	}

	// Demo files in root
	f.Files["file1"] = &FileItem{
		ID:       "file1",
		Name:     "README.md",
		Type:     FileTypeDocument,
		Size:     2456,
		ParentID: "root",
		ModTime:  time.Now().Add(-2 * time.Hour),
	}
	f.Files["file2"] = &FileItem{
		ID:       "file2",
		Name:     "config.yml",
		Type:     FileTypeCode,
		Size:     1024,
		ParentID: "root",
		ModTime:  time.Now().Add(-3 * time.Hour),
	}
	f.Files["file3"] = &FileItem{
		ID:       "file3",
		Name:     "archive.zip",
		Type:     FileTypeArchive,
		Size:     15728640,
		ParentID: "root",
		ModTime:  time.Now().Add(-5 * time.Hour),
	}

	// Files in Documents
	f.Files["doc1"] = &FileItem{
		ID:       "doc1",
		Name:     "report.pdf",
		Type:     FileTypeDocument,
		Size:     524288,
		ParentID: "folder1",
		ModTime:  time.Now().Add(-6 * time.Hour),
	}
	f.Files["doc2"] = &FileItem{
		ID:       "doc2",
		Name:     "notes.txt",
		Type:     FileTypeDocument,
		Size:     1234,
		ParentID: "folder1",
		ModTime:  time.Now().Add(-7 * time.Hour),
	}

	// Files in Images
	f.Files["img1"] = &FileItem{
		ID:       "img1",
		Name:     "photo.jpg",
		Type:     FileTypeImage,
		Size:     2097152,
		ParentID: "folder2",
		ModTime:  time.Now().Add(-8 * time.Hour),
	}
	f.Files["img2"] = &FileItem{
		ID:       "img2",
		Name:     "logo.png",
		Type:     FileTypeImage,
		Size:     102400,
		ParentID: "folder2",
		ModTime:  time.Now().Add(-9 * time.Hour),
	}
	f.Files["img3"] = &FileItem{
		ID:       "img3",
		Name:     "banner.svg",
		Type:     FileTypeImage,
		Size:     51200,
		ParentID: "folder2",
		ModTime:  time.Now().Add(-10 * time.Hour),
	}

	return nil
}

// Terminate handles cleanup.
func (f *FileManager) Terminate(ctx context.Context, reason core.TerminateReason) error {
	return nil
}

// getCurrentFolder returns the current folder
func (f *FileManager) getCurrentFolder() *FileItem {
	if len(f.CurrentPath) == 0 {
		return f.Files["root"]
	}
	return f.Files[f.CurrentPath[len(f.CurrentPath)-1]]
}

// HandleEvent handles user interactions.
func (f *FileManager) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	// Navigation
	case "navigate":
		if id, ok := payload["id"].(string); ok {
			if file, exists := f.Files[id]; exists && file.Type == FileTypeFolder {
				f.CurrentPath = append(f.CurrentPath, id)
				f.Selected = make(map[string]bool)
			}
		}

	case "navigate_up":
		if len(f.CurrentPath) > 1 {
			f.CurrentPath = f.CurrentPath[:len(f.CurrentPath)-1]
			f.Selected = make(map[string]bool)
		}

	case "navigate_to":
		if idx, ok := payload["index"].(float64); ok {
			index := int(idx)
			if index >= 0 && index < len(f.CurrentPath) {
				f.CurrentPath = f.CurrentPath[:index+1]
				f.Selected = make(map[string]bool)
			}
		}

	// Selection
	case "select":
		if id, ok := payload["id"].(string); ok {
			multi, _ := payload["multi"].(bool)
			if !multi {
				f.Selected = make(map[string]bool)
			}
			f.Selected[id] = !f.Selected[id]
			if !f.Selected[id] {
				delete(f.Selected, id)
			}
		}

	case "select_all":
		folder := f.getCurrentFolder()
		allSelected := len(f.Selected) == len(folder.Children)
		f.Selected = make(map[string]bool)
		if !allSelected {
			for _, childID := range folder.Children {
				f.Selected[childID] = true
			}
		}

	// View options
	case "set_view":
		if mode, ok := payload["mode"].(string); ok {
			f.ViewMode = mode
		}

	case "set_sort":
		if by, ok := payload["by"].(string); ok {
			if f.SortBy == by {
				f.SortAsc = !f.SortAsc
			} else {
				f.SortBy = by
				f.SortAsc = true
			}
		}

	// File operations
	case "new_folder":
		f.ShowNewFolder = true
		f.NewFolderName = ""

	case "create_folder":
		if f.NewFolderName != "" {
			f.createFolder(f.NewFolderName)
		}
		f.ShowNewFolder = false
		f.NewFolderName = ""

	case "cancel_new_folder":
		f.ShowNewFolder = false
		f.NewFolderName = ""

	case "update_folder_name":
		if val, ok := payload["value"].(string); ok {
			f.NewFolderName = val
		}

	case "rename":
		if len(f.Selected) == 1 {
			for id := range f.Selected {
				f.RenameTarget = id
				f.RenameValue = f.Files[id].Name
				f.ShowRename = true
			}
		}

	case "do_rename":
		if f.RenameTarget != "" && f.RenameValue != "" {
			if file, exists := f.Files[f.RenameTarget]; exists {
				file.Name = f.RenameValue
			}
		}
		f.ShowRename = false
		f.RenameTarget = ""
		f.RenameValue = ""

	case "cancel_rename":
		f.ShowRename = false
		f.RenameTarget = ""
		f.RenameValue = ""

	case "update_rename":
		if val, ok := payload["value"].(string); ok {
			f.RenameValue = val
		}

	case "delete":
		for id := range f.Selected {
			f.deleteFile(id)
		}
		f.Selected = make(map[string]bool)

	case "copy":
		f.Clipboard = make([]string, 0, len(f.Selected))
		for id := range f.Selected {
			f.Clipboard = append(f.Clipboard, id)
		}
		f.ClipboardOp = "copy"

	case "cut":
		f.Clipboard = make([]string, 0, len(f.Selected))
		for id := range f.Selected {
			f.Clipboard = append(f.Clipboard, id)
		}
		f.ClipboardOp = "cut"

	case "paste":
		f.pasteFiles()

	// Upload simulation
	case "start_upload":
		filename, _ := payload["filename"].(string)
		size, _ := payload["size"].(float64)
		f.startUpload(filename, int64(size))

	case "upload_progress":
		id, _ := payload["id"].(string)
		progress, _ := payload["progress"].(float64)
		f.updateUploadProgress(id, int(progress))

	case "cancel_upload":
		id, _ := payload["id"].(string)
		f.cancelUpload(id)

	case "clear_uploads":
		f.Uploads = nil
		f.IsUploading = false

	// Keyboard shortcuts (simulated)
	case "keydown":
		key, _ := payload["key"].(string)
		ctrl, _ := payload["ctrl"].(bool)
		f.handleKeyboard(key, ctrl)
	}

	return nil
}

// createFolder creates a new folder in the current directory
func (f *FileManager) createFolder(name string) {
	id := fmt.Sprintf("folder_%d", time.Now().UnixNano())
	currentFolder := f.getCurrentFolder()

	f.Files[id] = &FileItem{
		ID:       id,
		Name:     name,
		Type:     FileTypeFolder,
		ParentID: currentFolder.ID,
		ModTime:  time.Now(),
		Children: []string{},
	}

	currentFolder.Children = append(currentFolder.Children, id)
}

// deleteFile deletes a file or folder
func (f *FileManager) deleteFile(id string) {
	file, exists := f.Files[id]
	if !exists {
		return
	}

	// Remove from parent
	if parent, ok := f.Files[file.ParentID]; ok {
		newChildren := make([]string, 0, len(parent.Children)-1)
		for _, childID := range parent.Children {
			if childID != id {
				newChildren = append(newChildren, childID)
			}
		}
		parent.Children = newChildren
	}

	// Recursively delete children
	if file.Type == FileTypeFolder {
		for _, childID := range file.Children {
			f.deleteFile(childID)
		}
	}

	delete(f.Files, id)
}

// pasteFiles pastes files from clipboard
func (f *FileManager) pasteFiles() {
	if len(f.Clipboard) == 0 {
		return
	}

	currentFolder := f.getCurrentFolder()

	for _, id := range f.Clipboard {
		original, exists := f.Files[id]
		if !exists {
			continue
		}

		if f.ClipboardOp == "copy" {
			// Create a copy
			newID := fmt.Sprintf("%s_copy_%d", id, time.Now().UnixNano())
			f.Files[newID] = &FileItem{
				ID:       newID,
				Name:     "Copy of " + original.Name,
				Type:     original.Type,
				Size:     original.Size,
				ParentID: currentFolder.ID,
				ModTime:  time.Now(),
			}
			currentFolder.Children = append(currentFolder.Children, newID)
		} else {
			// Move
			// Remove from old parent
			if parent, ok := f.Files[original.ParentID]; ok {
				newChildren := make([]string, 0, len(parent.Children)-1)
				for _, childID := range parent.Children {
					if childID != id {
						newChildren = append(newChildren, childID)
					}
				}
				parent.Children = newChildren
			}

			// Add to new parent
			original.ParentID = currentFolder.ID
			currentFolder.Children = append(currentFolder.Children, id)
		}
	}

	f.Clipboard = nil
	f.ClipboardOp = ""
}

// startUpload starts a simulated upload
func (f *FileManager) startUpload(filename string, size int64) {
	if size == 0 {
		size = 1024 * 1024 // Default 1MB
	}

	upload := &UploadJob{
		ID:       fmt.Sprintf("upload_%d", time.Now().UnixNano()),
		Filename: filename,
		Size:     size,
		Progress: 0,
		Status:   "uploading",
	}

	f.Uploads = append(f.Uploads, upload)
	f.IsUploading = true
}

// updateUploadProgress updates upload progress
func (f *FileManager) updateUploadProgress(id string, progress int) {
	for _, upload := range f.Uploads {
		if upload.ID == id {
			upload.Progress = progress
			if progress >= 100 {
				upload.Status = "complete"
				// Add file to current folder
				f.addUploadedFile(upload.Filename, upload.Size)
			}
			break
		}
	}

	// Check if all uploads complete
	allDone := true
	for _, upload := range f.Uploads {
		if upload.Status == "uploading" {
			allDone = false
			break
		}
	}
	if allDone {
		f.IsUploading = false
	}
}

// cancelUpload cancels an upload
func (f *FileManager) cancelUpload(id string) {
	var newUploads []*UploadJob
	for _, upload := range f.Uploads {
		if upload.ID != id {
			newUploads = append(newUploads, upload)
		}
	}
	f.Uploads = newUploads
}

// addUploadedFile adds a file from upload
func (f *FileManager) addUploadedFile(filename string, size int64) {
	id := fmt.Sprintf("file_%d", time.Now().UnixNano())
	currentFolder := f.getCurrentFolder()

	fileType := getFileType(filename)

	f.Files[id] = &FileItem{
		ID:       id,
		Name:     filename,
		Type:     fileType,
		Size:     size,
		ParentID: currentFolder.ID,
		ModTime:  time.Now(),
	}

	currentFolder.Children = append(currentFolder.Children, id)
}

// handleKeyboard handles keyboard shortcuts
func (f *FileManager) handleKeyboard(key string, ctrl bool) {
	if ctrl {
		switch key {
		case "c":
			f.Clipboard = make([]string, 0, len(f.Selected))
			for id := range f.Selected {
				f.Clipboard = append(f.Clipboard, id)
			}
			f.ClipboardOp = "copy"
		case "x":
			f.Clipboard = make([]string, 0, len(f.Selected))
			for id := range f.Selected {
				f.Clipboard = append(f.Clipboard, id)
			}
			f.ClipboardOp = "cut"
		case "v":
			f.pasteFiles()
		case "a":
			folder := f.getCurrentFolder()
			for _, id := range folder.Children {
				f.Selected[id] = true
			}
		}
	} else {
		switch key {
		case "Delete", "Backspace":
			for id := range f.Selected {
				f.deleteFile(id)
			}
			f.Selected = make(map[string]bool)
		case "F2":
			if len(f.Selected) == 1 {
				for id := range f.Selected {
					f.RenameTarget = id
					f.RenameValue = f.Files[id].Name
					f.ShowRename = true
				}
			}
		}
	}
}

// getFileType determines file type from extension
func getFileType(filename string) FileType {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".svg", ".webp", ".bmp":
		return FileTypeImage
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return FileTypeArchive
	case ".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".css", ".html":
		return FileTypeCode
	case ".pdf", ".doc", ".docx", ".txt", ".md", ".rtf":
		return FileTypeDocument
	default:
		return FileTypeOther
	}
}

// getSortedChildren returns sorted children of a folder
func (f *FileManager) getSortedChildren(folder *FileItem) []*FileItem {
	items := make([]*FileItem, 0, len(folder.Children))
	for _, id := range folder.Children {
		if file, exists := f.Files[id]; exists {
			items = append(items, file)
		}
	}

	// Sort folders first, then by selected criteria
	sort.Slice(items, func(i, j int) bool {
		// Folders always first
		if items[i].Type == FileTypeFolder && items[j].Type != FileTypeFolder {
			return true
		}
		if items[i].Type != FileTypeFolder && items[j].Type == FileTypeFolder {
			return false
		}

		var less bool
		switch f.SortBy {
		case "size":
			less = items[i].Size < items[j].Size
		case "date":
			less = items[i].ModTime.Before(items[j].ModTime)
		default: // name
			less = strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}

		if !f.SortAsc {
			less = !less
		}
		return less
	})

	return items
}

// Render returns the HTML representation.
func (f *FileManager) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := f.renderFileManager()
		_, err := w.Write([]byte(html))
		return err
	})
}

// renderFileManager generates the complete file manager HTML
func (f *FileManager) renderFileManager() string {
	cfg := website.PageConfig{
		Title:       "File Manager - GoliveKit Demo",
		Description: "File manager with uploads, drag-drop, and keyboard shortcuts.",
		URL:         "https://golivekit.cloud/demos/uploads",
		Keywords:    []string{"uploads", "files", "manager", "liveview"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	body := f.renderFileManagerBody()
	return website.RenderDocument(cfg, renderFileManagerStyles(), body)
}

// renderFileManagerStyles returns custom CSS
func renderFileManagerStyles() string {
	return `
<style>
.fm-container {
	max-width: 1100px;
	margin: 0 auto;
	padding: 1.5rem;
}

.fm-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 1rem;
	flex-wrap: wrap;
	gap: 1rem;
}

.fm-title {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.fm-title h1 {
	font-size: 1.5rem;
	margin: 0;
}

.fm-actions {
	display: flex;
	gap: 0.5rem;
}

.fm-btn {
	padding: 0.5rem 1rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	cursor: pointer;
	font-size: 0.875rem;
	display: flex;
	align-items: center;
	gap: 0.375rem;
	transition: all 0.2s;
}

.fm-btn:hover {
	border-color: var(--color-primary);
}

.fm-btn-primary {
	background: var(--color-primary);
	border-color: var(--color-primary);
	color: white;
}

.fm-btn-primary:hover {
	background: #7c3aed;
}

.fm-breadcrumb {
	display: flex;
	align-items: center;
	gap: 0.25rem;
	padding: 0.75rem 1rem;
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	margin-bottom: 1rem;
	font-size: 0.875rem;
}

.breadcrumb-item {
	cursor: pointer;
	color: var(--color-textMuted);
	transition: color 0.2s;
}

.breadcrumb-item:hover {
	color: var(--color-primary);
}

.breadcrumb-item.current {
	color: var(--color-text);
	font-weight: 600;
}

.breadcrumb-sep {
	color: var(--color-border);
}

.fm-toolbar {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 0.5rem 1rem;
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.5rem 0.5rem 0 0;
	font-size: 0.8125rem;
}

.toolbar-left {
	display: flex;
	align-items: center;
	gap: 1rem;
}

.toolbar-right {
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.view-btn {
	width: 32px;
	height: 32px;
	border: 1px solid var(--color-border);
	background: var(--color-bg);
	border-radius: 4px;
	cursor: pointer;
	display: flex;
	align-items: center;
	justify-content: center;
}

.view-btn.active {
	background: var(--color-primary);
	border-color: var(--color-primary);
	color: white;
}

.sort-select {
	padding: 0.375rem 0.75rem;
	border: 1px solid var(--color-border);
	border-radius: 0.375rem;
	background: var(--color-bg);
	font-size: 0.8125rem;
}

.fm-content {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-top: none;
	border-radius: 0 0 0.5rem 0.5rem;
	min-height: 400px;
	padding: 1rem;
}

.fm-grid {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
	gap: 1rem;
}

.fm-list {
	display: flex;
	flex-direction: column;
}

.file-item-grid {
	display: flex;
	flex-direction: column;
	align-items: center;
	padding: 1rem 0.5rem;
	border-radius: 0.5rem;
	cursor: pointer;
	transition: all 0.2s;
}

.file-item-grid:hover {
	background: var(--color-bg);
}

.file-item-grid.selected {
	background: rgba(139, 92, 246, 0.1);
	outline: 2px solid var(--color-primary);
}

.file-icon {
	font-size: 2.5rem;
	margin-bottom: 0.5rem;
}

.file-name {
	font-size: 0.75rem;
	text-align: center;
	word-break: break-word;
	max-width: 100%;
}

.file-item-list {
	display: flex;
	align-items: center;
	gap: 1rem;
	padding: 0.75rem 1rem;
	border-radius: 0.5rem;
	cursor: pointer;
	transition: all 0.2s;
}

.file-item-list:hover {
	background: var(--color-bg);
}

.file-item-list.selected {
	background: rgba(139, 92, 246, 0.1);
	outline: 2px solid var(--color-primary);
}

.file-item-list .file-icon {
	font-size: 1.5rem;
	margin: 0;
}

.file-item-list .file-name {
	flex: 1;
	font-size: 0.875rem;
	text-align: left;
}

.file-size {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	width: 80px;
	text-align: right;
}

.file-date {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	width: 100px;
	text-align: right;
}

.upload-panel {
	background: var(--color-bg);
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	padding: 1rem;
	margin-bottom: 1rem;
}

.upload-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 0.75rem;
}

.upload-title {
	font-weight: 600;
	font-size: 0.875rem;
}

.upload-item {
	display: flex;
	align-items: center;
	gap: 0.75rem;
	padding: 0.5rem 0;
	border-bottom: 1px solid var(--color-border);
}

.upload-item:last-child {
	border-bottom: none;
}

.upload-icon {
	font-size: 1.25rem;
}

.upload-info {
	flex: 1;
}

.upload-filename {
	font-size: 0.875rem;
	margin-bottom: 0.25rem;
}

.upload-progress {
	height: 4px;
	background: var(--color-border);
	border-radius: 2px;
	overflow: hidden;
}

.upload-progress-fill {
	height: 100%;
	background: var(--color-primary);
	transition: width 0.3s;
}

.upload-progress-fill.complete {
	background: var(--color-success);
}

.upload-status {
	font-size: 0.75rem;
	color: var(--color-textMuted);
}

.modal-overlay {
	position: fixed;
	inset: 0;
	background: rgba(0,0,0,0.5);
	display: flex;
	align-items: center;
	justify-content: center;
	z-index: 100;
}

.modal {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
	padding: 1.5rem;
	min-width: 300px;
	max-width: 90%;
}

.modal-title {
	font-size: 1.125rem;
	font-weight: 600;
	margin-bottom: 1rem;
}

.modal-input {
	width: 100%;
	padding: 0.75rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	color: var(--color-text);
	font-size: 1rem;
	margin-bottom: 1rem;
}

.modal-input:focus {
	outline: none;
	border-color: var(--color-primary);
}

.modal-actions {
	display: flex;
	justify-content: flex-end;
	gap: 0.5rem;
}

.selected-info {
	font-size: 0.8125rem;
	color: var(--color-textMuted);
}

.context-menu {
	position: absolute;
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	padding: 0.5rem 0;
	min-width: 150px;
	box-shadow: 0 4px 20px rgba(0,0,0,0.2);
	z-index: 50;
}

.context-item {
	padding: 0.5rem 1rem;
	cursor: pointer;
	font-size: 0.875rem;
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.context-item:hover {
	background: var(--color-bg);
}

.context-sep {
	height: 1px;
	background: var(--color-border);
	margin: 0.5rem 0;
}

.back-link {
	display: inline-flex;
	align-items: center;
	gap: 0.5rem;
	color: var(--color-textMuted);
	text-decoration: none;
	margin-bottom: 1rem;
	transition: color 0.2s;
}

.back-link:hover {
	color: var(--color-primary);
}

.empty-folder {
	text-align: center;
	padding: 3rem;
	color: var(--color-textMuted);
}

.empty-folder-icon {
	font-size: 3rem;
	margin-bottom: 1rem;
}

.drop-zone {
	border: 2px dashed var(--color-border);
	border-radius: 0.5rem;
	padding: 2rem;
	text-align: center;
	margin-bottom: 1rem;
	transition: all 0.2s;
}

.drop-zone.active {
	border-color: var(--color-primary);
	background: rgba(139, 92, 246, 0.05);
}

.drop-zone-text {
	color: var(--color-textMuted);
}
</style>
`
}

// renderFileManagerBody generates the main content
func (f *FileManager) renderFileManagerBody() string {
	// Navbar
	navbar := components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "Uploads Demo",
		Links: []website.NavLink{
			{Label: "Demos", URL: "/demos", External: false},
			{Label: "Docs", URL: "/docs", External: false},
		},
	})

	selectedCount := len(f.Selected)
	currentFolder := f.getCurrentFolder()
	items := f.getSortedChildren(currentFolder)

	content := fmt.Sprintf(`
<main id="main-content">
<div class="fm-container" data-live-view="file-manager" lv-keydown="keydown" tabindex="0">

<a href="/demos" class="back-link">← Back to Demos</a>

<div class="fm-header">
	<div class="fm-title">
		<span style="font-size:1.5rem">📁</span>
		<h1>File Manager</h1>
	</div>
	<div class="fm-actions">
		<button class="fm-btn fm-btn-primary" lv-click="start_upload" lv-value-filename="newfile.txt" lv-value-size="1048576">
			📤 Upload
		</button>
		<button class="fm-btn" lv-click="new_folder">📁 New Folder</button>
	</div>
</div>

%s

%s

<div class="fm-toolbar">
	<div class="toolbar-left">
		<span class="selected-info" data-slot="selected">
			%s
		</span>
	</div>
	<div class="toolbar-right">
		%s
		<button class="view-btn %s" lv-click="set_view" lv-value-mode="grid" title="Grid view">⊞</button>
		<button class="view-btn %s" lv-click="set_view" lv-value-mode="list" title="List view">≡</button>
	</div>
</div>

<div class="fm-content" data-slot="content">
	%s
</div>

%s

</div>
</main>

<script src="/_live/golivekit.js"></script>
<script>
// Simulate upload progress
var uploadIntervals = {};
document.addEventListener('click', function(e) {
	if (e.target.textContent.includes('Upload')) {
		var id = 'upload_' + Date.now();
		var progress = 0;
		uploadIntervals[id] = setInterval(function() {
			progress += Math.random() * 15;
			if (progress >= 100) {
				progress = 100;
				clearInterval(uploadIntervals[id]);
			}
			if (window.liveSocket && window.liveSocket.isConnected()) {
				window.liveSocket.pushEvent("upload_progress", {id: id, progress: progress});
			}
		}, 200);
	}
});
</script>
`, f.renderBreadcrumb(), f.renderUploadPanel(), f.renderSelectedInfo(selectedCount),
		f.renderSortControls(), f.viewClass("grid"), f.viewClass("list"),
		f.renderFiles(items), f.renderModals())

	return navbar + content
}

// viewClass returns active class if view matches
func (f *FileManager) viewClass(mode string) string {
	if f.ViewMode == mode {
		return "active"
	}
	return ""
}

// renderSelectedInfo renders selected items info
func (f *FileManager) renderSelectedInfo(count int) string {
	if count == 0 {
		folder := f.getCurrentFolder()
		return fmt.Sprintf("%d items", len(folder.Children))
	}
	return fmt.Sprintf("%d selected", count)
}

// renderSortControls renders sort controls
func (f *FileManager) renderSortControls() string {
	return fmt.Sprintf(`
<select class="sort-select" lv-change="set_sort">
	<option value="name" %s>Name</option>
	<option value="size" %s>Size</option>
	<option value="date" %s>Date</option>
</select>
`, selected("name", f.SortBy), selected("size", f.SortBy), selected("date", f.SortBy))
}

// renderBreadcrumb renders the path breadcrumb
func (f *FileManager) renderBreadcrumb() string {
	var html string
	for i, id := range f.CurrentPath {
		folder := f.Files[id]
		if folder == nil {
			continue
		}

		class := "breadcrumb-item"
		if i == len(f.CurrentPath)-1 {
			class += " current"
		}

		html += fmt.Sprintf(`<span class="%s" lv-click="navigate_to" lv-value-index="%d">📂 %s</span>`, class, i, folder.Name)

		if i < len(f.CurrentPath)-1 {
			html += `<span class="breadcrumb-sep">/</span>`
		}
	}

	return `<div class="fm-breadcrumb">` + html + `</div>`
}

// renderUploadPanel renders the upload progress panel
func (f *FileManager) renderUploadPanel() string {
	if len(f.Uploads) == 0 {
		return ""
	}

	var items string
	for _, upload := range f.Uploads {
		statusClass := ""
		if upload.Status == "complete" {
			statusClass = "complete"
		}

		icon := "📄"
		if upload.Status == "complete" {
			icon = "✅"
		}

		items += fmt.Sprintf(`
<div class="upload-item">
	<span class="upload-icon">%s</span>
	<div class="upload-info">
		<div class="upload-filename">%s</div>
		<div class="upload-progress">
			<div class="upload-progress-fill %s" style="width:%d%%"></div>
		</div>
	</div>
	<span class="upload-status">%d%%</span>
</div>
`, icon, upload.Filename, statusClass, upload.Progress, upload.Progress)
	}

	return fmt.Sprintf(`
<div class="upload-panel" data-slot="uploads">
	<div class="upload-header">
		<span class="upload-title">Uploading %d files...</span>
		<button class="fm-btn" lv-click="clear_uploads">✕</button>
	</div>
	%s
</div>
`, len(f.Uploads), items)
}

// renderFiles renders the file list or grid
func (f *FileManager) renderFiles(items []*FileItem) string {
	if len(items) == 0 {
		return `
<div class="empty-folder">
	<div class="empty-folder-icon">📂</div>
	<p>This folder is empty</p>
	<p style="font-size:0.875rem">Drop files here or click Upload</p>
</div>
`
	}

	if f.ViewMode == "list" {
		return f.renderFilesList(items)
	}
	return f.renderFilesGrid(items)
}

// renderFilesGrid renders files in grid view
func (f *FileManager) renderFilesGrid(items []*FileItem) string {
	var html string
	for _, item := range items {
		selectedClass := ""
		if f.Selected[item.ID] {
			selectedClass = "selected"
		}

		icon := getFileIcon(item.Type)
		event := "select"
		if item.Type == FileTypeFolder {
			event = "navigate"
		}

		html += fmt.Sprintf(`
<div class="file-item-grid %s" lv-click="%s" lv-value-id="%s">
	<span class="file-icon">%s</span>
	<span class="file-name">%s</span>
</div>
`, selectedClass, event, item.ID, icon, item.Name)
	}

	return `<div class="fm-grid">` + html + `</div>`
}

// renderFilesList renders files in list view
func (f *FileManager) renderFilesList(items []*FileItem) string {
	var html string
	for _, item := range items {
		selectedClass := ""
		if f.Selected[item.ID] {
			selectedClass = "selected"
		}

		icon := getFileIcon(item.Type)
		event := "select"
		if item.Type == FileTypeFolder {
			event = "navigate"
		}

		size := ""
		if item.Type != FileTypeFolder {
			size = formatSize(item.Size)
		}

		date := item.ModTime.Format("Jan 02, 15:04")

		html += fmt.Sprintf(`
<div class="file-item-list %s" lv-click="%s" lv-value-id="%s">
	<span class="file-icon">%s</span>
	<span class="file-name">%s</span>
	<span class="file-size">%s</span>
	<span class="file-date">%s</span>
</div>
`, selectedClass, event, item.ID, icon, item.Name, size, date)
	}

	return `<div class="fm-list">` + html + `</div>`
}

// renderModals renders modal dialogs
func (f *FileManager) renderModals() string {
	if f.ShowNewFolder {
		return fmt.Sprintf(`
<div class="modal-overlay" lv-click="cancel_new_folder">
	<div class="modal" onclick="event.stopPropagation()">
		<div class="modal-title">📁 New Folder</div>
		<input type="text" class="modal-input" placeholder="Folder name"
			lv-change="update_folder_name" lv-debounce="100" value="%s" autofocus>
		<div class="modal-actions">
			<button class="fm-btn" lv-click="cancel_new_folder">Cancel</button>
			<button class="fm-btn fm-btn-primary" lv-click="create_folder">Create</button>
		</div>
	</div>
</div>
`, f.NewFolderName)
	}

	if f.ShowRename {
		return fmt.Sprintf(`
<div class="modal-overlay" lv-click="cancel_rename">
	<div class="modal" onclick="event.stopPropagation()">
		<div class="modal-title">✏️ Rename</div>
		<input type="text" class="modal-input" placeholder="New name"
			lv-change="update_rename" lv-debounce="100" value="%s" autofocus>
		<div class="modal-actions">
			<button class="fm-btn" lv-click="cancel_rename">Cancel</button>
			<button class="fm-btn fm-btn-primary" lv-click="do_rename">Rename</button>
		</div>
	</div>
</div>
`, f.RenameValue)
	}

	return ""
}

// getFileIcon returns an emoji icon for the file type
func getFileIcon(t FileType) string {
	switch t {
	case FileTypeFolder:
		return "📁"
	case FileTypeDocument:
		return "📄"
	case FileTypeImage:
		return "🖼️"
	case FileTypeArchive:
		return "📦"
	case FileTypeCode:
		return "💻"
	default:
		return "📄"
	}
}

// formatSize formats bytes to human readable
func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

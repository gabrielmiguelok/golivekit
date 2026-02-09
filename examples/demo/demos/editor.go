// Package demos provides demo components for GoliveKit showcase.
package demos

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// EditorUser represents a user in the editor room
type EditorUser struct {
	ID         string
	Name       string
	Color      string
	CursorPos  int
	IsTyping   bool
	LastActive time.Time
}

// DocumentVersion represents a saved version
type DocumentVersion struct {
	ID        string
	Content   string
	Author    string
	Timestamp time.Time
}

// EditorRoom represents a collaborative editing room
type EditorRoom struct {
	ID          string
	Content     string
	Users       map[string]*EditorUser
	Versions    []DocumentVersion
	LastSaved   time.Time
	WordCount   int
	CharCount   int
}

// Global rooms (simulated PubSub)
var (
	editorRooms    = make(map[string]*EditorRoom)
	editorRoomsMu  sync.RWMutex
	editorUsers    atomic.Int64
	editorColors   = []string{"#ef4444", "#f97316", "#eab308", "#22c55e", "#06b6d4", "#3b82f6", "#8b5cf6", "#ec4899"}
	editorColorIdx int
)

func init() {
	// Create a default room with sample content
	editorRooms["default"] = &EditorRoom{
		ID: "default",
		Content: `# Meeting Notes

## Agenda

1. Project status update
2. Budget review
3. Next steps

## Notes

- Action item: Review proposal by Friday
- Follow up with marketing team
- Schedule demo for stakeholders

## Action Items

| Owner | Task | Due |
|-------|------|-----|
| Alice | Review PR | Monday |
| Bob | Update docs | Tuesday |
| Carol | Deploy v2 | Wednesday |
`,
		Users:     make(map[string]*EditorUser),
		Versions:  []DocumentVersion{},
		LastSaved: time.Now(),
	}
	editorRooms["default"].updateCounts()
}

// updateCounts updates word and character counts
func (r *EditorRoom) updateCounts() {
	r.CharCount = len(r.Content)
	words := strings.Fields(r.Content)
	r.WordCount = len(words)
}

// CollabEditor is the collaborative text editor component.
type CollabEditor struct {
	core.BaseComponent

	// User state
	UserID      string
	UserName    string
	UserColor   string
	RoomID      string
	CursorPos   int
	IsTyping    bool

	// Editor state
	LocalContent string
	AutoSave     bool
	LastSaved    time.Time

	// UI state
	ShowShareModal bool
}

// NewCollabEditor creates a new collaborative editor component.
func NewCollabEditor() core.Component {
	return &CollabEditor{
		AutoSave: true,
	}
}

// Name returns the component name.
func (e *CollabEditor) Name() string {
	return "collab-editor"
}

// Mount initializes the editor.
func (e *CollabEditor) Mount(ctx context.Context, params core.Params, session core.Session) error {
	// Generate user info
	e.UserID = fmt.Sprintf("user_%d", time.Now().UnixNano())
	e.UserName = fmt.Sprintf("Guest%d", time.Now().Unix()%1000)

	editorRoomsMu.Lock()
	e.UserColor = editorColors[editorColorIdx%len(editorColors)]
	editorColorIdx++

	// Get or create room
	roomID := params.GetDefault("room", "default")
	e.RoomID = roomID

	room, exists := editorRooms[roomID]
	if !exists {
		room = &EditorRoom{
			ID:        roomID,
			Content:   "",
			Users:     make(map[string]*EditorUser),
			Versions:  []DocumentVersion{},
			LastSaved: time.Now(),
		}
		editorRooms[roomID] = room
	}

	// Join room
	room.Users[e.UserID] = &EditorUser{
		ID:         e.UserID,
		Name:       e.UserName,
		Color:      e.UserColor,
		CursorPos:  0,
		LastActive: time.Now(),
	}

	e.LocalContent = room.Content
	e.LastSaved = room.LastSaved
	editorRoomsMu.Unlock()

	editorUsers.Add(1)

	return nil
}

// Terminate handles cleanup.
func (e *CollabEditor) Terminate(ctx context.Context, reason core.TerminateReason) error {
	editorRoomsMu.Lock()
	if room, exists := editorRooms[e.RoomID]; exists {
		delete(room.Users, e.UserID)
	}
	editorRoomsMu.Unlock()

	editorUsers.Add(-1)
	return nil
}

// HandleEvent handles user interactions.
func (e *CollabEditor) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	editorRoomsMu.Lock()
	room := editorRooms[e.RoomID]
	editorRoomsMu.Unlock()

	if room == nil {
		return nil
	}

	switch event {
	case "update_content":
		if val, ok := payload["value"].(string); ok {
			editorRoomsMu.Lock()
			room.Content = val
			room.updateCounts()
			e.LocalContent = val
			e.IsTyping = true
			if user, ok := room.Users[e.UserID]; ok {
				user.IsTyping = true
				user.LastActive = time.Now()
			}
			editorRoomsMu.Unlock()
		}

	case "cursor_move":
		if pos, ok := payload["position"].(float64); ok {
			editorRoomsMu.Lock()
			e.CursorPos = int(pos)
			if user, ok := room.Users[e.UserID]; ok {
				user.CursorPos = e.CursorPos
				user.LastActive = time.Now()
			}
			editorRoomsMu.Unlock()
		}

	case "stop_typing":
		editorRoomsMu.Lock()
		e.IsTyping = false
		if user, ok := room.Users[e.UserID]; ok {
			user.IsTyping = false
		}
		editorRoomsMu.Unlock()

	case "save":
		editorRoomsMu.Lock()
		room.LastSaved = time.Now()
		e.LastSaved = room.LastSaved

		// Save version
		version := DocumentVersion{
			ID:        fmt.Sprintf("v%d", len(room.Versions)+1),
			Content:   room.Content,
			Author:    e.UserName,
			Timestamp: room.LastSaved,
		}
		room.Versions = append(room.Versions, version)

		// Keep only last 10 versions
		if len(room.Versions) > 10 {
			room.Versions = room.Versions[len(room.Versions)-10:]
		}
		editorRoomsMu.Unlock()

	case "toggle_autosave":
		e.AutoSave = !e.AutoSave

	case "set_name":
		if val, ok := payload["value"].(string); ok && len(val) > 0 && len(val) <= 20 {
			e.UserName = val
			editorRoomsMu.Lock()
			if user, ok := room.Users[e.UserID]; ok {
				user.Name = val
			}
			editorRoomsMu.Unlock()
		}

	case "share":
		e.ShowShareModal = true

	case "close_share":
		e.ShowShareModal = false

	case "new_room":
		// Create a new room
		newID := fmt.Sprintf("room_%d", time.Now().UnixNano()%100000)
		editorRoomsMu.Lock()

		// Leave current room
		if oldRoom, exists := editorRooms[e.RoomID]; exists {
			delete(oldRoom.Users, e.UserID)
		}

		// Create new room
		newRoom := &EditorRoom{
			ID:        newID,
			Content:   "",
			Users:     make(map[string]*EditorUser),
			Versions:  []DocumentVersion{},
			LastSaved: time.Now(),
		}
		editorRooms[newID] = newRoom

		// Join new room
		newRoom.Users[e.UserID] = &EditorUser{
			ID:         e.UserID,
			Name:       e.UserName,
			Color:      e.UserColor,
			CursorPos:  0,
			LastActive: time.Now(),
		}

		e.RoomID = newID
		e.LocalContent = ""
		e.LastSaved = newRoom.LastSaved
		editorRoomsMu.Unlock()

	case "restore_version":
		if versionID, ok := payload["version"].(string); ok {
			editorRoomsMu.Lock()
			for _, v := range room.Versions {
				if v.ID == versionID {
					room.Content = v.Content
					room.updateCounts()
					e.LocalContent = v.Content
					break
				}
			}
			editorRoomsMu.Unlock()
		}
	}

	// Auto-save after content changes
	if e.AutoSave && event == "update_content" {
		editorRoomsMu.Lock()
		if time.Since(room.LastSaved) > 5*time.Second {
			room.LastSaved = time.Now()
			e.LastSaved = room.LastSaved
		}
		editorRoomsMu.Unlock()
	}

	return nil
}

// Render returns the HTML representation.
func (e *CollabEditor) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := e.renderEditor()
		_, err := w.Write([]byte(html))
		return err
	})
}

// renderEditor generates the complete editor HTML
func (e *CollabEditor) renderEditor() string {
	cfg := website.PageConfig{
		Title:       "Collaborative Editor - GoliveKit Demo",
		Description: "Real-time collaborative text editor with multi-cursor support.",
		URL:         "https://golivekit.cloud/demos/editor",
		Keywords:    []string{"editor", "collaborative", "realtime", "liveview"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	body := e.renderEditorBody()
	return website.RenderDocument(cfg, renderEditorStyles(), body)
}

// renderEditorStyles returns custom CSS
func renderEditorStyles() string {
	return `
<style>
.editor-container {
	max-width: 1000px;
	margin: 0 auto;
	padding: 1.5rem;
}

.editor-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 1rem;
	flex-wrap: wrap;
	gap: 1rem;
}

.editor-title {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.editor-title h1 {
	font-size: 1.5rem;
	margin: 0;
}

.room-badge {
	font-size: 0.75rem;
	padding: 0.25rem 0.5rem;
	background: var(--color-bgAlt);
	border-radius: 0.25rem;
	color: var(--color-textMuted);
	font-family: monospace;
}

.editor-actions {
	display: flex;
	gap: 0.5rem;
}

.editor-btn {
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

.editor-btn:hover {
	border-color: var(--color-primary);
}

.editor-btn-primary {
	background: var(--color-primary);
	border-color: var(--color-primary);
	color: white;
}

.editor-btn-primary:hover {
	background: #7c3aed;
}

.collaborators {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	margin-bottom: 1rem;
}

.collaborators-label {
	font-size: 0.875rem;
	color: var(--color-textMuted);
}

.collaborator-avatars {
	display: flex;
}

.collaborator-avatar {
	width: 32px;
	height: 32px;
	border-radius: 50%;
	display: flex;
	align-items: center;
	justify-content: center;
	color: white;
	font-weight: 700;
	font-size: 0.75rem;
	margin-left: -8px;
	border: 2px solid var(--color-bg);
	position: relative;
}

.collaborator-avatar:first-child {
	margin-left: 0;
}

.collaborator-avatar .typing-indicator {
	position: absolute;
	bottom: -2px;
	right: -2px;
	width: 10px;
	height: 10px;
	background: var(--color-success);
	border-radius: 50%;
	border: 2px solid var(--color-bg);
	animation: pulse 1s infinite;
}

@keyframes pulse {
	0%, 100% { opacity: 1; }
	50% { opacity: 0.5; }
}

.editor-main {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
	overflow: hidden;
}

.editor-toolbar {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	padding: 0.5rem 1rem;
	border-bottom: 1px solid var(--color-border);
	background: var(--color-bg);
}

.toolbar-btn {
	width: 32px;
	height: 32px;
	border: none;
	background: transparent;
	border-radius: 4px;
	cursor: pointer;
	display: flex;
	align-items: center;
	justify-content: center;
	transition: all 0.2s;
}

.toolbar-btn:hover {
	background: var(--color-bgAlt);
}

.toolbar-sep {
	width: 1px;
	height: 24px;
	background: var(--color-border);
}

.editor-area {
	position: relative;
}

.editor-textarea {
	width: 100%;
	min-height: 400px;
	padding: 1.5rem;
	border: none;
	background: transparent;
	color: var(--color-text);
	font-family: 'JetBrains Mono', 'Fira Code', monospace;
	font-size: 0.9375rem;
	line-height: 1.6;
	resize: vertical;
}

.editor-textarea:focus {
	outline: none;
}

.editor-footer {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 0.75rem 1rem;
	border-top: 1px solid var(--color-border);
	font-size: 0.8125rem;
	color: var(--color-textMuted);
}

.editor-stats {
	display: flex;
	gap: 1.5rem;
}

.stat-item {
	display: flex;
	align-items: center;
	gap: 0.375rem;
}

.save-status {
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.save-indicator {
	width: 8px;
	height: 8px;
	border-radius: 50%;
}

.save-indicator.saved {
	background: var(--color-success);
}

.save-indicator.saving {
	background: #fbbf24;
	animation: pulse 1s infinite;
}

.sidebar-panel {
	margin-top: 1.5rem;
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
	padding: 1rem;
}

.panel-title {
	font-size: 0.875rem;
	font-weight: 600;
	margin-bottom: 0.75rem;
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.version-list {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
	max-height: 200px;
	overflow-y: auto;
}

.version-item {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 0.5rem;
	background: var(--color-bg);
	border-radius: 0.375rem;
	font-size: 0.8125rem;
}

.version-info {
	display: flex;
	flex-direction: column;
}

.version-time {
	color: var(--color-textMuted);
	font-size: 0.75rem;
}

.version-author {
	font-weight: 500;
}

.restore-btn {
	padding: 0.25rem 0.5rem;
	border: 1px solid var(--color-border);
	border-radius: 0.25rem;
	background: transparent;
	cursor: pointer;
	font-size: 0.75rem;
	transition: all 0.2s;
}

.restore-btn:hover {
	border-color: var(--color-primary);
	color: var(--color-primary);
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
	min-width: 400px;
	max-width: 90%;
}

.modal-title {
	font-size: 1.125rem;
	font-weight: 600;
	margin-bottom: 1rem;
}

.share-url {
	display: flex;
	gap: 0.5rem;
	margin-bottom: 1rem;
}

.share-input {
	flex: 1;
	padding: 0.75rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	color: var(--color-text);
	font-family: monospace;
	font-size: 0.875rem;
}

.copy-btn {
	padding: 0.75rem 1rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	cursor: pointer;
}

.copy-btn:hover {
	border-color: var(--color-primary);
}

.modal-actions {
	display: flex;
	justify-content: flex-end;
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

.autosave-toggle {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	cursor: pointer;
}

.toggle-switch {
	width: 36px;
	height: 20px;
	background: var(--color-border);
	border-radius: 10px;
	position: relative;
	transition: all 0.2s;
}

.toggle-switch.active {
	background: var(--color-primary);
}

.toggle-switch::after {
	content: '';
	position: absolute;
	width: 16px;
	height: 16px;
	background: white;
	border-radius: 50%;
	top: 2px;
	left: 2px;
	transition: all 0.2s;
}

.toggle-switch.active::after {
	left: 18px;
}

.name-input {
	padding: 0.5rem;
	border: 1px solid var(--color-border);
	border-radius: 0.375rem;
	background: var(--color-bg);
	color: var(--color-text);
	font-size: 0.875rem;
	width: 150px;
}

.name-input:focus {
	outline: none;
	border-color: var(--color-primary);
}
</style>
`
}

// renderEditorBody generates the main content
func (e *CollabEditor) renderEditorBody() string {
	// Navbar
	navbar := components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "Editor Demo",
		Links: []website.NavLink{
			{Label: "Demos", URL: "/demos", External: false},
			{Label: "Docs", URL: "/docs", External: false},
		},
	})

	editorRoomsMu.RLock()
	room := editorRooms[e.RoomID]
	content := ""
	wordCount := 0
	charCount := 0
	collaborators := ""
	versions := ""

	if room != nil {
		content = room.Content
		wordCount = room.WordCount
		charCount = room.CharCount
		collaborators = e.renderCollaborators(room)
		versions = e.renderVersions(room)
	}
	editorRoomsMu.RUnlock()

	// Save status
	saveStatus := "Saved"
	saveClass := "saved"
	if time.Since(e.LastSaved) < 2*time.Second {
		saveStatus = "Saved"
		saveClass = "saved"
	}

	// Autosave toggle
	autosaveClass := ""
	if e.AutoSave {
		autosaveClass = "active"
	}

	mainContent := fmt.Sprintf(`
<main id="main-content">
<div class="editor-container" data-live-view="collab-editor">

<a href="/demos" class="back-link">← Back to Demos</a>

<div class="editor-header">
	<div class="editor-title">
		<span style="font-size:1.5rem">✍️</span>
		<h1>Collaborative Editor</h1>
		<span class="room-badge">Room: %s</span>
	</div>
	<div class="editor-actions">
		<button class="editor-btn" lv-click="share">🔗 Share</button>
		<button class="editor-btn" lv-click="new_room">+ New Room</button>
		<button class="editor-btn editor-btn-primary" lv-click="save">💾 Save</button>
	</div>
</div>

<div class="collaborators" data-slot="collaborators">
	<span class="collaborators-label">Collaborators:</span>
	%s
</div>

<div class="editor-main">
	<div class="editor-toolbar">
		<button class="toolbar-btn" title="Bold">B</button>
		<button class="toolbar-btn" title="Italic"><em>I</em></button>
		<button class="toolbar-btn" title="Heading">H</button>
		<span class="toolbar-sep"></span>
		<button class="toolbar-btn" title="List">•</button>
		<button class="toolbar-btn" title="Code">&lt;/&gt;</button>
		<button class="toolbar-btn" title="Link">🔗</button>
		<span class="toolbar-sep"></span>
		<div style="flex:1"></div>
		<input type="text" class="name-input" placeholder="Your name"
			lv-change="set_name" lv-debounce="300" value="%s">
	</div>

	<div class="editor-area">
		<textarea class="editor-textarea"
			placeholder="Start typing... Changes sync in real-time!"
			lv-change="update_content" lv-debounce="150"
			lv-blur="stop_typing"
			data-slot="content">%s</textarea>
	</div>

	<div class="editor-footer">
		<div class="editor-stats" data-slot="stats">
			<span class="stat-item">📝 %d words</span>
			<span class="stat-item">📊 %d characters</span>
		</div>
		<div class="save-status">
			<div class="autosave-toggle" lv-click="toggle_autosave">
				<div class="toggle-switch %s"></div>
				<span>Auto-save</span>
			</div>
			<span class="stat-item">
				<span class="save-indicator %s"></span>
				%s
			</span>
		</div>
	</div>
</div>

<div class="sidebar-panel">
	<div class="panel-title">📜 Version History</div>
	<div class="version-list" data-slot="versions">
		%s
	</div>
</div>

%s

</div>
</main>

<script src="/_live/golivekit.js"></script>
`, e.RoomID, collaborators, e.UserName, content, wordCount, charCount, autosaveClass, saveClass, saveStatus, versions, e.renderShareModal())

	return navbar + mainContent
}

// renderCollaborators renders the collaborator avatars
func (e *CollabEditor) renderCollaborators(room *EditorRoom) string {
	if room == nil {
		return ""
	}

	var html string
	for _, user := range room.Users {
		initial := string(user.Name[0])
		typingIndicator := ""
		if user.IsTyping {
			typingIndicator = `<span class="typing-indicator"></span>`
		}

		isSelf := ""
		if user.ID == e.UserID {
			isSelf = " (you)"
		}

		html += fmt.Sprintf(`
<div class="collaborator-avatar" style="background:%s" title="%s%s">
	%s
	%s
</div>
`, user.Color, user.Name, isSelf, initial, typingIndicator)
	}

	return `<div class="collaborator-avatars">` + html + `</div>`
}

// renderVersions renders the version history
func (e *CollabEditor) renderVersions(room *EditorRoom) string {
	if room == nil || len(room.Versions) == 0 {
		return `<p style="color:var(--color-textMuted);font-size:0.875rem;text-align:center">No saved versions yet</p>`
	}

	var html string
	// Show in reverse order (newest first)
	for i := len(room.Versions) - 1; i >= 0; i-- {
		v := room.Versions[i]
		html += fmt.Sprintf(`
<div class="version-item">
	<div class="version-info">
		<span class="version-author">%s</span>
		<span class="version-time">%s</span>
	</div>
	<button class="restore-btn" lv-click="restore_version" lv-value-version="%s">Restore</button>
</div>
`, v.Author, v.Timestamp.Format("Jan 02, 15:04"), v.ID)
	}

	return html
}

// renderShareModal renders the share modal
func (e *CollabEditor) renderShareModal() string {
	if !e.ShowShareModal {
		return ""
	}

	shareURL := fmt.Sprintf("https://golivekit.cloud/demos/editor?room=%s", e.RoomID)

	return fmt.Sprintf(`
<div class="modal-overlay" lv-click="close_share">
	<div class="modal" onclick="event.stopPropagation()">
		<div class="modal-title">🔗 Share This Document</div>
		<p style="color:var(--color-textMuted);margin-bottom:1rem;font-size:0.875rem">
			Share this link with others to collaborate in real-time:
		</p>
		<div class="share-url">
			<input type="text" class="share-input" value="%s" readonly>
			<button class="copy-btn" onclick="navigator.clipboard.writeText('%s')">📋 Copy</button>
		</div>
		<div class="modal-actions">
			<button class="editor-btn" lv-click="close_share">Close</button>
		</div>
	</div>
</div>
`, shareURL, shareURL)
}

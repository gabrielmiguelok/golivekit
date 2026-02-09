// Package demos provides demo components for GoliveKit showcase.
package demos

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// UserStatus represents a user's current status
type UserStatus int

const (
	StatusOnline UserStatus = iota
	StatusAway
	StatusTyping
)

// PlaylistUser represents a user in the playlist room
type PlaylistUser struct {
	ID        string
	Name      string
	Color     string
	Status    UserStatus
	JoinedAt  time.Time
	IsHost    bool
	LastSeen  time.Time
}

// Song represents a song in the playlist
type Song struct {
	ID       string
	Title    string
	Artist   string
	Duration string
	AddedBy  string
	Votes    int
	Voters   map[string]bool // UserID -> voted
}

// ChatMessage represents a chat message
type ChatMessage struct {
	ID        string
	UserID    string
	UserName  string
	UserColor string
	Text      string
	Timestamp time.Time
}

// Global playlist state (simulated PubSub)
var (
	playlistUsers    = make(map[string]*PlaylistUser)
	playlistSongs    = []*Song{}
	playlistChat     = []ChatMessage{}
	playlistNowPlaying = 0 // Index of currently playing song
	playlistProgress = 0   // Playback progress in seconds
	playlistMu       sync.RWMutex
	playlistListeners atomic.Int64
	userColors       = []string{"#ef4444", "#f97316", "#eab308", "#22c55e", "#06b6d4", "#3b82f6", "#8b5cf6", "#ec4899"}
	colorIndex       int
)

func init() {
	// Initialize with some demo songs
	playlistSongs = []*Song{
		{ID: "1", Title: "Electric Dreams", Artist: "Synthwave Artist", Duration: "3:45", AddedBy: "Alice", Votes: 5, Voters: map[string]bool{}},
		{ID: "2", Title: "Midnight Run", Artist: "Chillwave Band", Duration: "4:12", AddedBy: "Bob", Votes: 3, Voters: map[string]bool{}},
		{ID: "3", Title: "Neon Lights", Artist: "Retro Collective", Duration: "3:28", AddedBy: "Charlie", Votes: 1, Voters: map[string]bool{}},
	}

	// Start playback simulator
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			playlistMu.Lock()
			if len(playlistSongs) > 0 {
				playlistProgress++
				// Simulate song duration (assume 4 minutes average)
				if playlistProgress >= 240 {
					playlistProgress = 0
					playlistNowPlaying = (playlistNowPlaying + 1) % len(playlistSongs)
				}
			}
			playlistMu.Unlock()
		}
	}()
}

// RealtimePlaylist is the collaborative playlist component.
type RealtimePlaylist struct {
	core.BaseComponent

	// User state
	UserID      string
	UserName    string
	UserColor   string
	JoinedAt    time.Time
	IsTyping    bool
	CurrentTab  string // "queue" or "chat"

	// Form state
	NewSongTitle  string
	NewSongArtist string
	ChatInput     string
}

// NewRealtimePlaylist creates a new realtime playlist component.
func NewRealtimePlaylist() core.Component {
	return &RealtimePlaylist{
		CurrentTab: "queue",
	}
}

// Name returns the component name.
func (p *RealtimePlaylist) Name() string {
	return "realtime-playlist"
}

// Mount initializes the playlist.
func (p *RealtimePlaylist) Mount(ctx context.Context, params core.Params, session core.Session) error {
	// Generate user ID and assign color
	p.UserID = fmt.Sprintf("user_%d", time.Now().UnixNano())
	p.UserName = fmt.Sprintf("Guest%d", time.Now().Unix()%1000)
	p.JoinedAt = time.Now()

	playlistMu.Lock()
	p.UserColor = userColors[colorIndex%len(userColors)]
	colorIndex++

	// First user is host
	isHost := len(playlistUsers) == 0

	playlistUsers[p.UserID] = &PlaylistUser{
		ID:       p.UserID,
		Name:     p.UserName,
		Color:    p.UserColor,
		Status:   StatusOnline,
		JoinedAt: p.JoinedAt,
		IsHost:   isHost,
		LastSeen: time.Now(),
	}
	playlistMu.Unlock()

	playlistListeners.Add(1)

	// Add join message to chat
	p.addSystemMessage(fmt.Sprintf("%s joined the room", p.UserName))

	return nil
}

// Terminate handles cleanup.
func (p *RealtimePlaylist) Terminate(ctx context.Context, reason core.TerminateReason) error {
	playlistMu.Lock()
	delete(playlistUsers, p.UserID)
	playlistMu.Unlock()

	playlistListeners.Add(-1)

	// Add leave message
	p.addSystemMessage(fmt.Sprintf("%s left the room", p.UserName))

	return nil
}

// addSystemMessage adds a system message to chat
func (p *RealtimePlaylist) addSystemMessage(text string) {
	playlistMu.Lock()
	defer playlistMu.Unlock()

	msg := ChatMessage{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		UserID:    "system",
		UserName:  "System",
		UserColor: "#6b7280",
		Text:      text,
		Timestamp: time.Now(),
	}
	playlistChat = append(playlistChat, msg)

	// Keep only last 50 messages
	if len(playlistChat) > 50 {
		playlistChat = playlistChat[len(playlistChat)-50:]
	}
}

// HandleEvent handles user interactions.
func (p *RealtimePlaylist) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	case "vote":
		songID, _ := payload["song_id"].(string)
		direction, _ := payload["direction"].(string)
		p.handleVote(songID, direction)

	case "add_song":
		p.handleAddSong()

	case "update_song_title":
		if val, ok := payload["value"].(string); ok {
			p.NewSongTitle = val
		}

	case "update_song_artist":
		if val, ok := payload["value"].(string); ok {
			p.NewSongArtist = val
		}

	case "send_chat":
		p.handleSendChat()

	case "update_chat":
		if val, ok := payload["value"].(string); ok {
			p.ChatInput = val
			p.updateTypingStatus(len(val) > 0)
		}

	case "set_name":
		if val, ok := payload["value"].(string); ok && len(val) > 0 && len(val) <= 20 {
			oldName := p.UserName
			p.UserName = val
			playlistMu.Lock()
			if user, ok := playlistUsers[p.UserID]; ok {
				user.Name = val
			}
			playlistMu.Unlock()
			p.addSystemMessage(fmt.Sprintf("%s is now known as %s", oldName, val))
		}

	case "switch_tab":
		if tab, ok := payload["tab"].(string); ok {
			p.CurrentTab = tab
		}

	case "remove_song":
		songID, _ := payload["song_id"].(string)
		p.handleRemoveSong(songID)
	}

	return nil
}

// handleVote processes a vote on a song
func (p *RealtimePlaylist) handleVote(songID, direction string) {
	playlistMu.Lock()
	defer playlistMu.Unlock()

	for _, song := range playlistSongs {
		if song.ID == songID {
			if song.Voters == nil {
				song.Voters = make(map[string]bool)
			}

			// Check if user already voted
			if _, voted := song.Voters[p.UserID]; voted {
				return // Already voted
			}

			song.Voters[p.UserID] = true

			if direction == "up" {
				song.Votes++
			} else {
				song.Votes--
			}
			break
		}
	}

	// Resort songs by votes (keep now playing first)
	if len(playlistSongs) > 1 && playlistNowPlaying < len(playlistSongs) {
		nowPlaying := playlistSongs[playlistNowPlaying]
		queue := make([]*Song, 0, len(playlistSongs)-1)
		for i, s := range playlistSongs {
			if i != playlistNowPlaying {
				queue = append(queue, s)
			}
		}
		sort.Slice(queue, func(i, j int) bool {
			return queue[i].Votes > queue[j].Votes
		})
		playlistSongs = append([]*Song{nowPlaying}, queue...)
		playlistNowPlaying = 0
	}
}

// handleAddSong adds a new song to the queue
func (p *RealtimePlaylist) handleAddSong() {
	if p.NewSongTitle == "" || p.NewSongArtist == "" {
		return
	}

	playlistMu.Lock()
	newSong := &Song{
		ID:       fmt.Sprintf("song_%d", time.Now().UnixNano()),
		Title:    p.NewSongTitle,
		Artist:   p.NewSongArtist,
		Duration: "3:30", // Default duration
		AddedBy:  p.UserName,
		Votes:    0,
		Voters:   make(map[string]bool),
	}
	playlistSongs = append(playlistSongs, newSong)
	playlistMu.Unlock()

	p.NewSongTitle = ""
	p.NewSongArtist = ""

	p.addSystemMessage(fmt.Sprintf("%s added \"%s\" to the queue", p.UserName, newSong.Title))
}

// handleRemoveSong removes a song from the queue
func (p *RealtimePlaylist) handleRemoveSong(songID string) {
	playlistMu.Lock()
	defer playlistMu.Unlock()

	for i, song := range playlistSongs {
		if song.ID == songID {
			// Only allow removing if user added it or is host
			if song.AddedBy == p.UserName || (playlistUsers[p.UserID] != nil && playlistUsers[p.UserID].IsHost) {
				playlistSongs = append(playlistSongs[:i], playlistSongs[i+1:]...)
				if playlistNowPlaying >= len(playlistSongs) {
					playlistNowPlaying = 0
				}
			}
			break
		}
	}
}

// handleSendChat sends a chat message
func (p *RealtimePlaylist) handleSendChat() {
	if p.ChatInput == "" {
		return
	}

	playlistMu.Lock()
	msg := ChatMessage{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		UserID:    p.UserID,
		UserName:  p.UserName,
		UserColor: p.UserColor,
		Text:      p.ChatInput,
		Timestamp: time.Now(),
	}
	playlistChat = append(playlistChat, msg)

	// Keep only last 50 messages
	if len(playlistChat) > 50 {
		playlistChat = playlistChat[len(playlistChat)-50:]
	}
	playlistMu.Unlock()

	p.ChatInput = ""
	p.updateTypingStatus(false)
}

// updateTypingStatus updates the user's typing status
func (p *RealtimePlaylist) updateTypingStatus(isTyping bool) {
	if p.IsTyping == isTyping {
		return
	}
	p.IsTyping = isTyping

	playlistMu.Lock()
	if user, ok := playlistUsers[p.UserID]; ok {
		if isTyping {
			user.Status = StatusTyping
		} else {
			user.Status = StatusOnline
		}
		user.LastSeen = time.Now()
	}
	playlistMu.Unlock()
}

// Render returns the HTML representation.
func (p *RealtimePlaylist) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := p.renderPlaylist()
		_, err := w.Write([]byte(html))
		return err
	})
}

// renderPlaylist generates the complete playlist HTML
func (p *RealtimePlaylist) renderPlaylist() string {
	cfg := website.PageConfig{
		Title:       "Collaborative Playlist - GoliveKit Demo",
		Description: "Real-time collaborative playlist with voting and live chat.",
		URL:         "https://golivekit.cloud/demos/realtime",
		Keywords:    []string{"playlist", "realtime", "pubsub", "liveview"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	body := p.renderPlaylistBody()
	return website.RenderDocument(cfg, renderPlaylistStyles(), body)
}

// renderPlaylistStyles returns custom CSS
func renderPlaylistStyles() string {
	return `
<style>
.playlist-container {
	max-width: 1100px;
	margin: 0 auto;
	padding: 2rem;
}

.playlist-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 1.5rem;
	flex-wrap: wrap;
	gap: 1rem;
}

.playlist-title {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.playlist-title h1 {
	font-size: 1.75rem;
	margin: 0;
}

.listeners-badge {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	background: var(--color-bgAlt);
	padding: 0.5rem 1rem;
	border-radius: 2rem;
	font-size: 0.875rem;
}

.listeners-badge .dot {
	width: 8px;
	height: 8px;
	background: var(--color-success);
	border-radius: 50%;
	animation: pulse 2s infinite;
}

@keyframes pulse {
	0%, 100% { opacity: 1; }
	50% { opacity: 0.5; }
}

.playlist-content {
	display: grid;
	grid-template-columns: 280px 1fr;
	gap: 1.5rem;
}

@media (max-width: 900px) {
	.playlist-content {
		grid-template-columns: 1fr;
	}
}

.sidebar-panel {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 1rem;
	padding: 1rem;
	display: flex;
	flex-direction: column;
	gap: 1rem;
}

.main-panel {
	display: flex;
	flex-direction: column;
	gap: 1.5rem;
}

.now-playing {
	background: linear-gradient(135deg, var(--color-primary) 0%, #7c3aed 100%);
	border-radius: 1rem;
	padding: 1.5rem;
	color: white;
}

.now-playing-label {
	font-size: 0.75rem;
	text-transform: uppercase;
	letter-spacing: 0.1em;
	opacity: 0.8;
	margin-bottom: 0.5rem;
}

.now-playing-title {
	font-size: 1.5rem;
	font-weight: 700;
	margin: 0 0 0.25rem 0;
}

.now-playing-artist {
	opacity: 0.9;
	margin-bottom: 1rem;
}

.progress-bar {
	height: 4px;
	background: rgba(255,255,255,0.3);
	border-radius: 2px;
	overflow: hidden;
	margin-bottom: 0.5rem;
}

.progress-fill {
	height: 100%;
	background: white;
	transition: width 1s linear;
}

.progress-time {
	display: flex;
	justify-content: space-between;
	font-size: 0.75rem;
	opacity: 0.8;
}

.panel-card {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 1rem;
	overflow: hidden;
}

.panel-tabs {
	display: flex;
	border-bottom: 1px solid var(--color-border);
}

.panel-tab {
	flex: 1;
	padding: 0.75rem;
	text-align: center;
	cursor: pointer;
	border: none;
	background: none;
	color: var(--color-textMuted);
	font-weight: 500;
	transition: all 0.2s;
}

.panel-tab:hover {
	color: var(--color-text);
}

.panel-tab.active {
	color: var(--color-primary);
	background: var(--color-bg);
	border-bottom: 2px solid var(--color-primary);
}

.panel-content {
	padding: 1rem;
	max-height: 400px;
	overflow-y: auto;
}

.queue-item {
	display: flex;
	align-items: center;
	gap: 1rem;
	padding: 0.75rem;
	border-radius: 0.5rem;
	transition: background 0.2s;
}

.queue-item:hover {
	background: var(--color-bg);
}

.queue-position {
	width: 24px;
	text-align: center;
	color: var(--color-textMuted);
	font-weight: 600;
}

.queue-info {
	flex: 1;
	min-width: 0;
}

.queue-title {
	font-weight: 600;
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

.queue-meta {
	font-size: 0.75rem;
	color: var(--color-textMuted);
}

.vote-buttons {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 0.25rem;
}

.vote-btn {
	width: 28px;
	height: 28px;
	border: 1px solid var(--color-border);
	background: var(--color-bg);
	border-radius: 4px;
	cursor: pointer;
	font-size: 0.75rem;
	display: flex;
	align-items: center;
	justify-content: center;
	transition: all 0.2s;
}

.vote-btn:hover {
	border-color: var(--color-primary);
	color: var(--color-primary);
}

.vote-count {
	font-weight: 700;
	color: var(--color-primary);
	font-size: 0.875rem;
}

.chat-messages {
	display: flex;
	flex-direction: column;
	gap: 0.75rem;
}

.chat-message {
	display: flex;
	gap: 0.5rem;
}

.chat-avatar {
	width: 32px;
	height: 32px;
	border-radius: 50%;
	display: flex;
	align-items: center;
	justify-content: center;
	color: white;
	font-weight: 700;
	font-size: 0.75rem;
	flex-shrink: 0;
}

.chat-content {
	flex: 1;
	min-width: 0;
}

.chat-name {
	font-weight: 600;
	font-size: 0.875rem;
}

.chat-text {
	color: var(--color-textMuted);
	font-size: 0.875rem;
	word-break: break-word;
}

.chat-time {
	font-size: 0.625rem;
	color: var(--color-textMuted);
	margin-left: 0.5rem;
}

.chat-system {
	text-align: center;
	color: var(--color-textMuted);
	font-size: 0.75rem;
	font-style: italic;
}

.chat-input-form {
	display: flex;
	gap: 0.5rem;
	padding: 1rem;
	border-top: 1px solid var(--color-border);
}

.chat-input {
	flex: 1;
	padding: 0.5rem 0.75rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	color: var(--color-text);
}

.chat-input:focus {
	outline: none;
	border-color: var(--color-primary);
}

.chat-send {
	padding: 0.5rem 1rem;
	background: var(--color-primary);
	color: white;
	border: none;
	border-radius: 0.5rem;
	cursor: pointer;
	font-weight: 600;
}

.chat-send:hover {
	background: #7c3aed;
}

.users-list {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.user-item {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	padding: 0.5rem;
	border-radius: 0.5rem;
}

.user-status {
	width: 8px;
	height: 8px;
	border-radius: 50%;
}

.user-status.online {
	background: var(--color-success);
}

.user-status.away {
	background: #fbbf24;
}

.user-status.typing {
	background: var(--color-primary);
	animation: pulse 1s infinite;
}

.user-name {
	flex: 1;
	font-size: 0.875rem;
}

.user-badge {
	font-size: 0.625rem;
	padding: 0.125rem 0.375rem;
	background: var(--color-primary);
	color: white;
	border-radius: 0.25rem;
}

.add-song-form {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.form-row {
	display: flex;
	gap: 0.5rem;
}

.form-input {
	flex: 1;
	padding: 0.5rem 0.75rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	color: var(--color-text);
	font-size: 0.875rem;
}

.form-input:focus {
	outline: none;
	border-color: var(--color-primary);
}

.form-btn {
	padding: 0.5rem 1rem;
	background: var(--color-primary);
	color: white;
	border: none;
	border-radius: 0.5rem;
	cursor: pointer;
	font-weight: 600;
	font-size: 0.875rem;
}

.form-btn:hover {
	background: #7c3aed;
}

.section-title {
	font-size: 0.75rem;
	text-transform: uppercase;
	letter-spacing: 0.05em;
	color: var(--color-textMuted);
	margin-bottom: 0.5rem;
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

.typing-indicator {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	font-style: italic;
	padding: 0.5rem 1rem;
	border-top: 1px solid var(--color-border);
}
</style>
`
}

// renderPlaylistBody generates the main content
func (p *RealtimePlaylist) renderPlaylistBody() string {
	// Navbar
	navbar := components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "Realtime Demo",
		Links: []website.NavLink{
			{Label: "Demos", URL: "/demos", External: false},
			{Label: "Docs", URL: "/docs", External: false},
		},
	})

	listeners := playlistListeners.Load()

	playlistMu.RLock()
	nowPlaying := p.renderNowPlaying()
	queue := p.renderQueue()
	chat := p.renderChat()
	users := p.renderUsers()
	typingUsers := p.getTypingUsers()
	playlistMu.RUnlock()

	typingIndicator := ""
	if len(typingUsers) > 0 {
		typingIndicator = fmt.Sprintf(`<div class="typing-indicator">%s</div>`, typingUsers)
	}

	content := fmt.Sprintf(`
<main id="main-content">
<div class="playlist-container" data-live-view="realtime-playlist">

<a href="/demos" class="back-link">← Back to Demos</a>

<div class="playlist-header">
	<div class="playlist-title">
		<span style="font-size:2rem">🎵</span>
		<h1>Collaborative Playlist</h1>
	</div>
	<div class="listeners-badge">
		<span class="dot"></span>
		<span data-slot="listeners">%d</span> listeners
	</div>
</div>

<div class="playlist-content">
	<div class="sidebar-panel">
		<div>
			<div class="section-title">Users Online</div>
			<div class="users-list" data-slot="users">
				%s
			</div>
		</div>

		<div>
			<div class="section-title">Add Song</div>
			<div class="add-song-form">
				<input type="text" class="form-input" placeholder="Song title"
					lv-change="update_song_title" lv-debounce="150" value="%s">
				<input type="text" class="form-input" placeholder="Artist"
					lv-change="update_song_artist" lv-debounce="150" value="%s">
				<button class="form-btn" lv-click="add_song">+ Add to Queue</button>
			</div>
		</div>

		<div>
			<div class="section-title">Your Name</div>
			<input type="text" class="form-input" placeholder="Enter name"
				lv-change="set_name" lv-debounce="300" value="%s" maxlength="20">
		</div>
	</div>

	<div class="main-panel">
		%s

		<div class="panel-card">
			<div class="panel-tabs">
				<button class="panel-tab %s" lv-click="switch_tab" lv-value-tab="queue">
					Queue (%d)
				</button>
				<button class="panel-tab %s" lv-click="switch_tab" lv-value-tab="chat">
					Chat
				</button>
			</div>

			<div class="panel-content" data-slot="panel-content">
				%s
			</div>

			%s
			%s
		</div>
	</div>
</div>

</div>
</main>

<script src="/_live/golivekit.js"></script>
`, listeners, users, p.NewSongTitle, p.NewSongArtist, p.UserName, nowPlaying,
		p.tabClass("queue"), len(playlistSongs), p.tabClass("chat"),
		p.renderTabContent(queue, chat), typingIndicator, p.renderChatInput())

	return navbar + content
}

// tabClass returns active class if tab matches current
func (p *RealtimePlaylist) tabClass(tab string) string {
	if p.CurrentTab == tab {
		return "active"
	}
	return ""
}

// renderTabContent returns content for active tab
func (p *RealtimePlaylist) renderTabContent(queue, chat string) string {
	if p.CurrentTab == "chat" {
		return chat
	}
	return queue
}

// renderChatInput returns chat input form (only for chat tab)
func (p *RealtimePlaylist) renderChatInput() string {
	if p.CurrentTab != "chat" {
		return ""
	}
	return fmt.Sprintf(`
<div class="chat-input-form">
	<input type="text" class="chat-input" placeholder="Type a message..."
		lv-change="update_chat" lv-debounce="100" value="%s">
	<button class="chat-send" lv-click="send_chat">Send</button>
</div>
`, p.ChatInput)
}

// renderNowPlaying renders the now playing card
func (p *RealtimePlaylist) renderNowPlaying() string {
	if len(playlistSongs) == 0 {
		return `
<div class="now-playing">
	<div class="now-playing-label">Now Playing</div>
	<div class="now-playing-title">No songs in queue</div>
	<div class="now-playing-artist">Add a song to get started!</div>
</div>
`
	}

	song := playlistSongs[playlistNowPlaying]
	progressPct := float64(playlistProgress) / 240.0 * 100.0
	currentMin := playlistProgress / 60
	currentSec := playlistProgress % 60

	return fmt.Sprintf(`
<div class="now-playing" data-slot="now-playing">
	<div class="now-playing-label">Now Playing</div>
	<div class="now-playing-title">%s</div>
	<div class="now-playing-artist">%s</div>
	<div class="progress-bar">
		<div class="progress-fill" style="width: %.1f%%"></div>
	</div>
	<div class="progress-time">
		<span>%d:%02d</span>
		<span>%s</span>
	</div>
</div>
`, song.Title, song.Artist, progressPct, currentMin, currentSec, song.Duration)
}

// renderQueue renders the song queue
func (p *RealtimePlaylist) renderQueue() string {
	if len(playlistSongs) == 0 {
		return `<p style="text-align:center;color:var(--color-textMuted)">Queue is empty. Add some songs!</p>`
	}

	var html string
	for i, song := range playlistSongs {
		if i == playlistNowPlaying {
			continue // Skip now playing
		}

		position := i
		if i > playlistNowPlaying {
			position = i - 1
		}

		html += fmt.Sprintf(`
<div class="queue-item">
	<div class="queue-position">%d</div>
	<div class="queue-info">
		<div class="queue-title">%s</div>
		<div class="queue-meta">%s • Added by %s</div>
	</div>
	<div class="vote-buttons">
		<button class="vote-btn" lv-click="vote" lv-value-song_id="%s" lv-value-direction="up">▲</button>
		<span class="vote-count">%d</span>
		<button class="vote-btn" lv-click="vote" lv-value-song_id="%s" lv-value-direction="down">▼</button>
	</div>
</div>
`, position+1, song.Title, song.Artist, song.AddedBy, song.ID, song.Votes, song.ID)
	}

	if html == "" {
		return `<p style="text-align:center;color:var(--color-textMuted)">No songs in queue</p>`
	}

	return html
}

// renderChat renders chat messages
func (p *RealtimePlaylist) renderChat() string {
	if len(playlistChat) == 0 {
		return `<p style="text-align:center;color:var(--color-textMuted)">No messages yet. Say hello!</p>`
	}

	var html string
	for _, msg := range playlistChat {
		if msg.UserID == "system" {
			html += fmt.Sprintf(`<div class="chat-system">%s</div>`, msg.Text)
		} else {
			initial := string(msg.UserName[0])
			timeStr := msg.Timestamp.Format("15:04")
			html += fmt.Sprintf(`
<div class="chat-message">
	<div class="chat-avatar" style="background:%s">%s</div>
	<div class="chat-content">
		<span class="chat-name" style="color:%s">%s</span>
		<span class="chat-time">%s</span>
		<div class="chat-text">%s</div>
	</div>
</div>
`, msg.UserColor, initial, msg.UserColor, msg.UserName, timeStr, msg.Text)
		}
	}

	return `<div class="chat-messages">` + html + `</div>`
}

// renderUsers renders the users list
func (p *RealtimePlaylist) renderUsers() string {
	var html string
	for _, user := range playlistUsers {
		statusClass := "online"
		if user.Status == StatusAway {
			statusClass = "away"
		} else if user.Status == StatusTyping {
			statusClass = "typing"
		}

		badge := ""
		if user.IsHost {
			badge = `<span class="user-badge">host</span>`
		}

		isSelf := ""
		if user.ID == p.UserID {
			isSelf = " (you)"
		}

		html += fmt.Sprintf(`
<div class="user-item">
	<span class="user-status %s"></span>
	<span class="user-name" style="color:%s">%s%s</span>
	%s
</div>
`, statusClass, user.Color, user.Name, isSelf, badge)
	}

	return html
}

// getTypingUsers returns a string of users currently typing
func (p *RealtimePlaylist) getTypingUsers() string {
	var typing []string
	for _, user := range playlistUsers {
		if user.Status == StatusTyping && user.ID != p.UserID {
			typing = append(typing, user.Name)
		}
	}

	if len(typing) == 0 {
		return ""
	} else if len(typing) == 1 {
		return typing[0] + " is typing..."
	} else if len(typing) == 2 {
		return typing[0] + " and " + typing[1] + " are typing..."
	}
	return fmt.Sprintf("%s and %d others are typing...", typing[0], len(typing)-1)
}

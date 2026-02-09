// Package demos provides demo components for GoliveKit showcase.
package demos

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// Direction constants for snake movement
type Direction int

const (
	DirUp Direction = iota
	DirDown
	DirLeft
	DirRight
)

// Point represents a coordinate on the game grid
type Point struct {
	X, Y int
}

// GameState represents the current state of the snake game
type GameState int

const (
	StateMenu GameState = iota
	StatePlaying
	StatePaused
	StateGameOver
)

// Leaderboard entry
type LeaderboardEntry struct {
	Name  string
	Score int
}

// Global leaderboard (shared across sessions)
var (
	globalLeaderboard = []LeaderboardEntry{
		{Name: "Alice", Score: 234},
		{Name: "Bob", Score: 189},
		{Name: "Charlie", Score: 156},
		{Name: "Diana", Score: 142},
		{Name: "Eve", Score: 98},
	}
	leaderboardMu sync.RWMutex
	gamePlayers   atomic.Int64
)

// SnakeGame is the Snake game component - runs entirely on the server.
type SnakeGame struct {
	core.BaseComponent

	// Game state
	State     GameState
	Snake     []Point
	Food      Point
	Direction Direction
	NextDir   Direction // Buffer for next direction (prevents 180° turns)
	Score     int
	HighScore int
	Speed     int // 1-10, affects tick rate
	GridW     int
	GridH     int

	// Timing
	LastTick  time.Time
	TickRate  time.Duration
	StartTime time.Time

	// Player info
	PlayerName string
}

// NewSnakeGame creates a new snake game component.
func NewSnakeGame() core.Component {
	return &SnakeGame{
		GridW: 20,
		GridH: 15,
		Speed: 5,
	}
}

// Name returns the component name.
func (g *SnakeGame) Name() string {
	return "snake-game"
}

// Mount initializes the game.
func (g *SnakeGame) Mount(ctx context.Context, params core.Params, session core.Session) error {
	g.State = StateMenu
	g.HighScore = 0
	g.PlayerName = "Player"
	g.Speed = 5
	gamePlayers.Add(1)
	return nil
}

// Terminate handles cleanup.
func (g *SnakeGame) Terminate(ctx context.Context, reason core.TerminateReason) error {
	gamePlayers.Add(-1)
	return nil
}

// initGame initializes a new game
func (g *SnakeGame) initGame() {
	// Initialize snake in the center
	centerX := g.GridW / 2
	centerY := g.GridH / 2
	g.Snake = []Point{
		{X: centerX, Y: centerY},
		{X: centerX - 1, Y: centerY},
		{X: centerX - 2, Y: centerY},
	}
	g.Direction = DirRight
	g.NextDir = DirRight
	g.Score = 0
	g.State = StatePlaying
	g.LastTick = time.Now()
	g.StartTime = time.Now()

	// Set tick rate based on speed (100ms at speed 10, 300ms at speed 1)
	g.TickRate = time.Duration(350-g.Speed*25) * time.Millisecond

	// Place food
	g.placeFood()
}

// placeFood places food at a random empty position
func (g *SnakeGame) placeFood() {
	for {
		g.Food = Point{
			X: rand.Intn(g.GridW),
			Y: rand.Intn(g.GridH),
		}
		// Make sure food doesn't spawn on snake
		valid := true
		for _, p := range g.Snake {
			if p.X == g.Food.X && p.Y == g.Food.Y {
				valid = false
				break
			}
		}
		if valid {
			break
		}
	}
}

// tick advances the game by one frame
func (g *SnakeGame) tick() {
	if g.State != StatePlaying {
		return
	}

	now := time.Now()
	if now.Sub(g.LastTick) < g.TickRate {
		return
	}
	g.LastTick = now

	// Apply buffered direction
	g.Direction = g.NextDir

	// Calculate new head position
	head := g.Snake[0]
	newHead := head

	switch g.Direction {
	case DirUp:
		newHead.Y--
	case DirDown:
		newHead.Y++
	case DirLeft:
		newHead.X--
	case DirRight:
		newHead.X++
	}

	// Check wall collision
	if newHead.X < 0 || newHead.X >= g.GridW || newHead.Y < 0 || newHead.Y >= g.GridH {
		g.gameOver()
		return
	}

	// Check self collision
	for _, p := range g.Snake {
		if p.X == newHead.X && p.Y == newHead.Y {
			g.gameOver()
			return
		}
	}

	// Move snake
	g.Snake = append([]Point{newHead}, g.Snake...)

	// Check food collision
	if newHead.X == g.Food.X && newHead.Y == g.Food.Y {
		g.Score += 10
		g.placeFood()
		// Don't remove tail (snake grows)
	} else {
		// Remove tail
		g.Snake = g.Snake[:len(g.Snake)-1]
	}
}

// gameOver handles game over state
func (g *SnakeGame) gameOver() {
	g.State = StateGameOver

	if g.Score > g.HighScore {
		g.HighScore = g.Score
	}

	// Update global leaderboard
	g.updateLeaderboard()
}

// updateLeaderboard adds score to leaderboard if high enough
func (g *SnakeGame) updateLeaderboard() {
	leaderboardMu.Lock()
	defer leaderboardMu.Unlock()

	// Check if score qualifies
	if len(globalLeaderboard) < 10 || g.Score > globalLeaderboard[len(globalLeaderboard)-1].Score {
		entry := LeaderboardEntry{Name: g.PlayerName, Score: g.Score}

		// Find insertion point
		inserted := false
		for i, e := range globalLeaderboard {
			if g.Score > e.Score {
				// Insert at position i
				globalLeaderboard = append(globalLeaderboard[:i], append([]LeaderboardEntry{entry}, globalLeaderboard[i:]...)...)
				inserted = true
				break
			}
		}
		if !inserted && len(globalLeaderboard) < 10 {
			globalLeaderboard = append(globalLeaderboard, entry)
		}

		// Keep only top 10
		if len(globalLeaderboard) > 10 {
			globalLeaderboard = globalLeaderboard[:10]
		}
	}
}

// HandleEvent handles user interactions.
func (g *SnakeGame) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	case "start":
		g.initGame()

	case "pause":
		if g.State == StatePlaying {
			g.State = StatePaused
		} else if g.State == StatePaused {
			g.State = StatePlaying
			g.LastTick = time.Now()
		}

	case "restart":
		g.initGame()

	case "menu":
		g.State = StateMenu

	case "speed":
		if val, ok := payload["value"].(string); ok {
			switch val {
			case "slower":
				if g.Speed > 1 {
					g.Speed--
				}
			case "faster":
				if g.Speed < 10 {
					g.Speed++
				}
			}
			g.TickRate = time.Duration(350-g.Speed*25) * time.Millisecond
		}

	case "keydown":
		key, _ := payload["key"].(string)
		g.handleKey(key)

	case "tick":
		// Periodic tick from client (using lv-tick or setInterval)
		g.tick()

	case "set_name":
		if name, ok := payload["value"].(string); ok && len(name) > 0 && len(name) <= 20 {
			g.PlayerName = name
		}
	}

	return nil
}

// handleKey processes keyboard input
func (g *SnakeGame) handleKey(key string) {
	if g.State != StatePlaying {
		return
	}

	// Prevent 180-degree turns
	switch key {
	case "ArrowUp", "w", "W":
		if g.Direction != DirDown {
			g.NextDir = DirUp
		}
	case "ArrowDown", "s", "S":
		if g.Direction != DirUp {
			g.NextDir = DirDown
		}
	case "ArrowLeft", "a", "A":
		if g.Direction != DirRight {
			g.NextDir = DirLeft
		}
	case "ArrowRight", "d", "D":
		if g.Direction != DirLeft {
			g.NextDir = DirRight
		}
	case " ": // Space to pause
		if g.State == StatePlaying {
			g.State = StatePaused
		} else if g.State == StatePaused {
			g.State = StatePlaying
			g.LastTick = time.Now()
		}
	}
}

// HandleInfo handles internal messages (for game tick via PubSub or timer)
func (g *SnakeGame) HandleInfo(ctx context.Context, msg any) error {
	if msg == "tick" {
		g.tick()
	}
	return nil
}

// Render returns the HTML representation.
func (g *SnakeGame) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := g.renderGame()
		_, err := w.Write([]byte(html))
		return err
	})
}

// renderGame generates the complete game HTML
func (g *SnakeGame) renderGame() string {
	cfg := website.PageConfig{
		Title:       "Snake Game - GoliveKit Demo",
		Description: "Classic Snake game running entirely on the server with GoliveKit.",
		URL:         "https://golivekit.cloud/demos/game",
		Keywords:    []string{"snake", "game", "liveview", "go"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	body := g.renderGameBody()
	return website.RenderDocument(cfg, renderGameStyles(), body)
}

// renderGameStyles returns custom CSS for the game
func renderGameStyles() string {
	return `
<style>
.game-container {
	max-width: 900px;
	margin: 0 auto;
	padding: 2rem;
}

.game-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 1.5rem;
}

.game-title {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.game-title h1 {
	font-size: 1.75rem;
	margin: 0;
}

.game-scores {
	display: flex;
	gap: 2rem;
}

.score-box {
	text-align: center;
}

.score-label {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	text-transform: uppercase;
	letter-spacing: 0.05em;
}

.score-value {
	font-size: 2rem;
	font-weight: 700;
	color: var(--color-primary);
}

.game-content {
	display: grid;
	grid-template-columns: 1fr 250px;
	gap: 1.5rem;
}

@media (max-width: 768px) {
	.game-content {
		grid-template-columns: 1fr;
	}
}

.game-board-container {
	background: var(--color-bgAlt);
	border: 2px solid var(--color-border);
	border-radius: 1rem;
	padding: 1rem;
}

.game-board {
	display: grid;
	gap: 1px;
	background: var(--color-border);
	border-radius: 0.5rem;
	overflow: hidden;
	aspect-ratio: 20/15;
}

.cell {
	background: var(--color-bg);
	transition: background 0.05s;
}

.cell.snake-head {
	background: var(--color-primary);
	border-radius: 4px;
}

.cell.snake-body {
	background: #7c3aed;
}

.cell.food {
	background: #ef4444;
	border-radius: 50%;
}

.game-controls {
	display: flex;
	justify-content: center;
	gap: 1rem;
	margin-top: 1rem;
}

.game-btn {
	padding: 0.75rem 1.5rem;
	border: none;
	border-radius: 0.5rem;
	font-weight: 600;
	cursor: pointer;
	transition: all 0.2s;
}

.game-btn-primary {
	background: var(--color-primary);
	color: white;
}

.game-btn-primary:hover {
	background: #7c3aed;
}

.game-btn-secondary {
	background: var(--color-bgAlt);
	color: var(--color-text);
	border: 1px solid var(--color-border);
}

.game-btn-secondary:hover {
	border-color: var(--color-primary);
}

.game-sidebar {
	display: flex;
	flex-direction: column;
	gap: 1rem;
}

.sidebar-card {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
	padding: 1rem;
}

.sidebar-card h3 {
	font-size: 0.875rem;
	color: var(--color-textMuted);
	margin: 0 0 0.75rem 0;
	text-transform: uppercase;
	letter-spacing: 0.05em;
}

.leaderboard {
	list-style: none;
	padding: 0;
	margin: 0;
}

.leaderboard li {
	display: flex;
	justify-content: space-between;
	padding: 0.5rem 0;
	border-bottom: 1px solid var(--color-border);
}

.leaderboard li:last-child {
	border-bottom: none;
}

.leaderboard .rank {
	color: var(--color-textMuted);
	width: 1.5rem;
}

.leaderboard .name {
	flex: 1;
}

.leaderboard .score {
	font-weight: 600;
	color: var(--color-primary);
}

.leaderboard .current {
	color: var(--color-success);
}

.speed-control {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.speed-bar {
	flex: 1;
	height: 8px;
	background: var(--color-border);
	border-radius: 4px;
	overflow: hidden;
}

.speed-fill {
	height: 100%;
	background: var(--color-primary);
	transition: width 0.2s;
}

.speed-btn {
	width: 32px;
	height: 32px;
	border-radius: 50%;
	border: 1px solid var(--color-border);
	background: var(--color-bg);
	cursor: pointer;
	display: flex;
	align-items: center;
	justify-content: center;
}

.speed-btn:hover {
	border-color: var(--color-primary);
}

.instructions {
	font-size: 0.875rem;
	color: var(--color-textMuted);
}

.instructions kbd {
	background: var(--color-bg);
	padding: 0.125rem 0.375rem;
	border-radius: 0.25rem;
	border: 1px solid var(--color-border);
	font-family: monospace;
}

.game-overlay {
	position: absolute;
	inset: 0;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	background: rgba(0, 0, 0, 0.8);
	border-radius: 0.5rem;
	color: white;
}

.overlay-title {
	font-size: 2rem;
	margin-bottom: 1rem;
}

.overlay-score {
	font-size: 1.25rem;
	margin-bottom: 1.5rem;
	color: var(--color-primary);
}

.game-board-wrapper {
	position: relative;
}

.back-link {
	display: inline-flex;
	align-items: center;
	gap: 0.5rem;
	color: var(--color-textMuted);
	text-decoration: none;
	transition: color 0.2s;
}

.back-link:hover {
	color: var(--color-primary);
}

.players-online {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	color: var(--color-textMuted);
	font-size: 0.875rem;
}

.players-online .dot {
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
</style>
`
}

// renderGameBody generates the main game content
func (g *SnakeGame) renderGameBody() string {
	// Navbar
	navbar := components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "Game Demo",
		Links: []website.NavLink{
			{Label: "Demos", URL: "/demos", External: false},
			{Label: "Docs", URL: "/docs", External: false},
		},
	})

	players := gamePlayers.Load()

	content := fmt.Sprintf(`
<main id="main-content">
<div class="game-container" data-live-view="snake-game" lv-keydown="keydown" tabindex="0">

<a href="/demos" class="back-link">← Back to Demos</a>

<div class="game-header">
	<div class="game-title">
		<span style="font-size:2rem">🐍</span>
		<h1>Snake Game</h1>
	</div>
	<div class="players-online">
		<span class="dot"></span>
		<span data-slot="players">%d</span> playing now
	</div>
</div>

<div class="game-content">
	<div class="game-board-container">
		<div class="game-board-wrapper">
			<div class="game-board" style="grid-template-columns: repeat(%d, 1fr);" data-slot="board">
				%s
			</div>
			%s
		</div>
		<div class="game-controls">
			%s
		</div>
	</div>

	<div class="game-sidebar">
		<div class="sidebar-card">
			<h3>Score</h3>
			<div class="game-scores">
				<div class="score-box">
					<div class="score-label">Current</div>
					<div class="score-value" data-slot="score">%d</div>
				</div>
				<div class="score-box">
					<div class="score-label">Best</div>
					<div class="score-value" style="color:var(--color-success)" data-slot="high">%d</div>
				</div>
			</div>
		</div>

		<div class="sidebar-card">
			<h3>Speed</h3>
			<div class="speed-control">
				<button class="speed-btn" lv-click="speed" lv-value-value="slower">−</button>
				<div class="speed-bar">
					<div class="speed-fill" style="width: %d%%"></div>
				</div>
				<button class="speed-btn" lv-click="speed" lv-value-value="faster">+</button>
			</div>
		</div>

		<div class="sidebar-card">
			<h3>Leaderboard</h3>
			<ol class="leaderboard" data-slot="leaderboard">
				%s
			</ol>
		</div>

		<div class="sidebar-card">
			<h3>Controls</h3>
			<div class="instructions">
				<p><kbd>↑</kbd> <kbd>↓</kbd> <kbd>←</kbd> <kbd>→</kbd> or <kbd>WASD</kbd> to move</p>
				<p><kbd>Space</kbd> to pause</p>
				<p style="margin-top:1rem;color:var(--color-primary)">🔥 Zero JavaScript!</p>
			</div>
		</div>
	</div>
</div>

</div>
</main>

<script src="/_live/golivekit.js"></script>
<script>
// Auto-tick for game loop (sends tick event periodically)
setInterval(function() {
	if (window.liveSocket && window.liveSocket.isConnected()) {
		window.liveSocket.pushEvent("tick", {});
	}
}, 100);
</script>
`, players, g.GridW, g.renderBoard(), g.renderOverlay(), g.renderControls(), g.Score, g.HighScore, g.Speed*10, g.renderLeaderboard())

	return navbar + content
}

// renderBoard generates the game grid HTML
func (g *SnakeGame) renderBoard() string {
	// Create grid
	grid := make([][]string, g.GridH)
	for y := 0; y < g.GridH; y++ {
		grid[y] = make([]string, g.GridW)
		for x := 0; x < g.GridW; x++ {
			grid[y][x] = "cell"
		}
	}

	// Mark snake
	for i, p := range g.Snake {
		if p.Y >= 0 && p.Y < g.GridH && p.X >= 0 && p.X < g.GridW {
			if i == 0 {
				grid[p.Y][p.X] = "cell snake-head"
			} else {
				grid[p.Y][p.X] = "cell snake-body"
			}
		}
	}

	// Mark food
	if g.Food.Y >= 0 && g.Food.Y < g.GridH && g.Food.X >= 0 && g.Food.X < g.GridW {
		grid[g.Food.Y][g.Food.X] = "cell food"
	}

	// Generate HTML
	var html string
	for y := 0; y < g.GridH; y++ {
		for x := 0; x < g.GridW; x++ {
			html += fmt.Sprintf(`<div class="%s"></div>`, grid[y][x])
		}
	}

	return html
}

// renderOverlay generates overlay for menu/pause/game over
func (g *SnakeGame) renderOverlay() string {
	switch g.State {
	case StateMenu:
		return `
<div class="game-overlay">
	<div class="overlay-title">🐍 Snake Game</div>
	<p style="margin-bottom:1.5rem">100% Server-Side with GoliveKit</p>
	<button class="game-btn game-btn-primary" lv-click="start">Start Game</button>
</div>
`
	case StatePaused:
		return `
<div class="game-overlay">
	<div class="overlay-title">⏸️ Paused</div>
	<button class="game-btn game-btn-primary" lv-click="pause">Resume</button>
</div>
`
	case StateGameOver:
		return fmt.Sprintf(`
<div class="game-overlay">
	<div class="overlay-title">💀 Game Over</div>
	<div class="overlay-score">Score: %d</div>
	<div style="display:flex;gap:1rem">
		<button class="game-btn game-btn-primary" lv-click="restart">Play Again</button>
		<button class="game-btn game-btn-secondary" lv-click="menu">Menu</button>
	</div>
</div>
`, g.Score)
	default:
		return ""
	}
}

// renderControls generates control buttons based on state
func (g *SnakeGame) renderControls() string {
	switch g.State {
	case StatePlaying:
		return `<button class="game-btn game-btn-secondary" lv-click="pause">⏸️ Pause</button>`
	case StatePaused:
		return `
<button class="game-btn game-btn-primary" lv-click="pause">▶️ Resume</button>
<button class="game-btn game-btn-secondary" lv-click="restart">🔄 Restart</button>
`
	default:
		return ""
	}
}

// renderLeaderboard generates the leaderboard HTML
func (g *SnakeGame) renderLeaderboard() string {
	leaderboardMu.RLock()
	defer leaderboardMu.RUnlock()

	var html string
	for i, entry := range globalLeaderboard {
		if i >= 5 {
			break
		}
		rank := i + 1
		suffix := ""
		if rank == 1 {
			suffix = " 🏆"
		}

		// Check if this is the current player's entry
		class := ""
		if entry.Name == g.PlayerName && entry.Score == g.Score {
			class = " current"
		}

		html += fmt.Sprintf(`
<li>
	<span class="rank">%d.</span>
	<span class="name%s">%s%s</span>
	<span class="score">%d</span>
</li>
`, rank, class, entry.Name, suffix, entry.Score)
	}

	return html
}

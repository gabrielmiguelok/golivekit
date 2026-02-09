// Package main provides the demos hub - a central portal for all GoliveKit demos.
package main

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// Demo card info
type DemoCard struct {
	Icon        string
	Title       string
	Subtitle    string
	Description string
	Path        string
	Features    []string
	Complexity  int // 1-5 stars
	Online      int // Users currently online
}

// DemosHub is the central hub component for all demos.
type DemosHub struct {
	core.BaseComponent
	ConnectedAt time.Time
}

// Global counters for hub stats
var hubVisitors atomic.Int64
var totalDemoEvents atomic.Int64

// NewDemosHub creates a new demos hub component.
func NewDemosHub() core.Component {
	return &DemosHub{}
}

// Name returns the component name.
func (h *DemosHub) Name() string {
	return "demos-hub"
}

// Mount initializes the hub.
func (h *DemosHub) Mount(ctx context.Context, params core.Params, session core.Session) error {
	h.ConnectedAt = time.Now()
	hubVisitors.Add(1)
	return nil
}

// Terminate handles cleanup.
func (h *DemosHub) Terminate(ctx context.Context, reason core.TerminateReason) error {
	hubVisitors.Add(-1)
	return nil
}

// HandleEvent handles user interactions.
func (h *DemosHub) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	// Hub is mostly navigation, events tracked for stats
	totalDemoEvents.Add(1)
	return nil
}

// Render returns the HTML representation.
func (h *DemosHub) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		visitors := hubVisitors.Load()
		events := totalDemoEvents.Load()
		uptime := time.Since(h.ConnectedAt).Round(time.Second)

		html := renderDemosHub(visitors, events, uptime.String())
		_, err := w.Write([]byte(html))
		return err
	})
}

// getDemoCards returns all available demo cards.
func getDemoCards() []DemoCard {
	return []DemoCard{
		{
			Icon:        "🎵",
			Title:       "Realtime Playlist",
			Subtitle:    "Collaborative Music",
			Description: "Real-time collaborative playlist with voting, presence tracking, and live chat.",
			Path:        "/demos/realtime",
			Features:    []string{"PubSub", "Presence", "Forms"},
			Complexity:  4,
		},
		{
			Icon:        "📝",
			Title:       "Forms Wizard",
			Subtitle:    "Multi-Step Validation",
			Description: "4-step form wizard with async validation, password strength, and file uploads.",
			Path:        "/demos/forms",
			Features:    []string{"Changesets", "Validation", "CSRF"},
			Complexity:  4,
		},
		{
			Icon:        "📁",
			Title:       "File Manager",
			Subtitle:    "Upload & Manage",
			Description: "Full file manager with drag-drop uploads, chunked transfers, and previews.",
			Path:        "/demos/uploads",
			Features:    []string{"Uploads", "Progress", "Security"},
			Complexity:  5,
		},
		{
			Icon:        "📊",
			Title:       "Live Dashboard",
			Subtitle:    "Real-time Metrics",
			Description: "Live metrics dashboard with streaming data, islands architecture, and charts.",
			Path:        "/demos/dashboard",
			Features:    []string{"Streaming", "Islands", "Metrics"},
			Complexity:  4,
		},
		{
			Icon:        "🐍",
			Title:       "Snake Game",
			Subtitle:    "Zero JavaScript",
			Description: "Classic Snake game running entirely on the server - 60 FPS without JS.",
			Path:        "/demos/game",
			Features:    []string{"Game Loop", "Keyboard", "Leaderboard"},
			Complexity:  3,
		},
		{
			Icon:        "✍️",
			Title:       "Collab Editor",
			Subtitle:    "Real-time Editing",
			Description: "Collaborative text editor with multi-cursor support and auto-save.",
			Path:        "/demos/editor",
			Features:    []string{"PubSub", "Presence", "Sync"},
			Complexity:  5,
		},
		{
			Icon:        "🎯",
			Title:       "Kitchen Sink",
			Subtitle:    "All Features",
			Description: "Comprehensive showcase of all GoliveKit features with benchmarking tools.",
			Path:        "/demos/showcase",
			Features:    []string{"Everything", "Benchmarks", "Metrics"},
			Complexity:  5,
		},
	}
}

// renderDemosHub generates the hub HTML.
func renderDemosHub(visitors, events int64, uptime string) string {
	cfg := website.PageConfig{
		Title:       "GoliveKit Demos - Interactive Showcases",
		Description: "Explore interactive demos showcasing GoliveKit's real-time capabilities.",
		URL:         "https://golivekit.cloud/demos",
		Keywords:    []string{"go", "liveview", "demos", "real-time"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	body := renderHubBody(visitors, events, uptime)
	return website.RenderDocument(cfg, renderHubStyles(), body)
}

// renderHubStyles returns custom CSS for the hub.
func renderHubStyles() string {
	return `
<style>
.demos-grid {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
	gap: 1.5rem;
	padding: 2rem 0;
}

.demo-card {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 1rem;
	padding: 1.5rem;
	transition: all 0.3s ease;
	cursor: pointer;
	text-decoration: none;
	color: inherit;
	display: flex;
	flex-direction: column;
}

.demo-card:hover {
	transform: translateY(-4px);
	box-shadow: 0 12px 40px rgba(139, 92, 246, 0.15);
	border-color: var(--color-primary);
}

.demo-card-header {
	display: flex;
	align-items: flex-start;
	gap: 1rem;
	margin-bottom: 1rem;
}

.demo-icon {
	font-size: 2.5rem;
	width: 60px;
	height: 60px;
	display: flex;
	align-items: center;
	justify-content: center;
	background: var(--color-bg);
	border-radius: 0.75rem;
	flex-shrink: 0;
}

.demo-titles {
	flex: 1;
}

.demo-title {
	font-size: 1.25rem;
	font-weight: 700;
	margin: 0 0 0.25rem 0;
}

.demo-subtitle {
	font-size: 0.875rem;
	color: var(--color-primary);
	margin: 0;
}

.demo-description {
	color: var(--color-textMuted);
	font-size: 0.9rem;
	line-height: 1.6;
	margin-bottom: 1rem;
	flex: 1;
}

.demo-features {
	display: flex;
	flex-wrap: wrap;
	gap: 0.5rem;
	margin-bottom: 1rem;
}

.demo-feature {
	background: var(--color-bg);
	padding: 0.25rem 0.75rem;
	border-radius: 2rem;
	font-size: 0.75rem;
	color: var(--color-textMuted);
}

.demo-footer {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding-top: 1rem;
	border-top: 1px solid var(--color-border);
}

.demo-complexity {
	display: flex;
	gap: 0.125rem;
}

.demo-complexity .star {
	color: #fbbf24;
}

.demo-complexity .star.empty {
	color: var(--color-border);
}

.demo-cta {
	color: var(--color-primary);
	font-weight: 600;
	font-size: 0.875rem;
}

.stats-bar {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
	padding: 1rem 1.5rem;
	display: flex;
	justify-content: center;
	gap: 3rem;
	flex-wrap: wrap;
}

.stat-item {
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.stat-icon {
	font-size: 1.25rem;
}

.stat-value {
	font-weight: 700;
	color: var(--color-primary);
}

.stat-label {
	color: var(--color-textMuted);
	font-size: 0.875rem;
}

.hub-hero {
	text-align: center;
	padding: 4rem 0 2rem;
}

.hub-hero h1 {
	font-size: 2.5rem;
	margin-bottom: 1rem;
}

.hub-hero p {
	color: var(--color-textMuted);
	font-size: 1.125rem;
	max-width: 600px;
	margin: 0 auto;
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
</style>
`
}

// renderHubBody generates the main content.
func renderHubBody(visitors, events int64, uptime string) string {
	// Navbar
	navbar := components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "Demos",
		Links: []website.NavLink{
			{Label: "Home", URL: "/", External: false},
			{Label: "Docs", URL: "/docs", External: false},
		},
	})

	// Hero section
	hero := `
<section class="hub-hero">
	<a href="/" class="back-link">← Back to Home</a>
	<h1>🎪 GoliveKit Demos</h1>
	<p>Explore interactive showcases demonstrating real-time web applications built entirely in Go. Each demo is production-ready and uses actual GoliveKit packages.</p>
</section>
`

	// Stats bar
	statsBar := fmt.Sprintf(`
<div class="container">
	<div class="stats-bar" data-live-view="demos-hub">
		<div class="stat-item">
			<span class="stat-icon">👥</span>
			<span class="stat-value" data-slot="visitors">%d</span>
			<span class="stat-label">visitors</span>
		</div>
		<div class="stat-item">
			<span class="stat-icon">⚡</span>
			<span class="stat-value" data-slot="events">%d</span>
			<span class="stat-label">events/min</span>
		</div>
		<div class="stat-item">
			<span class="stat-icon">⏱️</span>
			<span class="stat-value" data-slot="uptime">%s</span>
			<span class="stat-label">session</span>
		</div>
	</div>
</div>
`, visitors, events, uptime)

	// Demo cards
	cards := getDemoCards()
	cardsHTML := `<section class="section"><div class="container"><div class="demos-grid">`

	for _, card := range cards {
		cardsHTML += renderDemoCard(card)
	}

	cardsHTML += `</div></div></section>`

	// Footer info
	footer := `
<section class="section" style="padding-top:0">
	<div class="container text-center">
		<p style="color:var(--color-textMuted)">
			Each demo showcases specific GoliveKit packages and patterns.<br>
			<a href="/docs" style="color:var(--color-primary)">Read the documentation</a> to learn how to build your own.
		</p>
	</div>
</section>
`

	// GoliveKit script
	script := `<script src="/_live/golivekit.js"></script>`

	return navbar + `<main id="main-content">` + hero + statsBar + cardsHTML + footer + `</main>` + script
}

// renderDemoCard generates HTML for a single demo card.
func renderDemoCard(card DemoCard) string {
	// Render complexity stars
	stars := ""
	for i := 1; i <= 5; i++ {
		if i <= card.Complexity {
			stars += `<span class="star">★</span>`
		} else {
			stars += `<span class="star empty">★</span>`
		}
	}

	// Render features
	features := ""
	for _, f := range card.Features {
		features += fmt.Sprintf(`<span class="demo-feature">%s</span>`, f)
	}

	return fmt.Sprintf(`
<a href="%s" class="demo-card">
	<div class="demo-card-header">
		<div class="demo-icon">%s</div>
		<div class="demo-titles">
			<h3 class="demo-title">%s</h3>
			<p class="demo-subtitle">%s</p>
		</div>
	</div>
	<p class="demo-description">%s</p>
	<div class="demo-features">%s</div>
	<div class="demo-footer">
		<div class="demo-complexity">%s</div>
		<span class="demo-cta">Try it →</span>
	</div>
</a>
`, card.Path, card.Icon, card.Title, card.Subtitle, card.Description, features, stars)
}

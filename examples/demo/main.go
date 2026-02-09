// Package main provides a demo showcasing GoliveKit's real-time capabilities
// with a premium landing page built using pkg/templates.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gabrielmiguelok/golivekit/client"
	"github.com/gabrielmiguelok/golivekit/examples/demo/demos"
	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/internal/website/landing"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/router"
)

// Global visitor counter (shared across all sessions)
var globalVisitors atomic.Int64
var globalClicks atomic.Int64

func main() {
	// Get port from environment (for cloud deployment)
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// Create router
	r := router.New()

	// WebSocket endpoint for LiveView connections (must be registered BEFORE /_live/ prefix handler)
	r.Live("/_live/websocket", NewDemo)

	// Serve GoliveKit client JS
	r.Handle("/_live/", http.StripPrefix("/_live/", client.Handler()))

	// Register LiveView routes
	r.Live("/", NewDemo)
	r.Live("/docs", NewDocs)

	// Demos Hub and individual demos
	r.Live("/demos", NewDemosHub)
	r.Live("/demos/realtime", demos.NewRealtimePlaylist)
	r.Live("/demos/forms", demos.NewFormsWizard)
	r.Live("/demos/uploads", demos.NewFileManager)
	r.Live("/demos/dashboard", demos.NewLiveDashboard)
	r.Live("/demos/game", demos.NewSnakeGame)
	r.Live("/demos/editor", demos.NewCollabEditor)
	r.Live("/demos/showcase", demos.NewKitchenSink)

	// Health check for cloud platforms
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// robots.txt for SEO
	r.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`User-agent: *
Allow: /

Sitemap: https://golivekit.cloud/sitemap.xml
`))
	})

	// sitemap.xml for SEO
	r.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://golivekit.cloud/</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/docs</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.9</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/demos</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/demos/realtime</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/demos/forms</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/demos/uploads</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/demos/dashboard</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/demos/game</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/demos/editor</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>https://golivekit.cloud/demos/showcase</loc>
    <lastmod>2026-02-05</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.7</priority>
  </url>
</urlset>
`))
	})

	log.Printf("⚡ GoliveKit Demo starting at http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// Demo is the main demo component showcasing GoliveKit features.
type Demo struct {
	core.BaseComponent
	LocalCount  int
	ConnectedAt time.Time
}

// NewDemo creates a new demo component.
func NewDemo() core.Component {
	return &Demo{}
}

// Name returns the component name.
func (d *Demo) Name() string {
	return "demo"
}

// Mount initializes the demo.
func (d *Demo) Mount(ctx context.Context, params core.Params, session core.Session) error {
	d.LocalCount = 0
	d.ConnectedAt = time.Now()
	globalVisitors.Add(1)
	return nil
}

// Terminate handles cleanup.
func (d *Demo) Terminate(ctx context.Context, reason core.TerminateReason) error {
	globalVisitors.Add(-1)
	return nil
}

// HandleEvent handles user interactions.
func (d *Demo) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	case "increment":
		d.LocalCount++
		globalClicks.Add(1)
	case "decrement":
		d.LocalCount--
		globalClicks.Add(1)
	case "reset":
		d.LocalCount = 0
		globalClicks.Add(1)
	}
	return nil
}

// Render returns the HTML representation using pkg/templates.
func (d *Demo) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		visitors := globalVisitors.Load()
		clicks := globalClicks.Load()
		uptime := time.Since(d.ConnectedAt).Round(time.Second)

		// Page configuration
		cfg := website.PageConfig{
			Title:       "GoliveKit - Phoenix LiveView for Go",
			Description: "Build interactive real-time web applications in Go without JavaScript. Server-side rendering with WebSocket-powered DOM updates.",
			URL:         "https://golivekit.cloud",
			Keywords:    []string{"go", "golang", "liveview", "phoenix", "websocket", "real-time", "server-side-rendering"},
			Author:      "Gabriel Miguel",
			Language:    "en",
			ThemeColor:  "#8B5CF6",
		}

		// Landing page options
		opts := landing.DefaultOptions()
		opts.ShowCounter = true
		opts.CounterValue = d.LocalCount
		opts.CounterLabel = "Live counter - click to test real-time updates"
		opts.Stats = website.StatsConfig{
			Visitors:    visitors,
			TotalClicks: clicks,
			Uptime:      uptime.String(),
		}
		opts.GitHubURL = "https://github.com/gabrielmiguelok/golivekit"

		// Generate the page HTML
		html := renderLandingWithLiveData(cfg, opts, d.LocalCount, visitors, clicks, uptime.String())

		_, err := w.Write([]byte(html))
		return err
	})
}

// renderLandingWithLiveData generates the landing page with live data injected.
// This is a custom render that combines the template system with live data slots.
func renderLandingWithLiveData(cfg website.PageConfig, opts landing.Options, count int, visitors, clicks int64, uptime string) string {
	var body strings.Builder

	// Navbar
	body.WriteString(components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: opts.GitHubURL,
		ShowBadge: true,
		BadgeText: "100% Go",
		Links: []website.NavLink{
			{Label: "Docs", URL: "/docs", External: false},
			{Label: "Demos", URL: "/demos", External: false},
		},
	}))

	// Main content with data-live-view wrapper
	body.WriteString(`<main id="main-content">`)
	body.WriteString("\n")
	body.WriteString(`<div data-live-view="demo">`)
	body.WriteString("\n")

	// Clean Hero with value proposition
	body.WriteString(renderHero())

	// Live Demo section (counter + stats)
	body.WriteString(renderLiveDemo(count, visitors, clicks, uptime))

	// Features section
	body.WriteString(components.RenderFeatures(components.FeaturesOptions{
		Title:    "Why GoliveKit?",
		Subtitle: "Everything you need to build modern web applications",
		Features: components.DefaultGoliveKitFeatures(),
		Columns:  3,
	}))

	// Demos section
	body.WriteString(renderDemosSection())

	// Packages section
	body.WriteString(components.RenderPackages(components.PackagesOptions{
		Title:          "29 Packages",
		Subtitle:       "Modular architecture for maximum flexibility",
		Packages:       components.DefaultGoliveKitPackages(),
		ShowCategories: true,
	}))

	// Code section
	body.WriteString(components.RenderCodeSection(
		"Simple & Powerful",
		"Write your components in pure Go with a familiar lifecycle",
		components.DefaultGoliveKitCode(),
	))

	// Architecture section
	arch := components.DefaultGoliveKitArchitecture()
	body.WriteString(components.RenderArchitecture(arch))

	// CLI section
	body.WriteString(components.RenderCLI(components.CLIOptions{
		Title:    "Developer Experience",
		Subtitle: "Powerful CLI for rapid development",
		Commands: components.DefaultGoliveKitCLI(),
	}))

	// Getting started section
	body.WriteString(`<div id="getting-started">`)
	body.WriteString("\n")
	body.WriteString(components.RenderGettingStarted(components.GettingStartedOptions{
		Title:    "Get Started in 3 Steps",
		Subtitle: "From zero to real-time in under a minute",
		Steps:    components.DefaultGoliveKitSteps(),
	}))
	body.WriteString(`</div>`)
	body.WriteString("\n")

	body.WriteString(`</div>`)
	body.WriteString("\n")
	body.WriteString(`</main>`)
	body.WriteString("\n")

	// Footer
	footerOpts := components.DefaultGoliveKitFooter()
	footerOpts.Config.GitHubURL = opts.GitHubURL
	footerOpts.Config.DocsURL = "/docs"
	footerOpts.Config.Links = []website.NavLink{
		{Label: "Demos", URL: "/demos", External: false},
	}
	body.WriteString(components.RenderFooter(footerOpts))

	// GoliveKit client script
	body.WriteString(`<script src="/_live/golivekit.js"></script>`)
	body.WriteString("\n")

	return website.RenderDocument(cfg, "", body.String())
}

// renderHero generates a clean hero section with clear value proposition.
func renderHero() string {
	return `<section class="hero section-lg" aria-labelledby="hero-title">
<div class="container">
<div class="flex justify-center" style="margin-bottom:1.5rem">
<span class="badge animate-fade-in">
<span aria-hidden="true">🔥</span> 100% Go — Zero JavaScript
</span>
</div>

<h1 id="hero-title" class="hero-title animate-fade-in delay-1">
Real-time Web Apps<br><span class="text-gradient">Without JavaScript</span>
</h1>

<p class="hero-subtitle animate-fade-in delay-2">
GoliveKit brings Phoenix LiveView to Go. Build interactive applications with server-rendered components that update instantly via WebSocket — write everything in Go.
</p>

<div class="hero-actions animate-fade-in delay-3">
<a href="#getting-started" class="btn btn-primary btn-lg">Get Started <span aria-hidden="true">→</span></a>
<a href="#live-demo" class="btn btn-secondary btn-lg">See it in Action</a>
<a href="https://github.com/gabrielmiguelok/golivekit" class="btn btn-secondary btn-lg" target="_blank" rel="noopener noreferrer"><span aria-hidden="true">⭐</span> GitHub</a>
</div>
</div>
</section>
`
}

// renderLiveDemo generates the live demo section with counter and stats.
func renderLiveDemo(count int, visitors, clicks int64, uptime string) string {
	return fmt.Sprintf(`<section id="live-demo" class="section" style="background:var(--color-bgAlt)" aria-labelledby="demo-title">
<div class="container">
<div class="text-center" style="margin-bottom:2rem">
<h2 id="demo-title">See it in Action</h2>
<p style="margin-top:0.5rem">This counter updates in real-time via WebSocket — no page reload needed</p>
</div>

<div class="grid grid-2 gap-xl" style="max-width:800px;margin:0 auto;align-items:center">

<div class="card">
<div class="card-body text-center">
<h3 style="margin-bottom:1rem;font-size:1rem;color:var(--color-textMuted)">Interactive Counter</h3>
<div class="counter-display" role="group" aria-label="Live counter demo">
<button class="counter-btn counter-btn-dec" lv-click="decrement" aria-label="Decrement counter">−</button>
<output class="counter-value" data-slot="count" aria-live="polite">%d</output>
<button class="counter-btn counter-btn-inc" lv-click="increment" aria-label="Increment counter">+</button>
</div>
<p style="font-size:0.75rem;color:var(--color-textMuted);margin-top:1rem">
<span class="animate-pulse" style="color:var(--color-success)" aria-hidden="true">●</span> Click the buttons — changes sync instantly
</p>
</div>
</div>

<div class="card">
<div class="card-body">
<h3 style="margin-bottom:1rem;font-size:1rem;color:var(--color-textMuted);text-align:center">Live Stats</h3>
<div style="display:grid;gap:1rem">
<div style="display:flex;justify-content:space-between;padding:0.5rem 0;border-bottom:1px solid var(--color-border)">
<span style="color:var(--color-textMuted)">Active Visitors</span>
<span style="font-weight:700;color:var(--color-primary)" data-slot="visitors">%d</span>
</div>
<div style="display:flex;justify-content:space-between;padding:0.5rem 0;border-bottom:1px solid var(--color-border)">
<span style="color:var(--color-textMuted)">Total Clicks</span>
<span style="font-weight:700;color:var(--color-primary)" data-slot="clicks">%d</span>
</div>
<div style="display:flex;justify-content:space-between;padding:0.5rem 0">
<span style="color:var(--color-textMuted)">Your Session</span>
<span style="font-weight:700;color:var(--color-primary)" data-slot="uptime">%s</span>
</div>
</div>
</div>
</div>

</div>
</div>
</section>
`, count, visitors, clicks, uptime)
}

// renderDemosSection generates the demos showcase section.
func renderDemosSection() string {
	return `<section class="section" style="background:var(--color-bgAlt)" aria-labelledby="demos-title">
<div class="container">
<div class="text-center" style="margin-bottom:2rem">
<h2 id="demos-title">Live Demos</h2>
<p style="margin-top:0.5rem">Explore interactive examples showcasing GoliveKit's capabilities</p>
</div>

<div class="grid grid-3 gap-lg" style="max-width:1000px;margin:0 auto">

<a href="/demos/realtime" class="card" style="text-decoration:none">
<div class="card-body text-center">
<div style="font-size:2rem;margin-bottom:0.5rem">🎵</div>
<h3 style="font-size:1rem;margin-bottom:0.5rem">Realtime Playlist</h3>
<p style="font-size:0.875rem;color:var(--color-textMuted)">Multi-user voting with PubSub</p>
</div>
</a>

<a href="/demos/forms" class="card" style="text-decoration:none">
<div class="card-body text-center">
<div style="font-size:2rem;margin-bottom:0.5rem">📝</div>
<h3 style="font-size:1rem;margin-bottom:0.5rem">Forms Wizard</h3>
<p style="font-size:0.875rem;color:var(--color-textMuted)">Multi-step with async validation</p>
</div>
</a>

<a href="/demos/uploads" class="card" style="text-decoration:none">
<div class="card-body text-center">
<div style="font-size:2rem;margin-bottom:0.5rem">📁</div>
<h3 style="font-size:1rem;margin-bottom:0.5rem">File Manager</h3>
<p style="font-size:0.875rem;color:var(--color-textMuted)">Chunked uploads with progress</p>
</div>
</a>

<a href="/demos/dashboard" class="card" style="text-decoration:none">
<div class="card-body text-center">
<div style="font-size:2rem;margin-bottom:0.5rem">📊</div>
<h3 style="font-size:1rem;margin-bottom:0.5rem">Live Dashboard</h3>
<p style="font-size:0.875rem;color:var(--color-textMuted)">Streaming data with charts</p>
</div>
</a>

<a href="/demos/game" class="card" style="text-decoration:none">
<div class="card-body text-center">
<div style="font-size:2rem;margin-bottom:0.5rem">🐍</div>
<h3 style="font-size:1rem;margin-bottom:0.5rem">Snake Game</h3>
<p style="font-size:0.875rem;color:var(--color-textMuted)">60 FPS server-side game</p>
</div>
</a>

<a href="/demos/editor" class="card" style="text-decoration:none">
<div class="card-body text-center">
<div style="font-size:2rem;margin-bottom:0.5rem">✏️</div>
<h3 style="font-size:1rem;margin-bottom:0.5rem">Collab Editor</h3>
<p style="font-size:0.875rem;color:var(--color-textMuted)">Multi-cursor editing</p>
</div>
</a>

</div>

<div class="text-center" style="margin-top:2rem">
<a href="/demos" class="btn btn-secondary">View All Demos →</a>
</div>
</div>
</section>
`
}


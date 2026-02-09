// Package demos provides demo components for GoliveKit showcase.
package demos

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// MetricPoint represents a single data point in the timeline
type MetricPoint struct {
	Time  time.Time
	Value float64
}

// SocketInfo represents info about an active socket
type SocketInfo struct {
	ID       string
	Route    string
	Duration time.Duration
}

// EventLog represents a logged event
type EventLog struct {
	Time    time.Time
	Event   string
	Payload string
}

// Global dashboard metrics (simulated)
var (
	dashConnections  atomic.Int64
	dashEventsTotal  atomic.Int64
	dashBytesTotal   atomic.Int64
	dashEventHistory = make([]MetricPoint, 0, 60)
	dashEventLogs    = make([]EventLog, 0, 20)
	dashSockets      = make(map[string]SocketInfo)
	dashMu           sync.RWMutex
	dashStartTime    = time.Now()
)

func init() {
	// Initialize with some history
	now := time.Now()
	for i := 59; i >= 0; i-- {
		dashEventHistory = append(dashEventHistory, MetricPoint{
			Time:  now.Add(-time.Duration(i) * time.Second),
			Value: float64(rand.Intn(100) + 50),
		})
	}

	// Start metric updater
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			dashMu.Lock()
			// Add new point
			newValue := float64(rand.Intn(150) + 50)
			dashEventHistory = append(dashEventHistory, MetricPoint{
				Time:  time.Now(),
				Value: newValue,
			})
			// Keep only last 60 seconds
			if len(dashEventHistory) > 60 {
				dashEventHistory = dashEventHistory[1:]
			}
			dashMu.Unlock()
		}
	}()
}

// LiveDashboard is the live metrics dashboard component.
type LiveDashboard struct {
	core.BaseComponent

	// User settings
	RefreshRate int    // seconds
	SelectedTab string // "sockets", "events", "memory"

	// Connection info
	SocketID    string
	ConnectedAt time.Time
}

// NewLiveDashboard creates a new dashboard component.
func NewLiveDashboard() core.Component {
	return &LiveDashboard{
		RefreshRate: 1,
		SelectedTab: "sockets",
	}
}

// Name returns the component name.
func (d *LiveDashboard) Name() string {
	return "live-dashboard"
}

// Mount initializes the dashboard.
func (d *LiveDashboard) Mount(ctx context.Context, params core.Params, session core.Session) error {
	d.SocketID = fmt.Sprintf("sock_%d", time.Now().UnixNano()%10000)
	d.ConnectedAt = time.Now()

	dashConnections.Add(1)

	dashMu.Lock()
	dashSockets[d.SocketID] = SocketInfo{
		ID:       d.SocketID,
		Route:    "/demos/dashboard",
		Duration: 0,
	}
	dashMu.Unlock()

	return nil
}

// Terminate handles cleanup.
func (d *LiveDashboard) Terminate(ctx context.Context, reason core.TerminateReason) error {
	dashConnections.Add(-1)

	dashMu.Lock()
	delete(dashSockets, d.SocketID)
	dashMu.Unlock()

	return nil
}

// HandleEvent handles user interactions.
func (d *LiveDashboard) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	dashEventsTotal.Add(1)
	dashBytesTotal.Add(int64(len(event) + 50)) // Approximate payload size

	// Log event
	dashMu.Lock()
	dashEventLogs = append(dashEventLogs, EventLog{
		Time:    time.Now(),
		Event:   event,
		Payload: fmt.Sprintf("%v", payload),
	})
	if len(dashEventLogs) > 20 {
		dashEventLogs = dashEventLogs[1:]
	}
	dashMu.Unlock()

	switch event {
	case "set_refresh":
		if val, ok := payload["value"].(float64); ok {
			d.RefreshRate = int(val)
			if d.RefreshRate < 1 {
				d.RefreshRate = 1
			}
			if d.RefreshRate > 10 {
				d.RefreshRate = 10
			}
		}

	case "switch_tab":
		if tab, ok := payload["tab"].(string); ok {
			d.SelectedTab = tab
		}

	case "tick":
		// Periodic update from client
	}

	return nil
}

// Render returns the HTML representation.
func (d *LiveDashboard) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := d.renderDashboard()
		_, err := w.Write([]byte(html))
		return err
	})
}

// renderDashboard generates the complete dashboard HTML
func (d *LiveDashboard) renderDashboard() string {
	cfg := website.PageConfig{
		Title:       "Live Dashboard - GoliveKit Demo",
		Description: "Real-time metrics dashboard with streaming data and charts.",
		URL:         "https://golivekit.cloud/demos/dashboard",
		Keywords:    []string{"dashboard", "metrics", "streaming", "liveview"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	body := d.renderDashboardBody()
	return website.RenderDocument(cfg, renderDashboardStyles(), body)
}

// renderDashboardStyles returns custom CSS
func renderDashboardStyles() string {
	return `
<style>
.dashboard-container {
	max-width: 1200px;
	margin: 0 auto;
	padding: 1.5rem;
}

.dashboard-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 1.5rem;
	flex-wrap: wrap;
	gap: 1rem;
}

.dashboard-title {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.dashboard-title h1 {
	font-size: 1.5rem;
	margin: 0;
}

.refresh-control {
	display: flex;
	align-items: center;
	gap: 0.75rem;
	background: var(--color-bgAlt);
	padding: 0.5rem 1rem;
	border-radius: 0.5rem;
	font-size: 0.875rem;
}

.refresh-btn {
	width: 28px;
	height: 28px;
	border: 1px solid var(--color-border);
	background: var(--color-bg);
	border-radius: 4px;
	cursor: pointer;
	display: flex;
	align-items: center;
	justify-content: center;
}

.refresh-btn:hover {
	border-color: var(--color-primary);
}

.metrics-grid {
	display: grid;
	grid-template-columns: repeat(4, 1fr);
	gap: 1rem;
	margin-bottom: 1.5rem;
}

@media (max-width: 900px) {
	.metrics-grid {
		grid-template-columns: repeat(2, 1fr);
	}
}

@media (max-width: 500px) {
	.metrics-grid {
		grid-template-columns: 1fr;
	}
}

.metric-card {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
	padding: 1.25rem;
}

.metric-label {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	text-transform: uppercase;
	letter-spacing: 0.05em;
	margin-bottom: 0.5rem;
}

.metric-value {
	font-size: 2rem;
	font-weight: 700;
	color: var(--color-text);
}

.metric-change {
	font-size: 0.75rem;
	margin-top: 0.25rem;
}

.metric-change.up {
	color: var(--color-success);
}

.metric-change.down {
	color: #ef4444;
}

.chart-card {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
	padding: 1.25rem;
	margin-bottom: 1.5rem;
}

.chart-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 1rem;
}

.chart-title {
	font-size: 0.875rem;
	font-weight: 600;
}

.chart-legend {
	display: flex;
	gap: 1rem;
	font-size: 0.75rem;
	color: var(--color-textMuted);
}

.chart-container {
	height: 150px;
	position: relative;
	font-family: monospace;
	font-size: 0.625rem;
	color: var(--color-textMuted);
}

.ascii-chart {
	white-space: pre;
	line-height: 1.2;
}

.chart-bar {
	display: inline-block;
	background: var(--color-primary);
	min-width: 8px;
	margin-right: 2px;
	border-radius: 2px 2px 0 0;
	transition: height 0.3s;
}

.panels-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 1.5rem;
}

@media (max-width: 800px) {
	.panels-grid {
		grid-template-columns: 1fr;
	}
}

.panel-card {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
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
	font-size: 0.875rem;
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
	max-height: 300px;
	overflow-y: auto;
}

.socket-item {
	display: flex;
	align-items: center;
	gap: 0.75rem;
	padding: 0.5rem;
	border-radius: 0.5rem;
	font-size: 0.875rem;
}

.socket-item:hover {
	background: var(--color-bg);
}

.socket-id {
	font-family: monospace;
	background: var(--color-bg);
	padding: 0.25rem 0.5rem;
	border-radius: 0.25rem;
	font-size: 0.75rem;
}

.socket-route {
	flex: 1;
	color: var(--color-textMuted);
}

.socket-duration {
	font-size: 0.75rem;
	color: var(--color-textMuted);
}

.event-item {
	display: flex;
	gap: 0.75rem;
	padding: 0.5rem;
	border-bottom: 1px solid var(--color-border);
	font-size: 0.8125rem;
}

.event-item:last-child {
	border-bottom: none;
}

.event-time {
	font-family: monospace;
	color: var(--color-textMuted);
	font-size: 0.75rem;
	white-space: nowrap;
}

.event-name {
	font-weight: 600;
	color: var(--color-primary);
}

.event-payload {
	color: var(--color-textMuted);
	font-size: 0.75rem;
	font-family: monospace;
	word-break: break-all;
}

.memory-bar {
	height: 20px;
	background: var(--color-border);
	border-radius: 4px;
	overflow: hidden;
	margin-bottom: 0.5rem;
}

.memory-fill {
	height: 100%;
	background: var(--color-primary);
	transition: width 0.3s;
}

.memory-stats {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 0.75rem;
	font-size: 0.8125rem;
}

.memory-stat {
	display: flex;
	justify-content: space-between;
}

.memory-label {
	color: var(--color-textMuted);
}

.memory-value {
	font-weight: 600;
	font-family: monospace;
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

.uptime-badge {
	font-size: 0.75rem;
	color: var(--color-success);
	display: flex;
	align-items: center;
	gap: 0.25rem;
}

.sparkline {
	display: flex;
	align-items: flex-end;
	height: 100px;
	gap: 2px;
	padding: 0.5rem;
	background: var(--color-bg);
	border-radius: 0.5rem;
}

.spark-bar {
	flex: 1;
	min-width: 4px;
	background: var(--color-primary);
	border-radius: 2px 2px 0 0;
	opacity: 0.7;
	transition: height 0.3s;
}

.spark-bar:hover {
	opacity: 1;
}

.chart-axis {
	display: flex;
	justify-content: space-between;
	font-size: 0.625rem;
	color: var(--color-textMuted);
	padding: 0 0.5rem;
	margin-top: 0.25rem;
}
</style>
`
}

// renderDashboardBody generates the main content
func (d *LiveDashboard) renderDashboardBody() string {
	// Navbar
	navbar := components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "Dashboard Demo",
		Links: []website.NavLink{
			{Label: "Demos", URL: "/demos", External: false},
			{Label: "Docs", URL: "/docs", External: false},
		},
	})

	// Get metrics
	connections := dashConnections.Load()
	eventsTotal := dashEventsTotal.Load()
	bytesTotal := dashBytesTotal.Load()
	uptime := time.Since(dashStartTime).Round(time.Second)

	// Get memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate events per second (average over history)
	dashMu.RLock()
	eventsPerSec := 0.0
	if len(dashEventHistory) > 1 {
		for _, p := range dashEventHistory {
			eventsPerSec += p.Value
		}
		eventsPerSec /= float64(len(dashEventHistory))
	}
	dashMu.RUnlock()

	// Format bytes
	bytesStr := formatBytes(bytesTotal)
	memUsed := formatBytes(int64(m.Alloc))
	memSys := formatBytes(int64(m.Sys))

	content := fmt.Sprintf(`
<main id="main-content">
<div class="dashboard-container" data-live-view="live-dashboard">

<a href="/demos" class="back-link">← Back to Demos</a>

<div class="dashboard-header">
	<div class="dashboard-title">
		<span style="font-size:1.5rem">📊</span>
		<h1>Live Dashboard</h1>
		<span class="uptime-badge" data-slot="uptime">● %s uptime</span>
	</div>
	<div class="refresh-control">
		<span>Auto-refresh:</span>
		<button class="refresh-btn" lv-click="set_refresh" lv-value-value="%d">−</button>
		<span data-slot="refresh">%ds</span>
		<button class="refresh-btn" lv-click="set_refresh" lv-value-value="%d">+</button>
	</div>
</div>

<div class="metrics-grid" data-slot="metrics">
	<div class="metric-card">
		<div class="metric-label">Connections</div>
		<div class="metric-value" data-slot="connections">%d</div>
		<div class="metric-change up">● Active</div>
	</div>
	<div class="metric-card">
		<div class="metric-label">Events/sec</div>
		<div class="metric-value" data-slot="events-sec">%.0f</div>
		<div class="metric-change up">▲ Avg</div>
	</div>
	<div class="metric-card">
		<div class="metric-label">Total Events</div>
		<div class="metric-value" data-slot="events-total">%d</div>
		<div class="metric-change up">▲ Cumulative</div>
	</div>
	<div class="metric-card">
		<div class="metric-label">Data Transfer</div>
		<div class="metric-value" data-slot="bytes">%s</div>
		<div class="metric-change">Total</div>
	</div>
</div>

<div class="chart-card">
	<div class="chart-header">
		<span class="chart-title">Events Timeline (last 60 seconds)</span>
		<div class="chart-legend">
			<span>Min: 50</span>
			<span>Max: 200</span>
		</div>
	</div>
	<div class="chart-container" data-slot="chart">
		%s
	</div>
	<div class="chart-axis">
		<span>-60s</span>
		<span>-30s</span>
		<span>now</span>
	</div>
</div>

<div class="panels-grid">
	<div class="panel-card">
		<div class="panel-tabs">
			<button class="panel-tab %s" lv-click="switch_tab" lv-value-tab="sockets">Active Sockets</button>
			<button class="panel-tab %s" lv-click="switch_tab" lv-value-tab="events">Recent Events</button>
		</div>
		<div class="panel-content" data-slot="panel">
			%s
		</div>
	</div>

	<div class="panel-card">
		<div class="panel-tabs">
			<button class="panel-tab active">Memory</button>
		</div>
		<div class="panel-content">
			<div class="memory-bar">
				<div class="memory-fill" style="width:%d%%"></div>
			</div>
			<div class="memory-stats" data-slot="memory">
				<div class="memory-stat">
					<span class="memory-label">Allocated</span>
					<span class="memory-value">%s</span>
				</div>
				<div class="memory-stat">
					<span class="memory-label">System</span>
					<span class="memory-value">%s</span>
				</div>
				<div class="memory-stat">
					<span class="memory-label">Goroutines</span>
					<span class="memory-value">%d</span>
				</div>
				<div class="memory-stat">
					<span class="memory-label">GC Cycles</span>
					<span class="memory-value">%d</span>
				</div>
			</div>
		</div>
	</div>
</div>

</div>
</main>

<script src="/_live/golivekit.js"></script>
<script>
setInterval(function() {
	if (window.liveSocket && window.liveSocket.isConnected()) {
		window.liveSocket.pushEvent("tick", {});
	}
}, %d);
</script>
`, uptime.String(), d.RefreshRate-1, d.RefreshRate, d.RefreshRate+1,
		connections, eventsPerSec, eventsTotal, bytesStr,
		d.renderSparkline(),
		d.tabClass("sockets"), d.tabClass("events"),
		d.renderSelectedPanel(),
		int(float64(m.Alloc)/float64(m.Sys)*100),
		memUsed, memSys, runtime.NumGoroutine(), m.NumGC,
		d.RefreshRate*1000)

	return navbar + content
}

// tabClass returns active class if tab matches
func (d *LiveDashboard) tabClass(tab string) string {
	if d.SelectedTab == tab {
		return "active"
	}
	return ""
}

// renderSelectedPanel renders the content based on selected tab
func (d *LiveDashboard) renderSelectedPanel() string {
	if d.SelectedTab == "events" {
		return d.renderEventsPanel()
	}
	return d.renderSocketsPanel()
}

// renderSocketsPanel renders active sockets list
func (d *LiveDashboard) renderSocketsPanel() string {
	dashMu.RLock()
	defer dashMu.RUnlock()

	if len(dashSockets) == 0 {
		return `<p style="text-align:center;color:var(--color-textMuted)">No active sockets</p>`
	}

	var html string
	for _, sock := range dashSockets {
		duration := time.Since(time.Now().Add(-sock.Duration)).Round(time.Second)
		isSelf := ""
		if sock.ID == d.SocketID {
			isSelf = " (you)"
		}
		html += fmt.Sprintf(`
<div class="socket-item">
	<span class="socket-id">%s%s</span>
	<span class="socket-route">%s</span>
	<span class="socket-duration">%s</span>
</div>
`, sock.ID, isSelf, sock.Route, duration)
	}

	return html
}

// renderEventsPanel renders recent events log
func (d *LiveDashboard) renderEventsPanel() string {
	dashMu.RLock()
	defer dashMu.RUnlock()

	if len(dashEventLogs) == 0 {
		return `<p style="text-align:center;color:var(--color-textMuted)">No events yet</p>`
	}

	var html string
	// Show in reverse order (newest first)
	for i := len(dashEventLogs) - 1; i >= 0; i-- {
		log := dashEventLogs[i]
		html += fmt.Sprintf(`
<div class="event-item">
	<span class="event-time">%s</span>
	<span class="event-name">%s</span>
	<span class="event-payload">%s</span>
</div>
`, log.Time.Format("15:04:05"), log.Event, truncate(log.Payload, 30))
	}

	return html
}

// renderSparkline renders an ASCII/CSS sparkline chart
func (d *LiveDashboard) renderSparkline() string {
	dashMu.RLock()
	defer dashMu.RUnlock()

	var bars string
	maxVal := 200.0
	for _, p := range dashEventHistory {
		heightPct := (p.Value / maxVal) * 100
		if heightPct > 100 {
			heightPct = 100
		}
		bars += fmt.Sprintf(`<div class="spark-bar" style="height:%.0f%%" title="%.0f"></div>`, heightPct, p.Value)
	}

	return `<div class="sparkline">` + bars + `</div>`
}

// formatBytes formats bytes to human readable string
func formatBytes(b int64) string {
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

// truncate truncates a string to max length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

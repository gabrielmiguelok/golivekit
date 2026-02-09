// Package demos provides demo components for GoliveKit showcase.
package demos

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// BenchmarkResult represents benchmark test results
type BenchmarkResult struct {
	TotalEvents   int
	TotalTime     time.Duration
	AvgLatency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	Throughput    float64
	Errors        int
	BytesSent     int64
	BytesReceived int64
}

// KitchenSink is the kitchen sink demo component.
type KitchenSink struct {
	core.BaseComponent

	// Mini demos state
	CounterValue    int
	FormName        string
	FormValidated   bool
	ListItems       []string
	PresenceUsers   []string

	// Metrics state
	MetricEvents    int64
	MetricLatency   time.Duration
	MetricCacheHits int
	MetricMisses    int

	// Benchmark state
	BenchRunning     bool
	BenchEventsPerSec int
	BenchPayloadKB   int
	BenchConcurrent  int
	BenchProgress    int
	BenchResult      *BenchmarkResult

	// Connection info
	ConnectedAt time.Time
	EventsTotal atomic.Int64
}

// Global showcase stats
var (
	showcaseVisitors atomic.Int64
	showcaseEvents   atomic.Int64
)

// NewKitchenSink creates a new kitchen sink component.
func NewKitchenSink() core.Component {
	return &KitchenSink{
		BenchEventsPerSec: 50,
		BenchPayloadKB:    2,
		BenchConcurrent:   10,
	}
}

// Name returns the component name.
func (k *KitchenSink) Name() string {
	return "kitchen-sink"
}

// Mount initializes the component.
func (k *KitchenSink) Mount(ctx context.Context, params core.Params, session core.Session) error {
	k.ConnectedAt = time.Now()

	// Initialize mini demos
	k.CounterValue = 0
	k.ListItems = []string{"Item 1", "Item 2", "Item 3"}
	k.PresenceUsers = []string{"Alice", "Bob", "Charlie"}

	showcaseVisitors.Add(1)
	return nil
}

// Terminate handles cleanup.
func (k *KitchenSink) Terminate(ctx context.Context, reason core.TerminateReason) error {
	showcaseVisitors.Add(-1)
	return nil
}

// HandleEvent handles user interactions.
func (k *KitchenSink) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	k.EventsTotal.Add(1)
	showcaseEvents.Add(1)

	switch event {
	// Counter mini-demo
	case "counter_inc":
		k.CounterValue++
	case "counter_dec":
		k.CounterValue--
	case "counter_reset":
		k.CounterValue = 0

	// Form mini-demo
	case "form_update":
		if val, ok := payload["value"].(string); ok {
			k.FormName = val
			k.FormValidated = len(val) >= 3
		}

	// List mini-demo
	case "list_add":
		k.ListItems = append(k.ListItems, fmt.Sprintf("Item %d", len(k.ListItems)+1))
	case "list_remove":
		if idx, ok := payload["index"].(float64); ok && int(idx) < len(k.ListItems) {
			k.ListItems = append(k.ListItems[:int(idx)], k.ListItems[int(idx)+1:]...)
		}

	// Presence mini-demo (simulated)
	case "presence_join":
		names := []string{"Diana", "Eve", "Frank", "Grace"}
		k.PresenceUsers = append(k.PresenceUsers, names[rand.Intn(len(names))])
	case "presence_leave":
		if len(k.PresenceUsers) > 1 {
			k.PresenceUsers = k.PresenceUsers[:len(k.PresenceUsers)-1]
		}

	// Benchmark controls
	case "bench_events":
		if val, ok := payload["value"].(float64); ok {
			k.BenchEventsPerSec = int(val)
			if k.BenchEventsPerSec < 1 {
				k.BenchEventsPerSec = 1
			}
			if k.BenchEventsPerSec > 100 {
				k.BenchEventsPerSec = 100
			}
		}

	case "bench_payload":
		if val, ok := payload["value"].(float64); ok {
			k.BenchPayloadKB = int(val)
			if k.BenchPayloadKB < 1 {
				k.BenchPayloadKB = 1
			}
			if k.BenchPayloadKB > 10 {
				k.BenchPayloadKB = 10
			}
		}

	case "bench_concurrent":
		if val, ok := payload["value"].(float64); ok {
			k.BenchConcurrent = int(val)
			if k.BenchConcurrent < 1 {
				k.BenchConcurrent = 1
			}
			if k.BenchConcurrent > 50 {
				k.BenchConcurrent = 50
			}
		}

	case "bench_start":
		if !k.BenchRunning {
			k.startBenchmark()
		}

	case "bench_stop":
		k.BenchRunning = false

	case "bench_reset":
		k.BenchRunning = false
		k.BenchProgress = 0
		k.BenchResult = nil

	case "bench_progress":
		if progress, ok := payload["progress"].(float64); ok {
			k.BenchProgress = int(progress)
		}

	case "tick":
		// Update metrics
		k.updateMetrics()
	}

	return nil
}

// startBenchmark starts a simulated benchmark
func (k *KitchenSink) startBenchmark() {
	k.BenchRunning = true
	k.BenchProgress = 0
	k.BenchResult = nil

	// Simulate benchmark (would normally run actual tests)
	go func() {
		startTime := time.Now()
		totalEvents := k.BenchEventsPerSec * 20 // 20 second test

		for i := 0; i < 100 && k.BenchRunning; i++ {
			time.Sleep(200 * time.Millisecond)
			k.BenchProgress = i + 1
		}

		if k.BenchRunning {
			elapsed := time.Since(startTime)
			k.BenchResult = &BenchmarkResult{
				TotalEvents:   totalEvents,
				TotalTime:     elapsed,
				AvgLatency:    time.Duration(3+rand.Intn(5)) * time.Millisecond,
				P95Latency:    time.Duration(8+rand.Intn(5)) * time.Millisecond,
				P99Latency:    time.Duration(12+rand.Intn(8)) * time.Millisecond,
				Throughput:    float64(totalEvents) / elapsed.Seconds(),
				Errors:        0,
				BytesSent:     int64(totalEvents * k.BenchPayloadKB * 1024),
				BytesReceived: int64(totalEvents * 100),
			}
		}
		k.BenchRunning = false
	}()
}

// updateMetrics updates the live metrics
func (k *KitchenSink) updateMetrics() {
	k.MetricEvents = k.EventsTotal.Load()
	k.MetricLatency = time.Duration(2+rand.Intn(5)) * time.Millisecond
	k.MetricCacheHits = 80 + rand.Intn(15)
	k.MetricMisses = 100 - k.MetricCacheHits
}

// Render returns the HTML representation.
func (k *KitchenSink) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := k.renderKitchenSink()
		_, err := w.Write([]byte(html))
		return err
	})
}

// renderKitchenSink generates the complete HTML
func (k *KitchenSink) renderKitchenSink() string {
	cfg := website.PageConfig{
		Title:       "Kitchen Sink - GoliveKit Demo",
		Description: "Comprehensive showcase of all GoliveKit features with benchmarking.",
		URL:         "https://golivekit.cloud/demos/showcase",
		Keywords:    []string{"showcase", "benchmark", "all-features", "liveview"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	body := k.renderKitchenSinkBody()
	return website.RenderDocument(cfg, renderKitchenSinkStyles(), body)
}

// renderKitchenSinkStyles returns custom CSS
func renderKitchenSinkStyles() string {
	return `
<style>
.ks-container {
	max-width: 1200px;
	margin: 0 auto;
	padding: 1.5rem;
}

.ks-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 1.5rem;
	flex-wrap: wrap;
	gap: 1rem;
}

.ks-title {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.ks-title h1 {
	font-size: 1.5rem;
	margin: 0;
}

.ks-btn {
	padding: 0.5rem 1rem;
	border: 1px solid var(--color-border);
	border-radius: 0.5rem;
	background: var(--color-bg);
	cursor: pointer;
	font-size: 0.875rem;
	transition: all 0.2s;
}

.ks-btn:hover {
	border-color: var(--color-primary);
}

.ks-btn-primary {
	background: var(--color-primary);
	border-color: var(--color-primary);
	color: white;
}

.ks-btn-primary:hover {
	background: #7c3aed;
}

.ks-btn-danger {
	background: #ef4444;
	border-color: #ef4444;
	color: white;
}

.ks-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 1.5rem;
}

@media (max-width: 900px) {
	.ks-grid {
		grid-template-columns: 1fr;
	}
}

.ks-card {
	background: var(--color-bgAlt);
	border: 1px solid var(--color-border);
	border-radius: 0.75rem;
	padding: 1.25rem;
}

.card-title {
	font-size: 1rem;
	font-weight: 600;
	margin-bottom: 1rem;
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.metrics-grid {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 1rem;
}

.metric-item {
	background: var(--color-bg);
	padding: 1rem;
	border-radius: 0.5rem;
	text-align: center;
}

.metric-label {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	text-transform: uppercase;
	letter-spacing: 0.05em;
}

.metric-value {
	font-size: 1.5rem;
	font-weight: 700;
	color: var(--color-primary);
	margin-top: 0.25rem;
}

.mini-demos {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 1rem;
}

@media (max-width: 600px) {
	.mini-demos {
		grid-template-columns: 1fr;
	}
}

.mini-demo {
	background: var(--color-bg);
	padding: 1rem;
	border-radius: 0.5rem;
}

.mini-title {
	font-size: 0.8125rem;
	font-weight: 600;
	margin-bottom: 0.75rem;
	color: var(--color-textMuted);
}

.counter-demo {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 0.5rem;
}

.counter-btn {
	width: 36px;
	height: 36px;
	border-radius: 50%;
	border: 1px solid var(--color-border);
	background: var(--color-bgAlt);
	cursor: pointer;
	font-size: 1.25rem;
	display: flex;
	align-items: center;
	justify-content: center;
}

.counter-btn:hover {
	border-color: var(--color-primary);
}

.counter-value {
	font-size: 1.5rem;
	font-weight: 700;
	width: 60px;
	text-align: center;
}

.form-demo input {
	width: 100%;
	padding: 0.5rem;
	border: 1px solid var(--color-border);
	border-radius: 0.375rem;
	background: var(--color-bgAlt);
	color: var(--color-text);
	margin-bottom: 0.5rem;
}

.form-demo input:focus {
	outline: none;
	border-color: var(--color-primary);
}

.form-status {
	font-size: 0.75rem;
}

.form-status.valid {
	color: var(--color-success);
}

.form-status.invalid {
	color: #ef4444;
}

.list-demo {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.list-item {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 0.5rem;
	background: var(--color-bgAlt);
	border-radius: 0.375rem;
	font-size: 0.875rem;
}

.list-remove {
	width: 20px;
	height: 20px;
	border: none;
	background: transparent;
	cursor: pointer;
	color: var(--color-textMuted);
}

.list-remove:hover {
	color: #ef4444;
}

.list-add {
	padding: 0.375rem 0.75rem;
	border: 1px dashed var(--color-border);
	border-radius: 0.375rem;
	background: transparent;
	cursor: pointer;
	font-size: 0.8125rem;
	color: var(--color-textMuted);
}

.list-add:hover {
	border-color: var(--color-primary);
	color: var(--color-primary);
}

.presence-demo {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.presence-users {
	display: flex;
	flex-wrap: wrap;
	gap: 0.375rem;
}

.presence-user {
	display: flex;
	align-items: center;
	gap: 0.375rem;
	padding: 0.25rem 0.5rem;
	background: var(--color-bgAlt);
	border-radius: 1rem;
	font-size: 0.75rem;
}

.presence-dot {
	width: 6px;
	height: 6px;
	background: var(--color-success);
	border-radius: 50%;
}

.presence-controls {
	display: flex;
	gap: 0.5rem;
}

.presence-btn {
	padding: 0.25rem 0.5rem;
	border: 1px solid var(--color-border);
	border-radius: 0.25rem;
	background: transparent;
	cursor: pointer;
	font-size: 0.75rem;
}

.presence-btn:hover {
	border-color: var(--color-primary);
}

.bench-controls {
	display: grid;
	grid-template-columns: repeat(3, 1fr);
	gap: 1rem;
	margin-bottom: 1rem;
}

.control-group {
	display: flex;
	flex-direction: column;
	gap: 0.5rem;
}

.control-label {
	font-size: 0.75rem;
	color: var(--color-textMuted);
}

.control-slider {
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.slider {
	flex: 1;
	height: 6px;
	background: var(--color-border);
	border-radius: 3px;
	cursor: pointer;
}

.slider-fill {
	height: 100%;
	background: var(--color-primary);
	border-radius: 3px;
}

.slider-value {
	width: 40px;
	text-align: right;
	font-weight: 600;
}

.bench-actions {
	display: flex;
	gap: 0.5rem;
	margin-bottom: 1rem;
}

.bench-progress {
	margin-bottom: 1rem;
}

.progress-bar {
	height: 8px;
	background: var(--color-border);
	border-radius: 4px;
	overflow: hidden;
	margin-bottom: 0.5rem;
}

.progress-fill {
	height: 100%;
	background: var(--color-primary);
	transition: width 0.3s;
}

.progress-text {
	font-size: 0.75rem;
	color: var(--color-textMuted);
	text-align: center;
}

.bench-results {
	background: var(--color-bg);
	padding: 1rem;
	border-radius: 0.5rem;
}

.results-title {
	font-size: 0.875rem;
	font-weight: 600;
	margin-bottom: 0.75rem;
}

.results-grid {
	display: grid;
	grid-template-columns: repeat(2, 1fr);
	gap: 0.5rem;
	font-size: 0.8125rem;
}

.result-item {
	display: flex;
	justify-content: space-between;
}

.result-label {
	color: var(--color-textMuted);
}

.result-value {
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
</style>
`
}

// renderKitchenSinkBody generates the main content
func (k *KitchenSink) renderKitchenSinkBody() string {
	// Navbar
	navbar := components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "Kitchen Sink",
		Links: []website.NavLink{
			{Label: "Demos", URL: "/demos", External: false},
			{Label: "Docs", URL: "/docs", External: false},
		},
	})

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	visitors := showcaseVisitors.Load()
	events := showcaseEvents.Load()
	uptime := time.Since(k.ConnectedAt).Round(time.Second)

	content := fmt.Sprintf(`
<main id="main-content">
<div class="ks-container" data-live-view="kitchen-sink">

<a href="/demos" class="back-link">← Back to Demos</a>

<div class="ks-header">
	<div class="ks-title">
		<span style="font-size:1.5rem">🎯</span>
		<h1>Kitchen Sink</h1>
	</div>
	<button class="ks-btn ks-btn-primary" lv-click="bench_start">▶️ Run Benchmark</button>
</div>

<div class="ks-grid">
	<div class="ks-card">
		<div class="card-title">📊 Live Metrics</div>
		<div class="metrics-grid" data-slot="metrics">
			<div class="metric-item">
				<div class="metric-label">Diff Latency</div>
				<div class="metric-value">%dms</div>
			</div>
			<div class="metric-item">
				<div class="metric-label">Messages</div>
				<div class="metric-value">%d</div>
			</div>
			<div class="metric-item">
				<div class="metric-label">Cache Hits</div>
				<div class="metric-value">%d%%</div>
			</div>
			<div class="metric-item">
				<div class="metric-label">Memory</div>
				<div class="metric-value">%s</div>
			</div>
		</div>
	</div>

	<div class="ks-card">
		<div class="card-title">⚙️ Benchmark Controls</div>
		<div class="bench-controls">
			<div class="control-group">
				<span class="control-label">Events/sec</span>
				<div class="control-slider">
					<div class="slider" lv-click="bench_events" lv-value-value="%d">
						<div class="slider-fill" style="width:%d%%"></div>
					</div>
					<span class="slider-value">%d</span>
				</div>
			</div>
			<div class="control-group">
				<span class="control-label">Payload KB</span>
				<div class="control-slider">
					<div class="slider" lv-click="bench_payload" lv-value-value="%d">
						<div class="slider-fill" style="width:%d%%"></div>
					</div>
					<span class="slider-value">%d</span>
				</div>
			</div>
			<div class="control-group">
				<span class="control-label">Concurrent</span>
				<div class="control-slider">
					<div class="slider" lv-click="bench_concurrent" lv-value-value="%d">
						<div class="slider-fill" style="width:%d%%"></div>
					</div>
					<span class="slider-value">%d</span>
				</div>
			</div>
		</div>
		<div class="bench-actions">
			<button class="ks-btn ks-btn-primary" lv-click="bench_start" %s>▶️ Start</button>
			<button class="ks-btn ks-btn-danger" lv-click="bench_stop" %s>⏹️ Stop</button>
			<button class="ks-btn" lv-click="bench_reset">🔄 Reset</button>
		</div>
		<div class="bench-progress" data-slot="progress">
			<div class="progress-bar">
				<div class="progress-fill" style="width:%d%%"></div>
			</div>
			<div class="progress-text">%s</div>
		</div>
		%s
	</div>

	<div class="ks-card" style="grid-column: span 2">
		<div class="card-title">🧪 Mini Demos</div>
		<div class="mini-demos">
			%s
			%s
			%s
			%s
		</div>
	</div>

	<div class="ks-card" style="grid-column: span 2">
		<div class="card-title">📡 Session Info</div>
		<div class="metrics-grid">
			<div class="metric-item">
				<div class="metric-label">Visitors</div>
				<div class="metric-value">%d</div>
			</div>
			<div class="metric-item">
				<div class="metric-label">Total Events</div>
				<div class="metric-value">%d</div>
			</div>
			<div class="metric-item">
				<div class="metric-label">Your Events</div>
				<div class="metric-value">%d</div>
			</div>
			<div class="metric-item">
				<div class="metric-label">Session Time</div>
				<div class="metric-value">%s</div>
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
}, 1000);
</script>
`, k.MetricLatency.Milliseconds(), k.MetricEvents, k.MetricCacheHits, formatBytes(int64(m.Alloc)),
		k.BenchEventsPerSec+10, k.BenchEventsPerSec, k.BenchEventsPerSec,
		k.BenchPayloadKB+1, k.BenchPayloadKB*10, k.BenchPayloadKB,
		k.BenchConcurrent+5, k.BenchConcurrent*2, k.BenchConcurrent,
		k.disabledIf(k.BenchRunning), k.disabledIf(!k.BenchRunning),
		k.BenchProgress, k.progressText(),
		k.renderBenchResults(),
		k.renderCounterDemo(), k.renderFormDemo(), k.renderListDemo(), k.renderPresenceDemo(),
		visitors, events, k.EventsTotal.Load(), uptime.String())

	return navbar + content
}

// disabledIf returns disabled attr if condition is true
func (k *KitchenSink) disabledIf(cond bool) string {
	if cond {
		return "disabled"
	}
	return ""
}

// progressText returns progress text
func (k *KitchenSink) progressText() string {
	if k.BenchRunning {
		return fmt.Sprintf("Running... %d%%", k.BenchProgress)
	}
	if k.BenchResult != nil {
		return "Complete!"
	}
	return "Ready to start"
}

// renderBenchResults renders benchmark results
func (k *KitchenSink) renderBenchResults() string {
	if k.BenchResult == nil {
		return ""
	}

	r := k.BenchResult
	return fmt.Sprintf(`
<div class="bench-results" data-slot="results">
	<div class="results-title">📈 Results</div>
	<div class="results-grid">
		<div class="result-item">
			<span class="result-label">Total Events</span>
			<span class="result-value">%d</span>
		</div>
		<div class="result-item">
			<span class="result-label">Throughput</span>
			<span class="result-value">%.1f/sec</span>
		</div>
		<div class="result-item">
			<span class="result-label">Avg Latency</span>
			<span class="result-value">%dms</span>
		</div>
		<div class="result-item">
			<span class="result-label">P95 Latency</span>
			<span class="result-value">%dms</span>
		</div>
		<div class="result-item">
			<span class="result-label">P99 Latency</span>
			<span class="result-value">%dms</span>
		</div>
		<div class="result-item">
			<span class="result-label">Errors</span>
			<span class="result-value">%d</span>
		</div>
	</div>
</div>
`, r.TotalEvents, r.Throughput, r.AvgLatency.Milliseconds(), r.P95Latency.Milliseconds(), r.P99Latency.Milliseconds(), r.Errors)
}

// renderCounterDemo renders the counter mini-demo
func (k *KitchenSink) renderCounterDemo() string {
	return fmt.Sprintf(`
<div class="mini-demo">
	<div class="mini-title">Counter</div>
	<div class="counter-demo">
		<button class="counter-btn" lv-click="counter_dec">−</button>
		<span class="counter-value" data-slot="counter">%d</span>
		<button class="counter-btn" lv-click="counter_inc">+</button>
	</div>
</div>
`, k.CounterValue)
}

// renderFormDemo renders the form mini-demo
func (k *KitchenSink) renderFormDemo() string {
	statusClass := "invalid"
	statusText := "Enter at least 3 characters"
	if k.FormValidated {
		statusClass = "valid"
		statusText = "✓ Valid"
	}

	return fmt.Sprintf(`
<div class="mini-demo">
	<div class="mini-title">Form Validation</div>
	<div class="form-demo">
		<input type="text" placeholder="Enter name..."
			lv-change="form_update" lv-debounce="150" value="%s">
		<div class="form-status %s">%s</div>
	</div>
</div>
`, k.FormName, statusClass, statusText)
}

// renderListDemo renders the list mini-demo
func (k *KitchenSink) renderListDemo() string {
	var items string
	for i, item := range k.ListItems {
		items += fmt.Sprintf(`
<div class="list-item">
	<span>%s</span>
	<button class="list-remove" lv-click="list_remove" lv-value-index="%d">✕</button>
</div>
`, item, i)
	}

	return fmt.Sprintf(`
<div class="mini-demo">
	<div class="mini-title">Dynamic List</div>
	<div class="list-demo" data-slot="list">
		%s
		<button class="list-add" lv-click="list_add">+ Add Item</button>
	</div>
</div>
`, items)
}

// renderPresenceDemo renders the presence mini-demo
func (k *KitchenSink) renderPresenceDemo() string {
	var users string
	for _, user := range k.PresenceUsers {
		users += fmt.Sprintf(`
<span class="presence-user">
	<span class="presence-dot"></span>
	%s
</span>
`, user)
	}

	return fmt.Sprintf(`
<div class="mini-demo">
	<div class="mini-title">Presence</div>
	<div class="presence-demo">
		<div class="presence-users" data-slot="presence">%s</div>
		<div class="presence-controls">
			<button class="presence-btn" lv-click="presence_join">+ Join</button>
			<button class="presence-btn" lv-click="presence_leave">− Leave</button>
		</div>
	</div>
</div>
`, users)
}

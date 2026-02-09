// Package metrics provides observability metrics for GoliveKit.
package metrics

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds all application metrics.
type Metrics struct {
	// Connections
	ConnectionsActive   *Gauge
	ConnectionsTotal    *Counter

	// Messages
	MessagesReceived *CounterVec
	MessagesSent     *CounterVec
	MessageLatency   *Histogram

	// Renders
	RenderCount    *Counter
	RenderDuration *Histogram
	DiffSize       *Histogram

	// Errors
	ErrorsTotal *CounterVec
	PanicsTotal *Counter

	// Resources
	MemoryPerSocket  *Histogram
	GoroutinesActive *Gauge

	// Custom metrics
	custom map[string]any
	mu     sync.RWMutex
}

// NewMetrics creates a new metrics instance.
func NewMetrics(namespace string) *Metrics {
	return &Metrics{
		ConnectionsActive: NewGauge(namespace+"_connections_active", "Number of active connections"),
		ConnectionsTotal:  NewCounter(namespace+"_connections_total", "Total connections established"),

		MessagesReceived: NewCounterVec(namespace+"_messages_received_total", "Messages received", "type"),
		MessagesSent:     NewCounterVec(namespace+"_messages_sent_total", "Messages sent", "type"),
		MessageLatency:   NewHistogram(namespace+"_message_latency_seconds", "Message processing latency"),

		RenderCount:    NewCounter(namespace+"_render_total", "Total render operations"),
		RenderDuration: NewHistogram(namespace+"_render_duration_seconds", "Render duration"),
		DiffSize:       NewHistogram(namespace+"_diff_size_bytes", "Diff size in bytes"),

		ErrorsTotal: NewCounterVec(namespace+"_errors_total", "Total errors", "type"),
		PanicsTotal: NewCounter(namespace+"_panics_total", "Total panics recovered"),

		MemoryPerSocket:  NewHistogram(namespace+"_memory_per_socket_bytes", "Memory per socket"),
		GoroutinesActive: NewGauge(namespace+"_goroutines_active", "Active goroutines"),

		custom: make(map[string]any),
	}
}

// Handler returns an HTTP handler for metrics.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Output in Prometheus format
		m.writeMetric(w, "connections_active", m.ConnectionsActive.Value())
		m.writeMetric(w, "connections_total", m.ConnectionsTotal.Value())
		m.writeMetric(w, "render_total", m.RenderCount.Value())
		m.writeMetric(w, "panics_total", m.PanicsTotal.Value())
		m.writeMetric(w, "goroutines_active", m.GoroutinesActive.Value())

		// Counter vecs
		for label, value := range m.MessagesReceived.Values() {
			m.writeMetricWithLabel(w, "messages_received_total", "type", label, value)
		}
		for label, value := range m.MessagesSent.Values() {
			m.writeMetricWithLabel(w, "messages_sent_total", "type", label, value)
		}
		for label, value := range m.ErrorsTotal.Values() {
			m.writeMetricWithLabel(w, "errors_total", "type", label, value)
		}

		// Histograms
		m.writeHistogram(w, "message_latency_seconds", m.MessageLatency)
		m.writeHistogram(w, "render_duration_seconds", m.RenderDuration)
		m.writeHistogram(w, "diff_size_bytes", m.DiffSize)
	})
}

func (m *Metrics) writeMetric(w http.ResponseWriter, name string, value float64) {
	fmt.Fprintf(w, "golivekit_%s %f\n", name, value)
}

func (m *Metrics) writeMetricWithLabel(w http.ResponseWriter, name, labelName, labelValue string, value float64) {
	fmt.Fprintf(w, "golivekit_%s{%s=\"%s\"} %f\n", name, labelName, labelValue, value)
}

func (m *Metrics) writeHistogram(w http.ResponseWriter, name string, h *Histogram) {
	stats := h.Stats()
	fmt.Fprintf(w, "golivekit_%s_sum %f\n", name, stats.Sum)
	fmt.Fprintf(w, "golivekit_%s_count %d\n", name, stats.Count)
	fmt.Fprintf(w, "golivekit_%s_min %f\n", name, stats.Min)
	fmt.Fprintf(w, "golivekit_%s_max %f\n", name, stats.Max)
	fmt.Fprintf(w, "golivekit_%s_avg %f\n", name, stats.Avg)
}

// Custom metric operations

func (m *Metrics) SetCustom(name string, value any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.custom[name] = value
}

func (m *Metrics) GetCustom(name string) any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.custom[name]
}

// Counter is a monotonically increasing counter.
type Counter struct {
	name  string
	help  string
	value int64
}

// NewCounter creates a new counter.
func NewCounter(name, help string) *Counter {
	return &Counter{name: name, help: help}
}

// Inc increments the counter by 1.
func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add adds the given value to the counter.
func (c *Counter) Add(delta int64) {
	atomic.AddInt64(&c.value, delta)
}

// Value returns the current counter value.
func (c *Counter) Value() float64 {
	return float64(atomic.LoadInt64(&c.value))
}

// Gauge is a value that can go up and down.
type Gauge struct {
	name  string
	help  string
	value int64
}

// NewGauge creates a new gauge.
func NewGauge(name, help string) *Gauge {
	return &Gauge{name: name, help: help}
}

// Set sets the gauge to a value.
func (g *Gauge) Set(value float64) {
	atomic.StoreInt64(&g.value, int64(value))
}

// Inc increments the gauge by 1.
func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

// Dec decrements the gauge by 1.
func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

// Add adds the given value to the gauge.
func (g *Gauge) Add(delta float64) {
	atomic.AddInt64(&g.value, int64(delta))
}

// Value returns the current gauge value.
func (g *Gauge) Value() float64 {
	return float64(atomic.LoadInt64(&g.value))
}

// CounterVec is a counter with labels.
type CounterVec struct {
	name    string
	help    string
	labels  []string
	values  map[string]*Counter
	mu      sync.RWMutex
}

// NewCounterVec creates a new counter vector.
func NewCounterVec(name, help string, labels ...string) *CounterVec {
	return &CounterVec{
		name:   name,
		help:   help,
		labels: labels,
		values: make(map[string]*Counter),
	}
}

// WithLabel returns a counter for the given label value.
func (cv *CounterVec) WithLabel(value string) *Counter {
	cv.mu.Lock()
	defer cv.mu.Unlock()

	if c, ok := cv.values[value]; ok {
		return c
	}

	c := NewCounter(cv.name, cv.help)
	cv.values[value] = c
	return c
}

// Inc increments the counter for the given label.
func (cv *CounterVec) Inc(label string) {
	cv.WithLabel(label).Inc()
}

// Values returns all counter values.
func (cv *CounterVec) Values() map[string]float64 {
	cv.mu.RLock()
	defer cv.mu.RUnlock()

	result := make(map[string]float64)
	for label, counter := range cv.values {
		result[label] = counter.Value()
	}
	return result
}

// Histogram tracks the distribution of values.
type Histogram struct {
	name   string
	help   string
	values []float64
	sum    float64
	count  int64
	min    float64
	max    float64
	mu     sync.Mutex
}

// NewHistogram creates a new histogram.
func NewHistogram(name, help string) *Histogram {
	return &Histogram{
		name:   name,
		help:   help,
		values: make([]float64, 0),
		min:    -1,
	}
}

// Observe records a value.
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.values = append(h.values, value)
	h.sum += value
	h.count++

	if h.min < 0 || value < h.min {
		h.min = value
	}
	if value > h.max {
		h.max = value
	}

	// Keep only last 10000 values to bound memory
	if len(h.values) > 10000 {
		h.values = h.values[5000:]
	}
}

// ObserveDuration records a duration value.
func (h *Histogram) ObserveDuration(d time.Duration) {
	h.Observe(d.Seconds())
}

// Timer returns a timer that automatically records duration.
func (h *Histogram) Timer() *Timer {
	return &Timer{
		histogram: h,
		start:     time.Now(),
	}
}

// Stats returns histogram statistics.
func (h *Histogram) Stats() HistogramStats {
	h.mu.Lock()
	defer h.mu.Unlock()

	stats := HistogramStats{
		Count: h.count,
		Sum:   h.sum,
		Min:   h.min,
		Max:   h.max,
	}

	if h.count > 0 {
		stats.Avg = h.sum / float64(h.count)
	}

	return stats
}

// HistogramStats contains histogram statistics.
type HistogramStats struct {
	Count int64
	Sum   float64
	Min   float64
	Max   float64
	Avg   float64
}

// Timer tracks operation duration.
type Timer struct {
	histogram *Histogram
	start     time.Time
}

// ObserveDuration records the elapsed time.
func (t *Timer) ObserveDuration() {
	t.histogram.ObserveDuration(time.Since(t.start))
}

// Stop is an alias for ObserveDuration.
func (t *Timer) Stop() {
	t.ObserveDuration()
}

// GlobalMetrics is the default metrics instance.
var GlobalMetrics = NewMetrics("golivekit")

// Helper functions for global metrics

func ConnectionOpened() {
	GlobalMetrics.ConnectionsActive.Inc()
	GlobalMetrics.ConnectionsTotal.Inc()
}

func ConnectionClosed() {
	GlobalMetrics.ConnectionsActive.Dec()
}

func MessageReceived(msgType string) {
	GlobalMetrics.MessagesReceived.Inc(msgType)
}

func MessageSent(msgType string) {
	GlobalMetrics.MessagesSent.Inc(msgType)
}

func RecordError(errType string) {
	GlobalMetrics.ErrorsTotal.Inc(errType)
}

func RecordRender(duration time.Duration, diffSize int) {
	GlobalMetrics.RenderCount.Inc()
	GlobalMetrics.RenderDuration.ObserveDuration(duration)
	GlobalMetrics.DiffSize.Observe(float64(diffSize))
}

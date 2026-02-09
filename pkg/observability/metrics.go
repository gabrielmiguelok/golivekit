// Package observability provides metrics and monitoring for GoliveKit.
package observability

import (
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// MetricType represents the type of metric.
type MetricType int

const (
	MetricCounter MetricType = iota
	MetricGauge
	MetricHistogram
)

// Metric represents a single metric.
type Metric struct {
	Name        string
	Type        MetricType
	Help        string
	Labels      []string
	labelValues map[string]*metricValue
	mu          sync.RWMutex
}

type metricValue struct {
	value    atomic.Int64
	floatVal float64
	floatMu  sync.RWMutex
}

// Counter is a monotonically increasing metric.
type Counter struct {
	value atomic.Int64
}

// Inc increments the counter by 1.
func (c *Counter) Inc() {
	c.value.Add(1)
}

// Add adds the given value to the counter.
func (c *Counter) Add(v int64) {
	c.value.Add(v)
}

// Value returns the current counter value.
func (c *Counter) Value() int64 {
	return c.value.Load()
}

// Gauge is a metric that can go up and down.
type Gauge struct {
	value atomic.Int64
}

// Set sets the gauge to the given value.
func (g *Gauge) Set(v int64) {
	g.value.Store(v)
}

// Inc increments the gauge by 1.
func (g *Gauge) Inc() {
	g.value.Add(1)
}

// Dec decrements the gauge by 1.
func (g *Gauge) Dec() {
	g.value.Add(-1)
}

// Add adds the given value to the gauge.
func (g *Gauge) Add(v int64) {
	g.value.Add(v)
}

// Value returns the current gauge value.
func (g *Gauge) Value() int64 {
	return g.value.Load()
}

// Histogram tracks the distribution of values.
type Histogram struct {
	buckets     []float64
	counts      []atomic.Int64
	sum         atomic.Int64
	count       atomic.Int64
	mu          sync.RWMutex
}

// NewHistogram creates a new histogram with the given buckets.
func NewHistogram(buckets []float64) *Histogram {
	return &Histogram{
		buckets: buckets,
		counts:  make([]atomic.Int64, len(buckets)+1),
	}
}

// Observe records a value.
func (h *Histogram) Observe(v float64) {
	h.count.Add(1)
	h.sum.Add(int64(v * 1000)) // Store as milliseconds for precision

	// Find the bucket
	for i, bucket := range h.buckets {
		if v <= bucket {
			h.counts[i].Add(1)
			return
		}
	}
	h.counts[len(h.buckets)].Add(1) // +Inf bucket
}

// Sum returns the sum of all observed values.
func (h *Histogram) Sum() float64 {
	return float64(h.sum.Load()) / 1000
}

// Count returns the number of observations.
func (h *Histogram) Count() int64 {
	return h.count.Load()
}

// Metrics holds all application metrics.
type Metrics struct {
	// Connection metrics
	ActiveConnections   Gauge
	TotalConnections    Counter
	ConnectionErrors    Counter
	ConnectionsRejected Counter

	// Message metrics
	MessagesReceived   Counter
	MessagesSent       Counter
	MessageErrors      Counter
	MessageLatency     *Histogram

	// Security metrics
	AuthFailures       Counter
	CSRFViolations     Counter
	RateLimitHits      Counter
	OriginBlocked      Counter

	// Component metrics
	ComponentMounts    Counter
	ComponentRenders   Counter
	RenderLatency      *Histogram

	// Custom metrics
	custom map[string]interface{}
	mu     sync.RWMutex
}

// DefaultBuckets are default latency buckets in seconds.
var DefaultBuckets = []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// NewMetrics creates a new metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		MessageLatency: NewHistogram(DefaultBuckets),
		RenderLatency:  NewHistogram(DefaultBuckets),
		custom:         make(map[string]interface{}),
	}
}

// RegisterCounter registers a custom counter.
func (m *Metrics) RegisterCounter(name string) *Counter {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := &Counter{}
	m.custom[name] = c
	return c
}

// RegisterGauge registers a custom gauge.
func (m *Metrics) RegisterGauge(name string) *Gauge {
	m.mu.Lock()
	defer m.mu.Unlock()

	g := &Gauge{}
	m.custom[name] = g
	return g
}

// RegisterHistogram registers a custom histogram.
func (m *Metrics) RegisterHistogram(name string, buckets []float64) *Histogram {
	m.mu.Lock()
	defer m.mu.Unlock()

	h := NewHistogram(buckets)
	m.custom[name] = h
	return h
}

// Handler returns an HTTP handler for Prometheus-style metrics.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		// Write built-in metrics
		writeMetric(w, "golivekit_connections_active", "gauge", m.ActiveConnections.Value())
		writeMetric(w, "golivekit_connections_total", "counter", m.TotalConnections.Value())
		writeMetric(w, "golivekit_connection_errors_total", "counter", m.ConnectionErrors.Value())
		writeMetric(w, "golivekit_connections_rejected_total", "counter", m.ConnectionsRejected.Value())

		writeMetric(w, "golivekit_messages_received_total", "counter", m.MessagesReceived.Value())
		writeMetric(w, "golivekit_messages_sent_total", "counter", m.MessagesSent.Value())
		writeMetric(w, "golivekit_message_errors_total", "counter", m.MessageErrors.Value())

		writeMetric(w, "golivekit_auth_failures_total", "counter", m.AuthFailures.Value())
		writeMetric(w, "golivekit_csrf_violations_total", "counter", m.CSRFViolations.Value())
		writeMetric(w, "golivekit_rate_limit_hits_total", "counter", m.RateLimitHits.Value())
		writeMetric(w, "golivekit_origin_blocked_total", "counter", m.OriginBlocked.Value())

		writeMetric(w, "golivekit_component_mounts_total", "counter", m.ComponentMounts.Value())
		writeMetric(w, "golivekit_component_renders_total", "counter", m.ComponentRenders.Value())

		// Write histograms
		writeHistogram(w, "golivekit_message_latency_seconds", m.MessageLatency)
		writeHistogram(w, "golivekit_render_latency_seconds", m.RenderLatency)
	})
}

func writeMetric(w http.ResponseWriter, name, metricType string, value int64) {
	w.Write([]byte("# TYPE " + name + " " + metricType + "\n"))
	w.Write([]byte(name + " " + strconv.FormatInt(value, 10) + "\n"))
}

func writeHistogram(w http.ResponseWriter, name string, h *Histogram) {
	w.Write([]byte("# TYPE " + name + " histogram\n"))

	cumulative := int64(0)
	for i, bucket := range h.buckets {
		cumulative += h.counts[i].Load()
		bucketStr := strconv.FormatFloat(bucket, 'f', 3, 64)
		w.Write([]byte(name + "_bucket{le=\"" + bucketStr + "\"} " + strconv.FormatInt(cumulative, 10) + "\n"))
	}
	cumulative += h.counts[len(h.buckets)].Load()
	w.Write([]byte(name + "_bucket{le=\"+Inf\"} " + strconv.FormatInt(cumulative, 10) + "\n"))
	w.Write([]byte(name + "_sum " + strconv.FormatFloat(h.Sum(), 'f', 3, 64) + "\n"))
	w.Write([]byte(name + "_count " + strconv.FormatInt(h.Count(), 10) + "\n"))
}

// Timer measures the duration of an operation.
type Timer struct {
	start     time.Time
	histogram *Histogram
}

// NewTimer starts a new timer.
func NewTimer(h *Histogram) *Timer {
	return &Timer{
		start:     time.Now(),
		histogram: h,
	}
}

// ObserveDuration records the elapsed time since the timer was created.
func (t *Timer) ObserveDuration() time.Duration {
	d := time.Since(t.start)
	if t.histogram != nil {
		t.histogram.Observe(d.Seconds())
	}
	return d
}

// Global metrics instance
var globalMetrics *Metrics
var metricsOnce sync.Once

// Global returns the global metrics instance.
func Global() *Metrics {
	metricsOnce.Do(func() {
		globalMetrics = NewMetrics()
	})
	return globalMetrics
}

// Convenience functions using global metrics

// IncConnections increments active connections.
func IncConnections() {
	Global().ActiveConnections.Inc()
	Global().TotalConnections.Inc()
}

// DecConnections decrements active connections.
func DecConnections() {
	Global().ActiveConnections.Dec()
}

// IncMessages increments message counters.
func IncMessages(sent bool) {
	if sent {
		Global().MessagesSent.Inc()
	} else {
		Global().MessagesReceived.Inc()
	}
}

// ObserveMessageLatency records message processing latency.
func ObserveMessageLatency(d time.Duration) {
	Global().MessageLatency.Observe(d.Seconds())
}

// IncSecurityEvent increments security event counters.
func IncSecurityEvent(eventType string) {
	m := Global()
	switch eventType {
	case "auth_failure":
		m.AuthFailures.Inc()
	case "csrf_violation":
		m.CSRFViolations.Inc()
	case "rate_limit":
		m.RateLimitHits.Inc()
	case "origin_blocked":
		m.OriginBlocked.Inc()
	}
}

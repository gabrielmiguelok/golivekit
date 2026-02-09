# observability

The `observability` package provides Prometheus-style metrics for monitoring GoliveKit applications.

## Installation

```go
import "github.com/gabrielmiguelok/golivekit/pkg/observability"
```

## Overview

The package provides:
- Counter, Gauge, and Histogram metric types
- HTTP handler for `/metrics` endpoint
- Built-in GoliveKit metrics
- Custom metric registration

## Metric Types

### Counter

Monotonically increasing value (e.g., request count):

```go
connections := observability.NewCounter(observability.CounterOpts{
    Name: "golivekit_websocket_connections_total",
    Help: "Total number of WebSocket connections",
    Labels: []string{"status"},
})

// Increment
connections.WithLabels("success").Inc()
connections.WithLabels("error").Inc()
```

### Gauge

Value that can go up or down (e.g., current connections):

```go
activeConns := observability.NewGauge(observability.GaugeOpts{
    Name: "golivekit_active_connections",
    Help: "Number of active WebSocket connections",
})

// Set value
activeConns.Set(100)

// Increment/Decrement
activeConns.Inc()
activeConns.Dec()
activeConns.Add(10)
activeConns.Sub(5)
```

### Histogram

Distribution of values (e.g., latencies):

```go
latency := observability.NewHistogram(observability.HistogramOpts{
    Name:    "golivekit_event_latency_seconds",
    Help:    "Event handling latency in seconds",
    Labels:  []string{"event_type"},
    Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
})

// Observe value
start := time.Now()
handleEvent()
latency.WithLabels("click").Observe(time.Since(start).Seconds())
```

## Built-in Metrics

GoliveKit exposes these metrics automatically:

| Metric | Type | Description |
|--------|------|-------------|
| `golivekit_connections_total` | Counter | Total connections by status |
| `golivekit_active_connections` | Gauge | Current active connections |
| `golivekit_messages_total` | Counter | Messages by direction and type |
| `golivekit_message_size_bytes` | Histogram | Message sizes |
| `golivekit_event_latency_seconds` | Histogram | Event handling latency |
| `golivekit_render_latency_seconds` | Histogram | Component render latency |
| `golivekit_diff_size_bytes` | Histogram | Diff payload sizes |
| `golivekit_diff_latency_seconds` | Histogram | Diff computation time |
| `golivekit_errors_total` | Counter | Errors by type |
| `golivekit_pubsub_messages_total` | Counter | PubSub messages |
| `golivekit_presence_count` | Gauge | Tracked presences |
| `golivekit_component_instances` | Gauge | Active component instances |

## HTTP Handler

Expose metrics for Prometheus scraping:

```go
// Create registry
registry := observability.NewRegistry()

// Register default GoliveKit metrics
observability.RegisterGoliveKitMetrics(registry)

// Add custom metrics
registry.Register(myCustomMetric)

// Create handler
http.Handle("/metrics", registry.Handler())
```

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'golivekit'
    static_configs:
      - targets: ['localhost:3000']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Custom Metrics

### Registering Metrics

```go
// Create registry
registry := observability.NewRegistry()

// Create and register counter
pageViews := observability.NewCounter(observability.CounterOpts{
    Name:   "myapp_page_views_total",
    Help:   "Total page views",
    Labels: []string{"page"},
})
registry.Register(pageViews)

// Use in application
pageViews.WithLabels("/home").Inc()
pageViews.WithLabels("/about").Inc()
```

### Timer Helper

```go
// Create histogram
duration := observability.NewHistogram(observability.HistogramOpts{
    Name:    "myapp_operation_duration_seconds",
    Help:    "Operation duration",
    Buckets: observability.DefaultBuckets,
})

// Use timer
timer := observability.NewTimer(duration)
defer timer.ObserveDuration()

// ... do work ...
```

### Batch Metrics

```go
// Create batch of related metrics
type DBMetrics struct {
    Queries       *observability.Counter
    QueryDuration *observability.Histogram
    Connections   *observability.Gauge
    Errors        *observability.Counter
}

func NewDBMetrics(registry *observability.Registry) *DBMetrics {
    m := &DBMetrics{
        Queries: observability.NewCounter(observability.CounterOpts{
            Name:   "db_queries_total",
            Help:   "Total database queries",
            Labels: []string{"operation"},
        }),
        QueryDuration: observability.NewHistogram(observability.HistogramOpts{
            Name:    "db_query_duration_seconds",
            Help:    "Query duration",
            Labels:  []string{"operation"},
            Buckets: observability.DefaultBuckets,
        }),
        Connections: observability.NewGauge(observability.GaugeOpts{
            Name: "db_connections",
            Help: "Active database connections",
        }),
        Errors: observability.NewCounter(observability.CounterOpts{
            Name:   "db_errors_total",
            Help:   "Database errors",
            Labels: []string{"type"},
        }),
    }

    registry.Register(m.Queries)
    registry.Register(m.QueryDuration)
    registry.Register(m.Connections)
    registry.Register(m.Errors)

    return m
}
```

## Bucket Configurations

### Default Buckets

Suitable for most latency measurements:

```go
observability.DefaultBuckets // [.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10]
```

### Linear Buckets

Evenly spaced:

```go
observability.LinearBuckets(0, 100, 10) // [0, 100, 200, ..., 900]
```

### Exponential Buckets

Powers of a base:

```go
observability.ExponentialBuckets(1, 2, 10) // [1, 2, 4, 8, 16, 32, 64, 128, 256, 512]
```

### Custom Buckets

For specific requirements:

```go
observability.HistogramOpts{
    Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
}
```

## Labels

### Best Practices

```go
// Good: Bounded cardinality
requests.WithLabels("GET", "200").Inc()
requests.WithLabels("POST", "201").Inc()

// Bad: Unbounded cardinality (user IDs, timestamps)
requests.WithLabels(userID).Inc() // Don't do this!
```

### Label Values

```go
// Define label values as constants
const (
    StatusSuccess = "success"
    StatusError   = "error"
    StatusTimeout = "timeout"
)

counter.WithLabels(StatusSuccess).Inc()
```

## Grafana Dashboards

### Connection Dashboard

```promql
# Active connections
golivekit_active_connections

# Connection rate (per second)
rate(golivekit_connections_total[5m])

# Error rate
rate(golivekit_connections_total{status="error"}[5m])
  / rate(golivekit_connections_total[5m])
```

### Latency Dashboard

```promql
# p50 event latency
histogram_quantile(0.5, rate(golivekit_event_latency_seconds_bucket[5m]))

# p95 event latency
histogram_quantile(0.95, rate(golivekit_event_latency_seconds_bucket[5m]))

# p99 event latency
histogram_quantile(0.99, rate(golivekit_event_latency_seconds_bucket[5m]))
```

### Message Dashboard

```promql
# Message rate by direction
rate(golivekit_messages_total[5m])

# Average message size
rate(golivekit_message_size_bytes_sum[5m])
  / rate(golivekit_message_size_bytes_count[5m])
```

## Alerting Rules

```yaml
groups:
  - name: golivekit
    rules:
      - alert: HighErrorRate
        expr: rate(golivekit_errors_total[5m]) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High error rate in GoliveKit

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(golivekit_event_latency_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High event latency (p95 > 500ms)

      - alert: ConnectionDrop
        expr: delta(golivekit_active_connections[5m]) < -100
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: Significant connection drop
```

## Integration with Plugin System

```go
type MetricsPlugin struct {
    plugin.BasePlugin
    registry *observability.Registry
    events   *observability.Counter
    latency  *observability.Histogram
}

func (p *MetricsPlugin) Init(app *plugin.App) error {
    p.events = observability.NewCounter(observability.CounterOpts{
        Name:   "golivekit_plugin_events_total",
        Help:   "Events processed by plugins",
        Labels: []string{"event"},
    })
    p.registry.Register(p.events)

    app.Hooks().Register(plugin.HookAfterEvent, "metrics", func(ctx plugin.Context) {
        event := ctx.Get("event").(string)
        p.events.WithLabels(event).Inc()
    })

    return nil
}
```

## Configuration

```go
registry := observability.NewRegistry(observability.RegistryOpts{
    // Include Go runtime metrics
    IncludeGoMetrics: true,

    // Include process metrics (open fds, memory)
    IncludeProcessMetrics: true,

    // Namespace prefix for all metrics
    Namespace: "myapp",

    // Common labels for all metrics
    ConstLabels: map[string]string{
        "service": "golivekit",
        "env":     "production",
    },
})
```

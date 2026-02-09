# health

The `health` package provides health check endpoints for GoliveKit applications, compatible with Kubernetes probes and load balancers.

## Installation

```go
import "github.com/gabrielmiguelok/golivekit/pkg/health"
```

## Overview

Health checks enable:
- Kubernetes liveness and readiness probes
- Load balancer health monitoring
- Dependency status monitoring
- Graceful degradation

## Status Types

| Status | Description |
|--------|-------------|
| `Healthy` | All checks passing |
| `Degraded` | Some non-critical checks failing |
| `Unhealthy` | Critical checks failing |

## Basic Usage

### Creating a Health Checker

```go
checker := health.NewChecker(health.Options{
    Timeout: 5 * time.Second,
})

// Add checks
checker.AddCheck("database", health.DatabaseCheck(db))
checker.AddCheck("redis", health.RedisCheck(redisClient))
checker.AddCheck("websocket", health.WebSocketPoolCheck(pool))
```

### HTTP Handlers

```go
// Register health endpoints
http.Handle("/healthz", checker.Handler())           // Combined check
http.Handle("/healthz/live", checker.LiveHandler())  // Liveness only
http.Handle("/healthz/ready", checker.ReadyHandler()) // Readiness only
```

## Kubernetes Integration

### Liveness Probe

Checks if the application should be restarted:

```yaml
livenessProbe:
  httpGet:
    path: /healthz/live
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 3
```

```go
// Liveness checks (is the app alive?)
checker.AddLivenessCheck("runtime", health.RuntimeCheck())
checker.AddLivenessCheck("deadlock", health.DeadlockCheck())
```

### Readiness Probe

Checks if the application can serve traffic:

```yaml
readinessProbe:
  httpGet:
    path: /healthz/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 3
  failureThreshold: 3
```

```go
// Readiness checks (can we serve traffic?)
checker.AddReadinessCheck("database", health.DatabaseCheck(db))
checker.AddReadinessCheck("cache", health.RedisCheck(redis))
```

## Built-in Checks

### PingCheck

Simple TCP connectivity check:

```go
check := health.PingCheck(health.PingOptions{
    Address: "database:5432",
    Timeout: 2 * time.Second,
})
```

### DatabaseCheck

SQL database health:

```go
check := health.DatabaseCheck(db, health.DatabaseOptions{
    Query:   "SELECT 1",
    Timeout: 3 * time.Second,
})
```

### RedisCheck

Redis connectivity:

```go
check := health.RedisCheck(client, health.RedisOptions{
    Key:     "health:check",
    Timeout: 2 * time.Second,
})
```

### WebSocketPoolCheck

WebSocket connection pool:

```go
check := health.WebSocketPoolCheck(pool, health.PoolOptions{
    MinConnections: 1,
    MaxAge:         time.Hour,
})
```

### MemoryCheck

Memory usage threshold:

```go
check := health.MemoryCheck(health.MemoryOptions{
    MaxHeapBytes:  1 * 1024 * 1024 * 1024, // 1GB
    MaxAllocBytes: 500 * 1024 * 1024,       // 500MB
})
```

### DiskCheck

Disk space threshold:

```go
check := health.DiskCheck(health.DiskOptions{
    Path:         "/data",
    MinFreeBytes: 1 * 1024 * 1024 * 1024, // 1GB minimum
    MinFreePct:   10,                      // 10% minimum
})
```

### GoroutineCheck

Goroutine count threshold:

```go
check := health.GoroutineCheck(health.GoroutineOptions{
    MaxCount:  10000,
    WarnCount: 5000,
})
```

### HTTPCheck

External HTTP service:

```go
check := health.HTTPCheck(health.HTTPOptions{
    URL:            "https://api.example.com/health",
    Method:         "GET",
    Timeout:        5 * time.Second,
    ExpectedStatus: 200,
    Headers:        map[string]string{"Authorization": "Bearer token"},
})
```

## Custom Checks

### Simple Check Function

```go
checker.AddCheck("custom", health.CheckFunc(func(ctx context.Context) health.Result {
    // Perform check
    if err := myService.Ping(ctx); err != nil {
        return health.Result{
            Status:  health.Unhealthy,
            Message: err.Error(),
        }
    }
    return health.Result{
        Status:  health.Healthy,
        Message: "Service responding",
    }
}))
```

### Check with Metadata

```go
checker.AddCheck("queue", health.CheckFunc(func(ctx context.Context) health.Result {
    depth := queue.Depth()
    status := health.Healthy
    if depth > 1000 {
        status = health.Degraded
    }
    if depth > 10000 {
        status = health.Unhealthy
    }

    return health.Result{
        Status:  status,
        Message: fmt.Sprintf("Queue depth: %d", depth),
        Details: map[string]any{
            "depth":    depth,
            "capacity": queue.Capacity(),
            "workers":  queue.Workers(),
        },
    }
}))
```

## Response Format

### Healthy Response

```json
{
    "status": "healthy",
    "checks": {
        "database": {
            "status": "healthy",
            "message": "Connection successful",
            "duration_ms": 5
        },
        "redis": {
            "status": "healthy",
            "message": "PONG received",
            "duration_ms": 2
        }
    },
    "duration_ms": 7
}
```

### Unhealthy Response

```json
{
    "status": "unhealthy",
    "checks": {
        "database": {
            "status": "unhealthy",
            "message": "Connection refused",
            "error": "dial tcp 127.0.0.1:5432: connect: connection refused",
            "duration_ms": 3001
        },
        "redis": {
            "status": "healthy",
            "message": "PONG received",
            "duration_ms": 2
        }
    },
    "duration_ms": 3003
}
```

## Configuration Options

```go
checker := health.NewChecker(health.Options{
    // Global timeout for all checks
    Timeout: 10 * time.Second,

    // Run checks in parallel
    Parallel: true,

    // Cache results for this duration
    CacheDuration: 5 * time.Second,

    // Custom status code mapping
    StatusCodes: map[health.Status]int{
        health.Healthy:   200,
        health.Degraded:  200, // Still serve traffic
        health.Unhealthy: 503,
    },

    // Include detailed output (disable in production for security)
    Verbose: false,

    // Custom response headers
    Headers: map[string]string{
        "X-Health-Version": "1.0",
    },
})
```

## Graceful Shutdown

Integrate with shutdown for clean termination:

```go
// Mark as not ready during shutdown
shutdown.OnShutdown(func() {
    checker.SetStatus(health.Unhealthy)
    // Wait for load balancer to remove from pool
    time.Sleep(10 * time.Second)
})
```

## Metrics Integration

Export health status as Prometheus metrics:

```go
checker.OnStatusChange(func(name string, result health.Result) {
    metrics.HealthCheckStatus.WithLabelValues(name).Set(
        statusToFloat(result.Status),
    )
    metrics.HealthCheckDuration.WithLabelValues(name).Observe(
        result.Duration.Seconds(),
    )
})
```

## Best Practices

1. **Separate liveness from readiness** - Liveness checks restart the pod, readiness removes from service
2. **Keep checks fast** - Set appropriate timeouts (1-5 seconds)
3. **Check dependencies** - Include database, cache, external services
4. **Cache results** - Avoid overloading dependencies
5. **Use degraded status** - For non-critical failures
6. **Monitor check metrics** - Track success rates and durations
7. **Don't expose details in production** - Use `Verbose: false`

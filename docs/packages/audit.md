# audit

The `audit` package provides security audit logging for GoliveKit applications. It tracks security-relevant events with structured logging.

## Installation

```go
import "github.com/gabrielmiguelok/golivekit/pkg/audit"
```

## Overview

Security audit logging is essential for:
- Detecting security incidents
- Compliance requirements (SOC2, HIPAA, PCI)
- Forensic analysis
- Real-time alerting

## Event Types

The package defines 12 security event types:

| Event Type | Severity | Description |
|------------|----------|-------------|
| `AuthSuccess` | Info | Successful authentication |
| `AuthFailure` | Warning | Failed authentication attempt |
| `AuthLogout` | Info | User logout |
| `CSRFViolation` | Critical | CSRF token validation failed |
| `XSSAttempt` | Critical | Potential XSS attack detected |
| `RateLimitExceeded` | Warning | Rate limit threshold exceeded |
| `InvalidInput` | Warning | Malformed or suspicious input |
| `UnauthorizedAccess` | Critical | Access to protected resource denied |
| `SessionHijack` | Critical | Session hijacking attempt detected |
| `SQLInjection` | Critical | SQL injection attempt detected |
| `PathTraversal` | Critical | Path traversal attempt detected |
| `ConfigChange` | Info | Security configuration changed |

## Severity Levels

| Level | Description |
|-------|-------------|
| `Info` | Normal operations, no action needed |
| `Warning` | Suspicious activity, may need review |
| `Critical` | Security incident, immediate action required |

## Basic Usage

### Creating an Audit Logger

```go
// Create with default JSON logger
logger := audit.NewLogger(audit.Options{
    Output: os.Stdout,
})

// Log an event
logger.Log(audit.Event{
    Type:      audit.AuthSuccess,
    UserID:    "user-123",
    IP:        "192.168.1.1",
    UserAgent: "Mozilla/5.0...",
    Details:   map[string]any{"method": "password"},
})
```

### Helper Functions

The package provides helper functions for common events:

```go
// Log authentication failure
audit.LogAuthFailure(logger, audit.AuthFailureDetails{
    UserID:    "user-123",
    IP:        "192.168.1.1",
    Reason:    "invalid_password",
    Attempts:  3,
})

// Log CSRF violation
audit.LogCSRFViolation(logger, audit.CSRFDetails{
    UserID:    "user-123",
    IP:        "192.168.1.1",
    Path:      "/api/transfer",
    Expected:  "abc123",
    Received:  "xyz789",
})

// Log rate limit exceeded
audit.LogRateLimitExceeded(logger, audit.RateLimitDetails{
    UserID:    "user-123",
    IP:        "192.168.1.1",
    Limit:     100,
    Window:    time.Minute,
    Current:   150,
})

// Log unauthorized access
audit.LogUnauthorizedAccess(logger, audit.UnauthorizedDetails{
    UserID:    "user-123",
    IP:        "192.168.1.1",
    Resource:  "/admin/users",
    Required:  "admin",
    Actual:    "user",
})
```

## Logger Implementations

### JSONLogger

Outputs structured JSON logs:

```go
logger := audit.NewJSONLogger(os.Stdout)
```

Output format:
```json
{
    "timestamp": "2024-01-15T10:30:00Z",
    "type": "auth_failure",
    "severity": "warning",
    "user_id": "user-123",
    "ip": "192.168.1.1",
    "user_agent": "Mozilla/5.0...",
    "details": {
        "reason": "invalid_password",
        "attempts": 3
    }
}
```

### FileLogger

Writes to rotating log files:

```go
logger := audit.NewFileLogger(audit.FileLoggerOptions{
    Path:       "/var/log/app/audit.log",
    MaxSize:    100 * 1024 * 1024, // 100MB
    MaxBackups: 10,
    Compress:   true,
})
defer logger.Close()
```

### AsyncLogger

Non-blocking logger with buffering:

```go
logger := audit.NewAsyncLogger(audit.AsyncOptions{
    Logger:     jsonLogger,
    BufferSize: 1000,
    Workers:    4,
})
defer logger.Close() // Flushes remaining events
```

### MultiLogger

Sends events to multiple destinations:

```go
logger := audit.NewMultiLogger(
    audit.NewJSONLogger(os.Stdout),
    audit.NewFileLogger(fileOpts),
    audit.NewHTTPLogger(webhookURL),
)
```

## Middleware Integration

### HTTP Middleware

```go
func AuditMiddleware(logger audit.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Log all requests to sensitive endpoints
            if strings.HasPrefix(r.URL.Path, "/admin") {
                logger.Log(audit.Event{
                    Type:      audit.UnauthorizedAccess,
                    IP:        r.RemoteAddr,
                    UserAgent: r.UserAgent(),
                    Details: map[string]any{
                        "path":   r.URL.Path,
                        "method": r.Method,
                    },
                })
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### GoliveKit Plugin

```go
type AuditPlugin struct {
    plugin.BasePlugin
    logger audit.Logger
}

func (p *AuditPlugin) Init(app *plugin.App) error {
    app.Hooks().Register(plugin.HookOnError, "audit", func(ctx plugin.Context, err error) {
        p.logger.Log(audit.Event{
            Type:     audit.InvalidInput,
            Severity: audit.Warning,
            Details:  map[string]any{"error": err.Error()},
        })
    })
    return nil
}
```

## Alerting

### Critical Event Handler

```go
logger := audit.NewLogger(audit.Options{
    OnCritical: func(event audit.Event) {
        // Send to alerting system
        alerting.Send(alerting.Alert{
            Severity: "critical",
            Title:    fmt.Sprintf("Security: %s", event.Type),
            Details:  event.Details,
        })
    },
})
```

### Webhook Integration

```go
logger := audit.NewHTTPLogger(audit.HTTPLoggerOptions{
    URL:     "https://alerts.example.com/webhook",
    Headers: map[string]string{"Authorization": "Bearer token"},
    Filter: func(e audit.Event) bool {
        return e.Severity == audit.Critical
    },
})
```

## Querying Logs

### Structured Query

```go
// With file logger supporting queries
results, err := logger.Query(audit.Query{
    Types:     []audit.EventType{audit.AuthFailure},
    StartTime: time.Now().Add(-24 * time.Hour),
    EndTime:   time.Now(),
    UserID:    "user-123",
    Limit:     100,
})
```

## Configuration

```go
logger := audit.NewLogger(audit.Options{
    // Output destination
    Output: os.Stdout,

    // Minimum severity to log
    MinSeverity: audit.Warning,

    // Include stack traces for critical events
    IncludeStack: true,

    // Redact sensitive fields
    RedactFields: []string{"password", "token", "secret"},

    // Custom timestamp format
    TimeFormat: time.RFC3339Nano,

    // Include request ID for correlation
    IncludeRequestID: true,
})
```

## Best Practices

1. **Log all authentication events** - Both successes and failures
2. **Include context** - IP, user agent, request ID
3. **Set appropriate severity** - Reserve critical for actual incidents
4. **Use async logging** - Don't block request handling
5. **Rotate logs** - Prevent disk exhaustion
6. **Monitor critical events** - Set up real-time alerts
7. **Redact sensitive data** - Never log passwords or tokens
8. **Correlate with request IDs** - Enable tracing across services

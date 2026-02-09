# Security Policy - GoliveKit

This document describes the security features, configurations, and best practices for GoliveKit.

## Table of Contents

1. [Security Features](#security-features)
2. [Quick Start - Secure Configuration](#quick-start)
3. [OWASP Top 10 Coverage](#owasp-coverage)
4. [Configuration Reference](#configuration-reference)
5. [Reporting Vulnerabilities](#reporting-vulnerabilities)

---

## Security Features

### WebSocket Security

**Origin Validation** (P0 - Critical)
- All WebSocket connections validate the `Origin` header
- Prevents WebSocket hijacking attacks (CSRF over WebSocket)
- Configure allowed origins explicitly in production

```go
wsConfig := &transport.WebSocketConfig{
    AllowedOrigins:  []string{"https://yourdomain.com"},
    InsecureDevMode: false, // NEVER true in production
}
transport := transport.NewWebSocketTransportWithConfig(config, wsConfig)
```

### CORS Protection

**SSE CORS** (P0 - Critical)
- No wildcard `*` CORS by default
- Explicit origin allowlist required

```go
sseConfig := &transport.SSEConfig{
    AllowedOrigins:   []string{"https://yourdomain.com"},
    AllowCredentials: true,
}
```

### Long-Polling Security

**Signed Client IDs** (P0 - Critical)
- Client IDs are HMAC-signed to prevent enumeration
- Optional authentication token validation

```go
lpConfig := &transport.LongPollingConfig{
    RequireAuth: true,
    TokenValidator: func(token string) (bool, error) {
        return validateToken(token), nil
    },
    HMACSecret:     []byte("your-secret-key"),
    ClientIDExpiry: 24 * time.Hour,
}
```

### CSRF Protection

**Enabled by Default**
- Double-submit cookie pattern
- Automatic token generation and validation
- Configurable expiration and cookie settings

```go
csrf := security.NewCSRFProtection(security.CSRFConfig{
    Secret:     []byte("your-32-byte-secret-key"),
    MaxAge:     24 * time.Hour,
    SameSite:   http.SameSiteLaxMode,
    Secure:     true, // Requires HTTPS
})
router.Use(csrf.Middleware())
```

### Security Headers

**OWASP-Recommended Headers**
```
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
Strict-Transport-Security: max-age=31536000; includeSubDomains
Content-Security-Policy: default-src 'self'; script-src 'self' 'nonce-xxx'; ...
```

### Rate Limiting

**Distributed Rate Limiting**
```go
limiter := limits.NewMemoryRateLimiter() // or Redis-based

allowed, err := limiter.Allow(ctx, "user:123", 100, time.Minute)
if !allowed {
    // Rate limit exceeded
}
```

**Connection Limiting**
```go
connLimiter := limits.NewConnectionLimiter(100) // max 100 per IP
router.Use(connLimiter.Middleware())
```

### Audit Logging

**Security Event Logging**
```go
logger := audit.NewAsyncLogger(audit.NewFileLogger("/var/log/security.log"), 1000)

logger.Log(audit.SecurityEvent{
    EventType: audit.EventAuthFailure,
    SourceIP:  clientIP,
    Severity:  audit.SeverityWarning,
    Details:   map[string]interface{}{"reason": "invalid token"},
})
```

---

## Quick Start

### Production Configuration

```go
import (
    "github.com/gabrielmiguelok/golivekit/pkg/core"
    "github.com/gabrielmiguelok/golivekit/pkg/router"
    "github.com/gabrielmiguelok/golivekit/pkg/security"
)

func main() {
    // Use production config
    config := core.ProductionConfig()

    r := router.New()

    // Security middleware (order matters!)
    r.Use(router.SecureHeaders())
    r.Use(limits.NewConnectionLimiter(100).Middleware())
    r.Use(security.NewCSRFProtection(security.DefaultCSRFConfig()).Middleware())

    // Configure WebSocket with allowed origins
    transport.SetDefaultWebSocketConfig(&transport.WebSocketConfig{
        AllowedOrigins:  []string{"https://yourdomain.com"},
        InsecureDevMode: false,
    })

    // Health checks
    healthChecker := health.DefaultChecker("1.0.0")
    r.Handle("/health/live", healthChecker.LivenessHandler())
    r.Handle("/health/ready", healthChecker.ReadinessHandler())

    // Your routes...
    r.Live("/", NewHomeComponent)

    http.ListenAndServeTLS(":443", "cert.pem", "key.pem", r)
}
```

### Development Configuration

```go
config := core.DevelopmentConfig() // Relaxed security for dev
```

---

## OWASP Coverage

| OWASP Top 10 | GoliveKit Protection | Status |
|--------------|---------------------|--------|
| A01 Broken Access Control | CSRF protection, origin validation | ✅ |
| A02 Cryptographic Failures | Secure token generation, HMAC signing | ✅ |
| A03 Injection | HTML auto-sanitization, template escaping | ✅ |
| A04 Insecure Design | Secure defaults, config validation | ✅ |
| A05 Security Misconfiguration | Secure headers, CSP with nonce | ✅ |
| A06 Vulnerable Components | Minimal dependencies | ✅ |
| A07 Auth Failures | Rate limiting, audit logging | ✅ |
| A08 Data Integrity Failures | CSRF, signed client IDs | ✅ |
| A09 Logging Failures | Audit logging package | ✅ |
| A10 SSRF | URL validation in templates | ⚠️ Manual |

---

## Configuration Reference

### TimeoutConfig

```go
type TimeoutConfig struct {
    RequestTimeout   time.Duration // 30s - Overall HTTP timeout
    ComponentMount   time.Duration // 5s  - Mount() timeout
    ComponentRender  time.Duration // 2s  - Render() timeout
    ComponentEvent   time.Duration // 3s  - HandleEvent() timeout
    PubSubPublish    time.Duration // 1s  - Publish timeout
    WebSocketRead    time.Duration // 60s - WS read timeout
    WebSocketWrite   time.Duration // 10s - WS write timeout
    GracefulShutdown time.Duration // 30s - Shutdown timeout
}
```

### SecurityConfig

```go
type SecurityConfig struct {
    CSRFEnabled         bool     // true  - Enable CSRF protection
    CSRFSecret          []byte   // Required when CSRF enabled
    AllowedOrigins      []string // nil   - Same-origin only
    InsecureDevMode     bool     // false - NEVER true in production
    RateLimitEnabled    bool     // true  - Enable rate limiting
    RateLimitPerSecond  int      // 100   - Requests per second
    MaxConnectionsPerIP int      // 100   - Connection limit
    AutoSanitize        bool     // true  - Auto HTML sanitization
    SecureHeaders       bool     // true  - Add security headers
    AuditLogging        bool     // true  - Enable audit logs
}
```

---

## Security Checklist for Production

- [ ] Set `InsecureDevMode: false` everywhere
- [ ] Configure explicit `AllowedOrigins`
- [ ] Generate secure `CSRFSecret` (32+ bytes)
- [ ] Enable HTTPS with valid certificates
- [ ] Set `Secure: true` for cookies
- [ ] Configure rate limiting
- [ ] Enable audit logging
- [ ] Review CSP for your application
- [ ] Set up health checks
- [ ] Configure graceful shutdown
- [ ] Run security tests: `go test ./... -race`
- [ ] Run vulnerability scan: `govulncheck ./...`

---

## Reporting Vulnerabilities

If you discover a security vulnerability, please report it responsibly:

1. **Do NOT** open a public issue
2. Email security@yourdomain.com with:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

We will respond within 48 hours and work with you on a fix.

---

## Version History

| Version | Security Updates |
|---------|-----------------|
| 1.1.0 | WebSocket origin validation, SSE CORS fix, PubSub race fix |
| 1.0.0 | Initial release with basic security features |

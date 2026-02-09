// Package core provides core types and interfaces for GoliveKit.
// This file defines timeout and configuration settings.
package core

import (
	"time"
)

// TimeoutConfig configures timeouts for various operations.
type TimeoutConfig struct {
	// RequestTimeout is the overall timeout for HTTP requests.
	RequestTimeout time.Duration

	// ComponentMount is the timeout for component Mount() calls.
	ComponentMount time.Duration

	// ComponentRender is the timeout for component Render() calls.
	ComponentRender time.Duration

	// ComponentEvent is the timeout for HandleEvent() calls.
	ComponentEvent time.Duration

	// PubSubPublish is the timeout for pubsub Publish() calls.
	PubSubPublish time.Duration

	// WebSocketRead is the read timeout for WebSocket connections.
	WebSocketRead time.Duration

	// WebSocketWrite is the write timeout for WebSocket connections.
	WebSocketWrite time.Duration

	// SessionCleanup is the interval for cleaning up inactive sessions.
	SessionCleanup time.Duration

	// GracefulShutdown is the timeout for graceful shutdown.
	GracefulShutdown time.Duration
}

// DefaultTimeoutConfig returns secure default timeout configuration.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		RequestTimeout:   30 * time.Second,
		ComponentMount:   5 * time.Second,
		ComponentRender:  2 * time.Second,
		ComponentEvent:   3 * time.Second,
		PubSubPublish:    1 * time.Second,
		WebSocketRead:    60 * time.Second,
		WebSocketWrite:   10 * time.Second,
		SessionCleanup:   5 * time.Minute,
		GracefulShutdown: 30 * time.Second,
	}
}

// StrictTimeoutConfig returns stricter timeouts for high-security environments.
func StrictTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		RequestTimeout:   15 * time.Second,
		ComponentMount:   2 * time.Second,
		ComponentRender:  1 * time.Second,
		ComponentEvent:   2 * time.Second,
		PubSubPublish:    500 * time.Millisecond,
		WebSocketRead:    30 * time.Second,
		WebSocketWrite:   5 * time.Second,
		SessionCleanup:   2 * time.Minute,
		GracefulShutdown: 15 * time.Second,
	}
}

// RelaxedTimeoutConfig returns more relaxed timeouts for development.
func RelaxedTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		RequestTimeout:   120 * time.Second,
		ComponentMount:   30 * time.Second,
		ComponentRender:  10 * time.Second,
		ComponentEvent:   30 * time.Second,
		PubSubPublish:    5 * time.Second,
		WebSocketRead:    300 * time.Second,
		WebSocketWrite:   30 * time.Second,
		SessionCleanup:   30 * time.Minute,
		GracefulShutdown: 60 * time.Second,
	}
}

// SecurityConfig configures security settings.
type SecurityConfig struct {
	// CSRF enables CSRF protection.
	CSRFEnabled bool

	// CSRFSecret is the secret key for CSRF tokens.
	CSRFSecret []byte

	// CSRFTokenExpiry is how long CSRF tokens are valid.
	CSRFTokenExpiry time.Duration

	// AllowedOrigins for WebSocket and CORS.
	AllowedOrigins []string

	// InsecureDevMode disables security checks (ONLY for development!).
	InsecureDevMode bool

	// RateLimitEnabled enables rate limiting.
	RateLimitEnabled bool

	// RateLimitPerSecond is the rate limit per IP per second.
	RateLimitPerSecond int

	// MaxConnectionsPerIP limits connections per IP.
	MaxConnectionsPerIP int

	// AutoSanitize enables automatic HTML sanitization.
	AutoSanitize bool

	// SecureHeaders enables security response headers.
	SecureHeaders bool

	// AuditLogging enables security event logging.
	AuditLogging bool
}

// DefaultSecurityConfig returns secure default configuration.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		CSRFEnabled:         true,
		CSRFTokenExpiry:     24 * time.Hour,
		AllowedOrigins:      nil, // Same-origin only
		InsecureDevMode:     false,
		RateLimitEnabled:    true,
		RateLimitPerSecond:  100,
		MaxConnectionsPerIP: 100,
		AutoSanitize:        true,
		SecureHeaders:       true,
		AuditLogging:        true,
	}
}

// DevelopmentSecurityConfig returns relaxed config for development.
func DevelopmentSecurityConfig() SecurityConfig {
	return SecurityConfig{
		CSRFEnabled:         false,
		AllowedOrigins:      []string{"*"},
		InsecureDevMode:     true,
		RateLimitEnabled:    false,
		MaxConnectionsPerIP: 0, // Unlimited
		AutoSanitize:        false,
		SecureHeaders:       false,
		AuditLogging:        false,
	}
}

// Config combines all configuration settings.
type Config struct {
	Timeouts TimeoutConfig
	Security SecurityConfig

	// Server settings
	Address string
	Debug   bool

	// Resource limits
	MaxMessageSize   int64
	MaxConnections   int
	MaxSubscriptions int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Timeouts:         DefaultTimeoutConfig(),
		Security:         DefaultSecurityConfig(),
		Address:          ":3000",
		Debug:            false,
		MaxMessageSize:   1024 * 1024,     // 1MB
		MaxConnections:   10000,
		MaxSubscriptions: 100,
	}
}

// ProductionConfig returns configuration optimized for production.
func ProductionConfig() Config {
	return Config{
		Timeouts:         DefaultTimeoutConfig(),
		Security:         DefaultSecurityConfig(),
		Address:          ":443",
		Debug:            false,
		MaxMessageSize:   512 * 1024, // 512KB
		MaxConnections:   50000,
		MaxSubscriptions: 50,
	}
}

// DevelopmentConfig returns configuration optimized for development.
func DevelopmentConfig() Config {
	return Config{
		Timeouts:         RelaxedTimeoutConfig(),
		Security:         DevelopmentSecurityConfig(),
		Address:          ":3000",
		Debug:            true,
		MaxMessageSize:   10 * 1024 * 1024, // 10MB
		MaxConnections:   1000,
		MaxSubscriptions: 1000,
	}
}

// Validate validates the configuration.
func (c Config) Validate() error {
	if c.Security.CSRFEnabled && len(c.Security.CSRFSecret) == 0 {
		return ErrCSRFSecretRequired
	}
	if c.MaxMessageSize <= 0 {
		return ErrInvalidMaxMessageSize
	}
	return nil
}

// Configuration errors.
var (
	ErrCSRFSecretRequired    = configError("CSRF secret is required when CSRF is enabled")
	ErrInvalidMaxMessageSize = configError("MaxMessageSize must be positive")
)

type configError string

func (e configError) Error() string { return string(e) }

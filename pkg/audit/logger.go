// Package audit provides security audit logging for GoliveKit.
package audit

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// Event types for security auditing.
const (
	EventAuthFailure        = "auth_failure"
	EventAuthSuccess        = "auth_success"
	EventCSRFViolation      = "csrf_violation"
	EventRateLimitExceeded  = "rate_limit_exceeded"
	EventOriginBlocked      = "origin_blocked"
	EventSanitizationApplied = "sanitization_applied"
	EventSessionCreated     = "session_created"
	EventSessionDestroyed   = "session_destroyed"
	EventWebSocketConnect   = "websocket_connect"
	EventWebSocketDisconnect = "websocket_disconnect"
	EventInvalidInput       = "invalid_input"
	EventUnauthorizedAccess = "unauthorized_access"
)

// Severity levels for security events.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// SecurityEvent represents a security-relevant event for audit logging.
type SecurityEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   string                 `json:"event_type"`
	SourceIP    string                 `json:"source_ip"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	ComponentID string                 `json:"component_id,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	Path        string                 `json:"path,omitempty"`
	Method      string                 `json:"method,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Severity    string                 `json:"severity"`
}

// Logger is the interface for audit logging implementations.
type Logger interface {
	// Log records a security event.
	Log(event SecurityEvent)

	// LogWithContext records a security event with context.
	LogWithContext(ctx context.Context, event SecurityEvent)

	// Close flushes any pending logs and closes the logger.
	Close() error
}

// JSONLogger logs security events as JSON to an io.Writer.
type JSONLogger struct {
	encoder *json.Encoder
	writer  io.Writer
	mu      sync.Mutex
}

// NewJSONLogger creates a new JSON audit logger.
func NewJSONLogger(w io.Writer) *JSONLogger {
	return &JSONLogger{
		encoder: json.NewEncoder(w),
		writer:  w,
	}
}

// NewFileLogger creates a logger that writes to a file.
func NewFileLogger(path string) (*JSONLogger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return NewJSONLogger(f), nil
}

// NewStdLogger creates a logger that writes to stdout.
func NewStdLogger() *JSONLogger {
	return NewJSONLogger(os.Stdout)
}

// Log records a security event.
func (l *JSONLogger) Log(event SecurityEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if err := l.encoder.Encode(event); err != nil {
		log.Printf("audit: failed to encode event: %v", err)
	}
}

// LogWithContext records a security event with context.
func (l *JSONLogger) LogWithContext(ctx context.Context, event SecurityEvent) {
	// Extract request ID from context if available
	if reqID, ok := ctx.Value(requestIDKey{}).(string); ok && event.RequestID == "" {
		event.RequestID = reqID
	}
	l.Log(event)
}

// Close closes the logger.
func (l *JSONLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if closer, ok := l.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type requestIDKey struct{}

// MultiLogger logs to multiple destinations.
type MultiLogger struct {
	loggers []Logger
}

// NewMultiLogger creates a logger that writes to multiple destinations.
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

// Log records a security event to all loggers.
func (m *MultiLogger) Log(event SecurityEvent) {
	for _, l := range m.loggers {
		l.Log(event)
	}
}

// LogWithContext records a security event with context to all loggers.
func (m *MultiLogger) LogWithContext(ctx context.Context, event SecurityEvent) {
	for _, l := range m.loggers {
		l.LogWithContext(ctx, event)
	}
}

// Close closes all loggers.
func (m *MultiLogger) Close() error {
	var lastErr error
	for _, l := range m.loggers {
		if err := l.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// NopLogger is a no-op logger for testing or disabling audit logs.
type NopLogger struct{}

// NewNopLogger creates a no-op logger.
func NewNopLogger() *NopLogger {
	return &NopLogger{}
}

// Log does nothing.
func (n *NopLogger) Log(event SecurityEvent) {}

// LogWithContext does nothing.
func (n *NopLogger) LogWithContext(ctx context.Context, event SecurityEvent) {}

// Close does nothing.
func (n *NopLogger) Close() error { return nil }

// AsyncLogger wraps a logger with async buffered writes.
type AsyncLogger struct {
	logger  Logger
	events  chan SecurityEvent
	done    chan struct{}
	wg      sync.WaitGroup
}

// NewAsyncLogger creates an async wrapper around a logger.
func NewAsyncLogger(logger Logger, bufferSize int) *AsyncLogger {
	if bufferSize <= 0 {
		bufferSize = 1000
	}
	l := &AsyncLogger{
		logger: logger,
		events: make(chan SecurityEvent, bufferSize),
		done:   make(chan struct{}),
	}
	l.wg.Add(1)
	go l.worker()
	return l
}

// worker processes events from the buffer.
func (a *AsyncLogger) worker() {
	defer a.wg.Done()
	for {
		select {
		case event := <-a.events:
			a.logger.Log(event)
		case <-a.done:
			// Drain remaining events
			for {
				select {
				case event := <-a.events:
					a.logger.Log(event)
				default:
					return
				}
			}
		}
	}
}

// Log queues a security event for async processing.
func (a *AsyncLogger) Log(event SecurityEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	select {
	case a.events <- event:
	default:
		// Buffer full, log synchronously to prevent data loss
		a.logger.Log(event)
	}
}

// LogWithContext queues a security event with context.
func (a *AsyncLogger) LogWithContext(ctx context.Context, event SecurityEvent) {
	if reqID, ok := ctx.Value(requestIDKey{}).(string); ok && event.RequestID == "" {
		event.RequestID = reqID
	}
	a.Log(event)
}

// Close stops the async worker and flushes pending events.
func (a *AsyncLogger) Close() error {
	close(a.done)
	a.wg.Wait()
	return a.logger.Close()
}

// Helper functions for common events

// LogAuthFailure logs an authentication failure event.
func LogAuthFailure(logger Logger, ip, userAgent, reason string) {
	logger.Log(SecurityEvent{
		EventType: EventAuthFailure,
		SourceIP:  ip,
		UserAgent: userAgent,
		Severity:  SeverityWarning,
		Details:   map[string]interface{}{"reason": reason},
	})
}

// LogCSRFViolation logs a CSRF violation event.
func LogCSRFViolation(logger Logger, ip, path, method string) {
	logger.Log(SecurityEvent{
		EventType: EventCSRFViolation,
		SourceIP:  ip,
		Path:      path,
		Method:    method,
		Severity:  SeverityWarning,
	})
}

// LogRateLimitExceeded logs a rate limit exceeded event.
func LogRateLimitExceeded(logger Logger, ip, path string, violations int) {
	logger.Log(SecurityEvent{
		EventType: EventRateLimitExceeded,
		SourceIP:  ip,
		Path:      path,
		Severity:  SeverityWarning,
		Details:   map[string]interface{}{"violations": violations},
	})
}

// LogOriginBlocked logs a blocked origin event.
func LogOriginBlocked(logger Logger, ip, origin, host string) {
	logger.Log(SecurityEvent{
		EventType: EventOriginBlocked,
		SourceIP:  ip,
		Severity:  SeverityWarning,
		Details:   map[string]interface{}{"origin": origin, "host": host},
	})
}

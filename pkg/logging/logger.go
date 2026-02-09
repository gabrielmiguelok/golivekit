// Package logging provides structured logging for GoliveKit.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// Logger is the interface for structured logging.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger
	WithContext(ctx context.Context) Logger
}

// Field represents a log field.
type Field struct {
	Key   string
	Value any
}

// Common field constructors

func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// SlogLogger implements Logger using slog.
type SlogLogger struct {
	logger *slog.Logger
	ctx    context.Context
}

// NewSlogLogger creates a new slog-based logger.
func NewSlogLogger(opts ...LoggerOption) *SlogLogger {
	config := &loggerConfig{
		level:  slog.LevelInfo,
		output: os.Stdout,
		json:   false,
	}

	for _, opt := range opts {
		opt(config)
	}

	var handler slog.Handler
	if config.json {
		handler = slog.NewJSONHandler(config.output, &slog.HandlerOptions{
			Level:     config.level,
			AddSource: config.addSource,
		})
	} else {
		handler = slog.NewTextHandler(config.output, &slog.HandlerOptions{
			Level:     config.level,
			AddSource: config.addSource,
		})
	}

	return &SlogLogger{
		logger: slog.New(handler),
		ctx:    context.Background(),
	}
}

type loggerConfig struct {
	level     slog.Level
	output    io.Writer
	json      bool
	addSource bool
}

// LoggerOption configures the logger.
type LoggerOption func(*loggerConfig)

// WithLevel sets the log level.
func WithLevel(level slog.Level) LoggerOption {
	return func(c *loggerConfig) {
		c.level = level
	}
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) LoggerOption {
	return func(c *loggerConfig) {
		c.output = w
	}
}

// WithJSON enables JSON output.
func WithJSON() LoggerOption {
	return func(c *loggerConfig) {
		c.json = true
	}
}

// WithSource adds source location to logs.
func WithSource() LoggerOption {
	return func(c *loggerConfig) {
		c.addSource = true
	}
}

func (l *SlogLogger) toAttrs(fields []Field) []any {
	attrs := make([]any, 0, len(fields)*2)
	for _, f := range fields {
		attrs = append(attrs, f.Key, f.Value)
	}
	return attrs
}

// Debug logs a debug message.
func (l *SlogLogger) Debug(msg string, fields ...Field) {
	l.logger.DebugContext(l.ctx, msg, l.toAttrs(fields)...)
}

// Info logs an info message.
func (l *SlogLogger) Info(msg string, fields ...Field) {
	l.logger.InfoContext(l.ctx, msg, l.toAttrs(fields)...)
}

// Warn logs a warning message.
func (l *SlogLogger) Warn(msg string, fields ...Field) {
	l.logger.WarnContext(l.ctx, msg, l.toAttrs(fields)...)
}

// Error logs an error message.
func (l *SlogLogger) Error(msg string, fields ...Field) {
	l.logger.ErrorContext(l.ctx, msg, l.toAttrs(fields)...)
}

// With returns a logger with additional fields.
func (l *SlogLogger) With(fields ...Field) Logger {
	return &SlogLogger{
		logger: l.logger.With(l.toAttrs(fields)...),
		ctx:    l.ctx,
	}
}

// WithContext returns a logger with context.
func (l *SlogLogger) WithContext(ctx context.Context) Logger {
	return &SlogLogger{
		logger: l.logger,
		ctx:    ctx,
	}
}

// Context helpers

type loggerContextKey struct{}

// ContextWithLogger adds a logger to the context.
func ContextWithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// LoggerFromContext retrieves a logger from context.
func LoggerFromContext(ctx context.Context) Logger {
	logger, _ := ctx.Value(loggerContextKey{}).(Logger)
	return logger
}

// L is a shorthand for LoggerFromContext.
func L(ctx context.Context) Logger {
	logger := LoggerFromContext(ctx)
	if logger == nil {
		return DefaultLogger
	}
	return logger
}

// DefaultLogger is the default global logger.
var DefaultLogger Logger = NewSlogLogger()

// SetDefault sets the default logger.
func SetDefault(logger Logger) {
	DefaultLogger = logger
}

// Debug logs using the default logger.
func Debug(msg string, fields ...Field) {
	DefaultLogger.Debug(msg, fields...)
}

// Info logs using the default logger.
func Info(msg string, fields ...Field) {
	DefaultLogger.Info(msg, fields...)
}

// Warn logs using the default logger.
func Warn(msg string, fields ...Field) {
	DefaultLogger.Warn(msg, fields...)
}

// Error logs using the default logger.
func Error(msg string, fields ...Field) {
	DefaultLogger.Error(msg, fields...)
}

// NopLogger is a logger that does nothing.
type NopLogger struct{}

func (NopLogger) Debug(msg string, fields ...Field) {}
func (NopLogger) Info(msg string, fields ...Field)  {}
func (NopLogger) Warn(msg string, fields ...Field)  {}
func (NopLogger) Error(msg string, fields ...Field) {}
func (l NopLogger) With(fields ...Field) Logger     { return l }
func (l NopLogger) WithContext(ctx context.Context) Logger { return l }

// RequestLogger logs HTTP requests.
func RequestLogger(logger Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create request ID
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = generateRequestID()
			}

			// Add logger to context
			reqLogger := logger.With(
				String("request_id", reqID),
				String("method", r.Method),
				String("path", r.URL.Path),
			)
			ctx := ContextWithLogger(r.Context(), reqLogger)

			// Wrap response writer
			rw := &responseWriter{ResponseWriter: w, status: 200}

			// Log request start
			reqLogger.Info("request started")

			// Process request
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Log request end
			reqLogger.Info("request completed",
				Int("status", rw.status),
				Duration("duration", time.Since(start)),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

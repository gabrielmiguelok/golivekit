// Package tracing provides distributed tracing for GoliveKit.
package tracing

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Tracer provides distributed tracing functionality.
type Tracer struct {
	serviceName string
	spans       sync.Map
}

// NewTracer creates a new tracer.
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		serviceName: serviceName,
	}
}

// StartSpan starts a new span.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, *Span) {
	config := &spanConfig{}
	for _, opt := range opts {
		opt(config)
	}

	span := &Span{
		TraceID:   getTraceID(ctx),
		SpanID:    generateSpanID(),
		ParentID:  getSpanID(ctx),
		Name:      name,
		Service:   t.serviceName,
		StartTime: time.Now(),
		Tags:      make(map[string]string),
		Events:    make([]SpanEvent, 0),
	}

	// Add tags from config
	for k, v := range config.tags {
		span.Tags[k] = v
	}

	// Store span
	t.spans.Store(span.SpanID, span)

	// Add to context
	ctx = context.WithValue(ctx, traceIDKey{}, span.TraceID)
	ctx = context.WithValue(ctx, spanIDKey{}, span.SpanID)
	ctx = context.WithValue(ctx, spanKey{}, span)

	return ctx, span
}

// Span represents a trace span.
type Span struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Name      string
	Service   string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Status    SpanStatus
	Tags      map[string]string
	Events    []SpanEvent
	mu        sync.Mutex
}

// SpanStatus indicates span completion status.
type SpanStatus int

const (
	StatusUnset SpanStatus = iota
	StatusOK
	StatusError
)

// SpanEvent represents an event within a span.
type SpanEvent struct {
	Name      string
	Timestamp time.Time
	Attrs     map[string]string
}

// End completes the span.
func (s *Span) End() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)

	if s.Status == StatusUnset {
		s.Status = StatusOK
	}
}

// SetTag sets a tag on the span.
func (s *Span) SetTag(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Tags[key] = value
}

// SetStatus sets the span status.
func (s *Span) SetStatus(status SpanStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
}

// SetError marks the span as errored.
func (s *Span) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = StatusError
	s.Tags["error"] = err.Error()
}

// AddEvent adds an event to the span.
func (s *Span) AddEvent(name string, attrs map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Events = append(s.Events, SpanEvent{
		Name:      name,
		Timestamp: time.Now(),
		Attrs:     attrs,
	})
}

// SpanOption configures a span.
type SpanOption func(*spanConfig)

type spanConfig struct {
	tags map[string]string
}

// WithTag adds a tag to the span.
func WithTag(key, value string) SpanOption {
	return func(c *spanConfig) {
		if c.tags == nil {
			c.tags = make(map[string]string)
		}
		c.tags[key] = value
	}
}

// Context keys

type traceIDKey struct{}
type spanIDKey struct{}
type spanKey struct{}

// getTraceID extracts trace ID from context or generates a new one.
func getTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey{}).(string); ok {
		return id
	}
	return generateTraceID()
}

// getSpanID extracts span ID from context.
func getSpanID(ctx context.Context) string {
	if id, ok := ctx.Value(spanIDKey{}).(string); ok {
		return id
	}
	return ""
}

// SpanFromContext retrieves the current span from context.
func SpanFromContext(ctx context.Context) *Span {
	span, _ := ctx.Value(spanKey{}).(*Span)
	return span
}

// ID generation

var (
	idCounter uint64
	idMu      sync.Mutex
)

func generateTraceID() string {
	idMu.Lock()
	idCounter++
	id := idCounter
	idMu.Unlock()
	return fmt.Sprintf("trace-%d-%d", time.Now().UnixNano(), id)
}

func generateSpanID() string {
	idMu.Lock()
	idCounter++
	id := idCounter
	idMu.Unlock()
	return fmt.Sprintf("span-%d-%d", time.Now().UnixNano(), id)
}

// TracingMiddleware adds tracing to HTTP requests.
func TracingMiddleware(tracer *Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.StartSpan(r.Context(), "http.request",
				WithTag("http.method", r.Method),
				WithTag("http.url", r.URL.String()),
				WithTag("http.host", r.Host),
			)
			defer span.End()

			// Wrap response writer to capture status
			rw := &tracingResponseWriter{ResponseWriter: w, status: 200}

			// Process request
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Add response tags
			span.SetTag("http.status_code", fmt.Sprintf("%d", rw.status))

			if rw.status >= 400 {
				span.SetStatus(StatusError)
			}
		})
	}
}

type tracingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *tracingResponseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// PropagationHeader is the header used for trace propagation.
const PropagationHeader = "X-Trace-ID"

// InjectTraceID injects trace ID into HTTP headers.
func InjectTraceID(ctx context.Context, headers http.Header) {
	if traceID := getTraceID(ctx); traceID != "" {
		headers.Set(PropagationHeader, traceID)
	}
}

// ExtractTraceID extracts trace ID from HTTP headers.
func ExtractTraceID(headers http.Header) string {
	return headers.Get(PropagationHeader)
}

// GlobalTracer is the default tracer.
var GlobalTracer = NewTracer("golivekit")

// StartSpan starts a span using the global tracer.
func StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, *Span) {
	return GlobalTracer.StartSpan(ctx, name, opts...)
}

// Recorder collects spans for export.
type Recorder struct {
	spans []*Span
	mu    sync.Mutex
}

// NewRecorder creates a new span recorder.
func NewRecorder() *Recorder {
	return &Recorder{
		spans: make([]*Span, 0),
	}
}

// Record adds a span to the recorder.
func (r *Recorder) Record(span *Span) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = append(r.spans, span)
}

// Spans returns all recorded spans.
func (r *Recorder) Spans() []*Span {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]*Span, len(r.spans))
	copy(result, r.spans)
	return result
}

// Clear removes all recorded spans.
func (r *Recorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = r.spans[:0]
}

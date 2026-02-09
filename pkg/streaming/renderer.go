// Package streaming provides streaming SSR capabilities for GoliveKit.
// It enables progressive rendering with suspense boundaries, allowing
// faster time-to-first-byte while async content loads in the background.
package streaming

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// Renderer provides streaming server-side rendering.
type Renderer struct {
	flusher http.Flusher
	writer  io.Writer
	mu      sync.Mutex
}

// NewRenderer creates a new streaming renderer.
func NewRenderer(w http.ResponseWriter) (*Renderer, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported: ResponseWriter does not implement Flusher")
	}

	// Set streaming headers
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	return &Renderer{
		flusher: flusher,
		writer:  w,
	}, nil
}

// WriteAndFlush writes content and flushes immediately.
func (r *Renderer) WriteAndFlush(content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.writer.Write([]byte(content)); err != nil {
		return err
	}

	r.flusher.Flush()
	return nil
}

// Write writes content without flushing.
func (r *Renderer) Write(content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.writer.Write([]byte(content))
	return err
}

// Flush flushes the buffer.
func (r *Renderer) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flusher.Flush()
}

// StreamComponent renders a component with suspense support.
func (r *Renderer) StreamComponent(ctx context.Context, shell string, suspensePoints []SuspensePoint) error {
	// Send initial shell immediately
	if err := r.WriteAndFlush(shell); err != nil {
		return err
	}

	// Process each suspense point in parallel
	var wg sync.WaitGroup
	results := make(chan suspenseResult, len(suspensePoints))

	for _, sp := range suspensePoints {
		wg.Add(1)
		go func(point SuspensePoint) {
			defer wg.Done()

			content, err := point.Resolve(ctx)
			results <- suspenseResult{
				id:      point.ID,
				content: content,
				err:     err,
			}
		}(sp)
	}

	// Close results when all done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Send results as they complete
	for result := range results {
		if result.err != nil {
			// Send error fallback
			r.streamError(result.id, result.err)
			continue
		}

		// Send replacement script
		script := r.generateReplacementScript(result.id, result.content)
		if err := r.WriteAndFlush(script); err != nil {
			return err
		}
	}

	return nil
}

type suspenseResult struct {
	id      string
	content string
	err     error
}

// streamError sends an error replacement for a suspense point.
func (r *Renderer) streamError(id string, err error) {
	errorHTML := fmt.Sprintf(`<div class="suspense-error">Error loading content: %s</div>`, err.Error())
	script := r.generateReplacementScript(id, errorHTML)
	r.WriteAndFlush(script)
}

// generateReplacementScript creates a script that replaces a suspense placeholder.
func (r *Renderer) generateReplacementScript(id, content string) string {
	// Escape content for JavaScript string
	escaped := escapeForJS(content)

	return fmt.Sprintf(`<script>
(function() {
    var el = document.getElementById('suspense-%s');
    if (el) {
        var template = document.createElement('template');
        template.innerHTML = %s;
        el.replaceWith(template.content);
    }
})();
</script>
`, id, escaped)
}

// escapeForJS escapes content for use in a JavaScript string.
// This properly escapes all dangerous characters to prevent XSS attacks.
func escapeForJS(s string) string {
	var result bytes.Buffer
	result.WriteByte('"')

	for _, r := range s {
		switch r {
		case '\\':
			result.WriteString("\\\\")
		case '"':
			result.WriteString("\\\"")
		case '\'':
			result.WriteString("\\'")
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		case '<':
			// Escape to prevent </script> injection
			result.WriteString("\\u003c")
		case '>':
			result.WriteString("\\u003e")
		case '&':
			// Escape for HTML entity prevention
			result.WriteString("\\u0026")
		case '\u2028':
			// Line separator
			result.WriteString("\\u2028")
		case '\u2029':
			// Paragraph separator
			result.WriteString("\\u2029")
		default:
			result.WriteRune(r)
		}
	}

	result.WriteByte('"')
	return result.String()
}

// SuspensePoint represents a deferred rendering point.
type SuspensePoint struct {
	// ID is the unique identifier for this suspense point
	ID string

	// Fallback is the HTML to show while loading
	Fallback string

	// Resolve returns the final content
	Resolve func(ctx context.Context) (string, error)

	// Timeout is the maximum time to wait
	Timeout int // milliseconds
}

// SuspenseContext tracks suspense points during rendering.
type SuspenseContext struct {
	points []SuspensePoint
	mu     sync.Mutex
}

// NewSuspenseContext creates a new suspense context.
func NewSuspenseContext() *SuspenseContext {
	return &SuspenseContext{
		points: make([]SuspensePoint, 0),
	}
}

// Register adds a suspense point.
func (sc *SuspenseContext) Register(point SuspensePoint) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.points = append(sc.points, point)
}

// Points returns all registered suspense points.
func (sc *SuspenseContext) Points() []SuspensePoint {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	result := make([]SuspensePoint, len(sc.points))
	copy(result, sc.points)
	return result
}

// Clear removes all suspense points.
func (sc *SuspenseContext) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.points = sc.points[:0]
}

// Count returns the number of suspense points.
func (sc *SuspenseContext) Count() int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return len(sc.points)
}

// Context key for suspense context.
type suspenseContextKey struct{}

// WithSuspenseContext adds suspense context to a context.
func WithSuspenseContext(ctx context.Context, sc *SuspenseContext) context.Context {
	return context.WithValue(ctx, suspenseContextKey{}, sc)
}

// SuspenseContextFromContext retrieves suspense context.
func SuspenseContextFromContext(ctx context.Context) *SuspenseContext {
	sc, _ := ctx.Value(suspenseContextKey{}).(*SuspenseContext)
	return sc
}

// ShellExtractor extracts the shell and suspense points from rendered HTML.
type ShellExtractor struct {
	shell   bytes.Buffer
	points  []SuspensePoint
	inPoint bool
	current SuspensePoint
}

// NewShellExtractor creates a new shell extractor.
func NewShellExtractor() *ShellExtractor {
	return &ShellExtractor{
		points: make([]SuspensePoint, 0),
	}
}

// Extract processes rendered HTML and separates shell from suspense points.
func (se *ShellExtractor) Extract(html string) (string, []SuspensePoint) {
	// Simple implementation - look for suspense markers
	// In production, this would be more sophisticated
	return html, nil
}

// StreamingHandler wraps an HTTP handler to support streaming.
type StreamingHandler struct {
	handler func(ctx context.Context, w *Renderer) error
}

// NewStreamingHandler creates a new streaming handler.
func NewStreamingHandler(handler func(ctx context.Context, w *Renderer) error) *StreamingHandler {
	return &StreamingHandler{handler: handler}
}

// ServeHTTP implements http.Handler.
func (sh *StreamingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	renderer, err := NewRenderer(w)
	if err != nil {
		// Fall back to non-streaming
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	if err := sh.handler(r.Context(), renderer); err != nil {
		// Error already partially sent, log it
		fmt.Printf("streaming error: %v\n", err)
	}
}

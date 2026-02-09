package streaming

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"
)

// Suspense wraps content that loads asynchronously.
type Suspense struct {
	// ID is a unique identifier for this suspense boundary
	ID string

	// Fallback is shown while the content loads
	Fallback Renderable

	// Content is the async content to load
	Content func(ctx context.Context) (Renderable, error)

	// Timeout is the maximum time to wait before showing fallback permanently
	Timeout time.Duration

	// ErrorBoundary handles errors during content loading
	ErrorBoundary func(err error) Renderable
}

// Renderable is anything that can render to a writer.
type Renderable interface {
	Render(ctx context.Context, w io.Writer) error
}

// RenderFunc adapts a function to Renderable.
type RenderFunc func(ctx context.Context, w io.Writer) error

func (f RenderFunc) Render(ctx context.Context, w io.Writer) error {
	return f(ctx, w)
}

// Text creates a Renderable from a string.
func Text(s string) Renderable {
	return RenderFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(s))
		return err
	})
}

// Render renders the suspense boundary.
// In streaming mode, it renders the fallback and registers for later resolution.
// In non-streaming mode, it blocks until content is ready.
func (s *Suspense) Render(ctx context.Context, w io.Writer) error {
	// Check if we're in streaming mode
	sc := SuspenseContextFromContext(ctx)

	if sc != nil {
		// Streaming mode: render fallback and register suspense point
		return s.renderStreaming(ctx, w, sc)
	}

	// Non-streaming mode: block and render content directly
	return s.renderBlocking(ctx, w)
}

// renderStreaming renders the fallback and registers for later resolution.
func (s *Suspense) renderStreaming(ctx context.Context, w io.Writer, sc *SuspenseContext) error {
	// Write suspense placeholder with fallback
	fmt.Fprintf(w, `<div id="suspense-%s">`, s.ID)

	if s.Fallback != nil {
		if err := s.Fallback.Render(ctx, w); err != nil {
			return err
		}
	}

	fmt.Fprintf(w, `</div>`)

	// Register suspense point for later resolution
	sc.Register(SuspensePoint{
		ID:       s.ID,
		Fallback: "", // Already rendered
		Resolve: func(ctx context.Context) (string, error) {
			// Apply timeout if specified
			if s.Timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, s.Timeout)
				defer cancel()
			}

			// Load content
			content, err := s.Content(ctx)
			if err != nil {
				if s.ErrorBoundary != nil {
					content = s.ErrorBoundary(err)
				} else {
					return "", err
				}
			}

			// Render to string
			var buf bytes.Buffer
			if err := content.Render(ctx, &buf); err != nil {
				return "", err
			}

			return buf.String(), nil
		},
	})

	return nil
}

// renderBlocking blocks until content is ready and renders it.
func (s *Suspense) renderBlocking(ctx context.Context, w io.Writer) error {
	// Apply timeout if specified
	if s.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.Timeout)
		defer cancel()
	}

	// Load content
	content, err := s.Content(ctx)
	if err != nil {
		if s.ErrorBoundary != nil {
			return s.ErrorBoundary(err).Render(ctx, w)
		}
		if s.Fallback != nil {
			return s.Fallback.Render(ctx, w)
		}
		return err
	}

	return content.Render(ctx, w)
}

// SuspenseBuilder helps build suspense boundaries.
type SuspenseBuilder struct {
	suspense *Suspense
}

// NewSuspense creates a new suspense builder.
func NewSuspense(id string) *SuspenseBuilder {
	return &SuspenseBuilder{
		suspense: &Suspense{
			ID:      id,
			Timeout: 30 * time.Second, // Default 30s timeout
		},
	}
}

// WithFallback sets the fallback content.
func (b *SuspenseBuilder) WithFallback(fallback Renderable) *SuspenseBuilder {
	b.suspense.Fallback = fallback
	return b
}

// WithFallbackHTML sets fallback content from HTML string.
func (b *SuspenseBuilder) WithFallbackHTML(html string) *SuspenseBuilder {
	b.suspense.Fallback = Text(html)
	return b
}

// WithContent sets the async content loader.
func (b *SuspenseBuilder) WithContent(loader func(ctx context.Context) (Renderable, error)) *SuspenseBuilder {
	b.suspense.Content = loader
	return b
}

// WithTimeout sets the timeout duration.
func (b *SuspenseBuilder) WithTimeout(d time.Duration) *SuspenseBuilder {
	b.suspense.Timeout = d
	return b
}

// WithErrorBoundary sets the error handler.
func (b *SuspenseBuilder) WithErrorBoundary(handler func(err error) Renderable) *SuspenseBuilder {
	b.suspense.ErrorBoundary = handler
	return b
}

// Build returns the configured Suspense.
func (b *SuspenseBuilder) Build() *Suspense {
	return b.suspense
}

// Skeleton provides loading skeleton components.
type Skeleton struct {
	Width  string
	Height string
	Class  string
}

// Render renders the skeleton.
func (s Skeleton) Render(ctx context.Context, w io.Writer) error {
	style := ""
	if s.Width != "" {
		style += fmt.Sprintf("width:%s;", s.Width)
	}
	if s.Height != "" {
		style += fmt.Sprintf("height:%s;", s.Height)
	}

	class := "skeleton animate-pulse bg-gray-200 rounded"
	if s.Class != "" {
		class = s.Class
	}

	fmt.Fprintf(w, `<div class="%s" style="%s"></div>`, class, style)
	return nil
}

// SkeletonText creates a text skeleton.
func SkeletonText(lines int) Renderable {
	return RenderFunc(func(ctx context.Context, w io.Writer) error {
		for i := 0; i < lines; i++ {
			width := "75%"
			if i == lines-1 {
				width = "50%"
			}
			Skeleton{Width: width, Height: "1em", Class: "skeleton-line"}.Render(ctx, w)
			if i < lines-1 {
				w.Write([]byte("<br>"))
			}
		}
		return nil
	})
}

// SkeletonCard creates a card skeleton.
func SkeletonCard() Renderable {
	return RenderFunc(func(ctx context.Context, w io.Writer) error {
		fmt.Fprint(w, `<div class="skeleton-card p-4 border rounded">`)
		Skeleton{Width: "100%", Height: "150px"}.Render(ctx, w)
		fmt.Fprint(w, `<div class="mt-2">`)
		Skeleton{Width: "60%", Height: "1.5em"}.Render(ctx, w)
		fmt.Fprint(w, `</div><div class="mt-1">`)
		SkeletonText(2).Render(ctx, w)
		fmt.Fprint(w, `</div></div>`)
		return nil
	})
}

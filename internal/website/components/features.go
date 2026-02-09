package components

import (
	"fmt"
	"html"
	"strings"

	"github.com/gabrielmiguelok/golivekit/internal/website"
)

// FeaturesOptions configures the features section.
type FeaturesOptions struct {
	// Title is the section title
	Title string
	// Subtitle is optional description
	Subtitle string
	// Features is the list of features to display
	Features []website.Feature
	// Columns is the number of columns (default: 3)
	Columns int
}

// RenderFeatures generates a feature grid section.
func RenderFeatures(opts FeaturesOptions) string {
	var sb strings.Builder

	columns := opts.Columns
	if columns == 0 {
		columns = 3
	}

	gridClass := "grid-3"
	switch columns {
	case 2:
		gridClass = "grid-2"
	case 4:
		gridClass = "grid-4"
	case 6:
		gridClass = "grid-6"
	}

	sb.WriteString(`<section class="section" aria-labelledby="features-title">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container">`)
	sb.WriteString("\n")

	// Title
	if opts.Title != "" {
		sb.WriteString(`<div class="text-center" style="margin-bottom:3rem">`)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<h2 id="features-title">%s</h2>`, html.EscapeString(opts.Title)))
		sb.WriteString("\n")
		if opts.Subtitle != "" {
			sb.WriteString(fmt.Sprintf(`<p style="margin-top:0.5rem">%s</p>`, html.EscapeString(opts.Subtitle)))
			sb.WriteString("\n")
		}
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	// Grid
	sb.WriteString(fmt.Sprintf(`<div class="grid %s gap-lg">`, gridClass))
	sb.WriteString("\n")

	for i, feature := range opts.Features {
		sb.WriteString(renderFeatureCard(feature, i))
	}

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</section>`)
	sb.WriteString("\n")

	return sb.String()
}

func renderFeatureCard(f website.Feature, index int) string {
	delay := fmt.Sprintf("delay-%d", (index%4)+1)

	return fmt.Sprintf(`<article class="feature-card animate-fade-in %s">
<div class="feature-icon" aria-hidden="true">%s</div>
<h3 class="feature-title">%s</h3>
<p class="feature-desc">%s</p>
</article>
`, delay, f.Icon, html.EscapeString(f.Title), html.EscapeString(f.Description))
}

// DefaultGoliveKitFeatures returns the standard features for GoliveKit landing.
func DefaultGoliveKitFeatures() []website.Feature {
	return []website.Feature{
		{
			Icon:        "⚡",
			Title:       "Instant Response",
			Description: "Optimistic UI with <10ms perceived latency. Click feedback is instant, server confirms async.",
		},
		{
			Icon:        "🚀",
			Title:       "Zero JavaScript",
			Description: "Write your entire application in Go. Server-side rendering with minimal DOM patches.",
		},
		{
			Icon:        "🏝️",
			Title:       "Islands Architecture",
			Description: "Selective hydration for optimal performance. Interactive components only where needed.",
		},
		{
			Icon:        "📝",
			Title:       "Type-Safe Forms",
			Description: "Ecto-inspired changesets with validation. Strong typing from form to database.",
		},
		{
			Icon:        "🔒",
			Title:       "Secure by Default",
			Description: "CSRF protection, input sanitization, rate limiting, and authentication built-in.",
		},
		{
			Icon:        "📊",
			Title:       "Observable",
			Description: "Structured logging, Prometheus metrics, and OpenTelemetry tracing integration.",
		},
	}
}

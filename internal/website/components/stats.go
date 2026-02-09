package components

import (
	"fmt"
	"html"
	"strings"

	"github.com/gabrielmiguelok/golivekit/internal/website"
)

// StatsOptions configures the stats section.
type StatsOptions struct {
	// Title is the section title
	Title string
	// Stats contains the statistics to display
	Stats website.StatsConfig
	// ShowPulse shows the live indicator
	ShowPulse bool
}

// RenderStats generates a statistics section.
func RenderStats(opts StatsOptions) string {
	var sb strings.Builder

	sb.WriteString(`<section class="section" aria-labelledby="stats-title">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container">`)
	sb.WriteString("\n")

	if opts.Title != "" {
		sb.WriteString(`<div class="text-center" style="margin-bottom:2rem">`)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<h2 id="stats-title">%s`, html.EscapeString(opts.Title)))
		if opts.ShowPulse {
			sb.WriteString(` <span class="animate-pulse" style="color:var(--color-success)" aria-label="Live">●</span>`)
		}
		sb.WriteString(`</h2>`)
		sb.WriteString("\n")
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`<div class="stats-grid" style="max-width:600px;margin:0 auto" role="list">`)
	sb.WriteString("\n")

	// Visitors
	sb.WriteString(`<article class="stat-card" role="listitem">`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`<div class="stat-value" data-slot="visitors">%d</div>`, opts.Stats.Visitors))
	sb.WriteString("\n")
	sb.WriteString(`<div class="stat-label">Active Visitors</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</article>`)
	sb.WriteString("\n")

	// Total clicks
	sb.WriteString(`<article class="stat-card" role="listitem">`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`<div class="stat-value" data-slot="clicks">%d</div>`, opts.Stats.TotalClicks))
	sb.WriteString("\n")
	sb.WriteString(`<div class="stat-label">Total Clicks</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</article>`)
	sb.WriteString("\n")

	// Uptime
	sb.WriteString(`<article class="stat-card" role="listitem">`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`<div class="stat-value" data-slot="uptime">%s</div>`, html.EscapeString(opts.Stats.Uptime)))
	sb.WriteString("\n")
	sb.WriteString(`<div class="stat-label">Your Session</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</article>`)
	sb.WriteString("\n")

	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</section>`)
	sb.WriteString("\n")

	return sb.String()
}

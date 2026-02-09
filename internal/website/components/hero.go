package components

import (
	"fmt"
	"html"
	"strings"
)

// HeroOptions configures the hero section.
type HeroOptions struct {
	// Badge is optional badge text shown above title
	Badge string
	// BadgeIcon is the icon for the badge
	BadgeIcon string
	// Title is the main headline (supports HTML for gradients)
	Title string
	// TitleHTML allows raw HTML in title (for gradients)
	TitleHTML string
	// Subtitle is the description below the title
	Subtitle string
	// PrimaryButton is the primary CTA
	PrimaryButton HeroButton
	// SecondaryButton is the secondary CTA
	SecondaryButton HeroButton
	// ShowCounter shows a live counter demo
	ShowCounter bool
	// CounterLabel is the label for the counter
	CounterLabel string
}

// HeroButton represents a hero section button.
type HeroButton struct {
	Text    string
	URL     string
	Icon    string
	Primary bool
}

// RenderHero generates the hero section with title, subtitle, and CTAs.
func RenderHero(opts HeroOptions) string {
	var sb strings.Builder

	sb.WriteString(`<section class="hero section-lg" aria-labelledby="hero-title">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container">`)
	sb.WriteString("\n")

	// Badge
	if opts.Badge != "" {
		sb.WriteString(`<div class="flex justify-center" style="margin-bottom:1.5rem">`)
		sb.WriteString("\n")
		sb.WriteString(`<span class="badge animate-fade-in">`)
		if opts.BadgeIcon != "" {
			sb.WriteString(fmt.Sprintf(`<span aria-hidden="true">%s</span> `, opts.BadgeIcon))
		}
		sb.WriteString(html.EscapeString(opts.Badge))
		sb.WriteString(`</span>`)
		sb.WriteString("\n")
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	// Title
	sb.WriteString(`<h1 id="hero-title" class="hero-title animate-fade-in delay-1">`)
	if opts.TitleHTML != "" {
		sb.WriteString(opts.TitleHTML)
	} else {
		sb.WriteString(html.EscapeString(opts.Title))
	}
	sb.WriteString(`</h1>`)
	sb.WriteString("\n")

	// Subtitle
	if opts.Subtitle != "" {
		sb.WriteString(`<p class="hero-subtitle animate-fade-in delay-2">`)
		sb.WriteString(html.EscapeString(opts.Subtitle))
		sb.WriteString(`</p>`)
		sb.WriteString("\n")
	}

	// Counter demo
	if opts.ShowCounter {
		sb.WriteString(renderHeroCounter(opts.CounterLabel))
	}

	// Buttons
	if opts.PrimaryButton.Text != "" || opts.SecondaryButton.Text != "" {
		sb.WriteString(`<div class="hero-actions animate-fade-in delay-3">`)
		sb.WriteString("\n")

		if opts.PrimaryButton.Text != "" {
			target := ""
			if strings.HasPrefix(opts.PrimaryButton.URL, "http") {
				target = ` target="_blank" rel="noopener noreferrer"`
			}
			sb.WriteString(fmt.Sprintf(`<a href="%s" class="btn btn-primary btn-lg"%s>`,
				html.EscapeString(opts.PrimaryButton.URL), target))
			sb.WriteString(html.EscapeString(opts.PrimaryButton.Text))
			if opts.PrimaryButton.Icon != "" {
				sb.WriteString(fmt.Sprintf(` <span aria-hidden="true">%s</span>`, opts.PrimaryButton.Icon))
			}
			sb.WriteString(`</a>`)
			sb.WriteString("\n")
		}

		if opts.SecondaryButton.Text != "" {
			target := ""
			if strings.HasPrefix(opts.SecondaryButton.URL, "http") {
				target = ` target="_blank" rel="noopener noreferrer"`
			}
			sb.WriteString(fmt.Sprintf(`<a href="%s" class="btn btn-secondary btn-lg"%s>`,
				html.EscapeString(opts.SecondaryButton.URL), target))
			if opts.SecondaryButton.Icon != "" {
				sb.WriteString(fmt.Sprintf(`<span aria-hidden="true">%s</span> `, opts.SecondaryButton.Icon))
			}
			sb.WriteString(html.EscapeString(opts.SecondaryButton.Text))
			sb.WriteString(`</a>`)
			sb.WriteString("\n")
		}

		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</section>`)
	sb.WriteString("\n")

	return sb.String()
}

func renderHeroCounter(label string) string {
	if label == "" {
		label = "Live counter - click to test"
	}

	return fmt.Sprintf(`<div class="animate-fade-in delay-2" style="margin-bottom:2rem">
<div class="card" style="max-width:400px;margin:0 auto">
<div class="card-body">
<div class="counter-display" role="group" aria-label="%s">
<button class="counter-btn counter-btn-dec" lv-click="decrement" aria-label="Decrement counter">−</button>
<output class="counter-value" data-slot="count" aria-live="polite">0</output>
<button class="counter-btn counter-btn-inc" lv-click="increment" aria-label="Increment counter">+</button>
</div>
<p class="text-center" style="font-size:0.875rem;color:var(--color-textMuted);margin-top:0.5rem">
<span class="animate-pulse" style="color:var(--color-success)" aria-hidden="true">●</span> %s
</p>
</div>
</div>
</div>
`, html.EscapeString(label), html.EscapeString(label))
}

// RenderHeroWithCounter generates a hero with an embedded live counter.
func RenderHeroWithCounter(opts HeroOptions, count int) string {
	// Replace the counter display with actual value
	heroHTML := RenderHero(opts)
	return strings.Replace(heroHTML, `data-slot="count">0</output>`,
		fmt.Sprintf(`data-slot="count">%d</output>`, count), 1)
}

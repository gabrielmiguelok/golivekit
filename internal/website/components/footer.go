package components

import (
	"fmt"
	"html"
	"strings"

	"github.com/gabrielmiguelok/golivekit/internal/website"
)

// FooterOptions configures the footer component.
type FooterOptions struct {
	// Config is the footer configuration
	Config website.FooterConfig
	// ShowLogo shows the logo in footer
	ShowLogo bool
	// LogoText is the logo text
	LogoText string
	// Tagline is shown below the logo
	Tagline string
}

// RenderFooter generates the page footer.
func RenderFooter(opts FooterOptions) string {
	var sb strings.Builder

	sb.WriteString(`<footer class="section" style="border-top:1px solid var(--color-border)" role="contentinfo">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container">`)
	sb.WriteString("\n")

	sb.WriteString(`<div class="flex flex-col items-center gap-lg text-center">`)
	sb.WriteString("\n")

	// Logo and tagline
	if opts.ShowLogo && opts.LogoText != "" {
		sb.WriteString(`<div>`)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<div class="logo" style="font-size:1.25rem;margin-bottom:0.5rem">⚡ %s</div>`,
			html.EscapeString(opts.LogoText)))
		sb.WriteString("\n")
		if opts.Tagline != "" {
			sb.WriteString(fmt.Sprintf(`<p style="color:var(--color-textMuted)">%s</p>`,
				html.EscapeString(opts.Tagline)))
			sb.WriteString("\n")
		}
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	// Links
	sb.WriteString(`<nav class="flex gap-lg flex-wrap justify-center" aria-label="Footer navigation">`)
	sb.WriteString("\n")

	if opts.Config.GitHubURL != "" {
		sb.WriteString(fmt.Sprintf(`<a href="%s" class="btn btn-ghost" target="_blank" rel="noopener noreferrer">GitHub</a>`,
			html.EscapeString(opts.Config.GitHubURL)))
		sb.WriteString("\n")
	}

	if opts.Config.DocsURL != "" {
		sb.WriteString(fmt.Sprintf(`<a href="%s" class="btn btn-ghost" target="_blank" rel="noopener noreferrer">Documentation</a>`,
			html.EscapeString(opts.Config.DocsURL)))
		sb.WriteString("\n")
	}

	for _, link := range opts.Config.Links {
		target := ""
		rel := ""
		if link.External {
			target = ` target="_blank"`
			rel = ` rel="noopener noreferrer"`
		}
		sb.WriteString(fmt.Sprintf(`<a href="%s" class="btn btn-ghost"%s%s>%s</a>`,
			html.EscapeString(link.URL),
			target,
			rel,
			html.EscapeString(link.Label)))
		sb.WriteString("\n")
	}

	sb.WriteString(`</nav>`)
	sb.WriteString("\n")

	// License and copyright (using textMuted for WCAG 4.5:1 contrast)
	sb.WriteString(`<div style="color:var(--color-textMuted);font-size:0.875rem">`)
	sb.WriteString("\n")

	if opts.Config.License != "" {
		sb.WriteString(fmt.Sprintf(`<span>%s License</span>`, html.EscapeString(opts.Config.License)))
	}

	if opts.Config.Copyright != "" {
		if opts.Config.License != "" {
			sb.WriteString(` • `)
		}
		sb.WriteString(html.EscapeString(opts.Config.Copyright))
	}

	sb.WriteString(` • Made with ❤️ in Go`)
	sb.WriteString("\n")
	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</footer>`)
	sb.WriteString("\n")

	return sb.String()
}

// DefaultGoliveKitFooter returns the standard footer config for GoliveKit.
func DefaultGoliveKitFooter() FooterOptions {
	return FooterOptions{
		ShowLogo: true,
		LogoText: "GoliveKit",
		Tagline:  "Phoenix LiveView for Go",
		Config: website.FooterConfig{
			GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
			License:   "MIT",
		},
	}
}

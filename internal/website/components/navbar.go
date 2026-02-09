// Package components provides reusable UI components for landing pages.
package components

import (
	"fmt"
	"html"
	"strings"

	"github.com/gabrielmiguelok/golivekit/internal/website"
)

// NavbarOptions configures the navbar component.
type NavbarOptions struct {
	// Logo is the logo text (usually the product name)
	Logo string
	// LogoIcon is an optional icon before the logo
	LogoIcon string
	// Links are the navigation links
	Links []website.NavLink
	// GitHubURL is the GitHub repository URL (shown as button)
	GitHubURL string
	// ShowBadge shows a "100% Go" badge
	ShowBadge bool
	// BadgeText is the badge text
	BadgeText string
}

// RenderNavbar generates a sticky navigation bar.
func RenderNavbar(opts NavbarOptions) string {
	var sb strings.Builder

	sb.WriteString(`<a href="#main-content" class="skip-link">Skip to main content</a>`)
	sb.WriteString("\n")

	sb.WriteString(`<nav class="nav" role="navigation" aria-label="Main navigation">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container nav-inner">`)
	sb.WriteString("\n")

	// Logo (always visible)
	sb.WriteString(`<a href="/" class="logo" aria-label="Home">`)
	if opts.LogoIcon != "" {
		sb.WriteString(opts.LogoIcon)
		sb.WriteString(" ")
	}
	sb.WriteString(html.EscapeString(opts.Logo))
	sb.WriteString(`</a>`)
	sb.WriteString("\n")

	// Right side container
	sb.WriteString(`<div class="flex items-center gap-sm">`)
	sb.WriteString("\n")

	// Nav links group (hidden on mobile, shown on 640px+)
	sb.WriteString(`<div class="nav-links">`)
	sb.WriteString("\n")

	// Navigation links
	for _, link := range opts.Links {
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

	// Badge
	if opts.ShowBadge && opts.BadgeText != "" {
		sb.WriteString(fmt.Sprintf(`<span class="badge" aria-label="%s">`, html.EscapeString(opts.BadgeText)))
		sb.WriteString(`<span aria-hidden="true">🔥</span> `)
		sb.WriteString(html.EscapeString(opts.BadgeText))
		sb.WriteString(`</span>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	// GitHub button (always visible - icon only on mobile, icon+text on desktop)
	if opts.GitHubURL != "" {
		sb.WriteString(fmt.Sprintf(`<a href="%s" class="btn btn-secondary btn-sm" target="_blank" rel="noopener noreferrer" aria-label="View on GitHub">`,
			html.EscapeString(opts.GitHubURL)))
		sb.WriteString(`<svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>`)
		sb.WriteString(`<span class="hide-mobile"> GitHub</span>`)
		sb.WriteString(`</a>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</nav>`)
	sb.WriteString("\n")

	return sb.String()
}

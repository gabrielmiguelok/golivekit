// Package website provides reusable page templates and UI components for building
// premium landing pages and documentation sites. It follows GoliveKit's philosophy
// of 100% Go, no external CSS frameworks, and instant performance.
//
// Note: This package is internal and contains the GoliveKit website templates.
// It is not intended for external use. For building your own landing pages,
// use the component patterns as reference.
package website

// PageConfig defines the configuration for a landing page including SEO metadata.
type PageConfig struct {
	// Title is the page title (shown in browser tab and search results)
	Title string
	// Description is the meta description for SEO
	Description string
	// URL is the canonical URL of the page
	URL string
	// Keywords are SEO keywords for the page
	Keywords []string
	// Author is the author meta tag
	Author string
	// OGImage is the Open Graph image URL (for social sharing)
	OGImage string
	// Language is the page language (default: "en")
	Language string
	// ThemeColor is the mobile browser theme color
	ThemeColor string
	// Favicon is the path to the favicon
	Favicon string
}

// Feature represents a feature card in the features section.
type Feature struct {
	// Icon is a Unicode emoji or symbol
	Icon string
	// Title is the feature title
	Title string
	// Description explains the feature
	Description string
}

// Package represents a package in the package grid.
type Package struct {
	// Name is the package name (e.g., "core", "router")
	Name string
	// Description briefly explains what the package does
	Description string
	// Category groups packages (e.g., "Core", "UI", "State")
	Category string
	// Color is the hex color for the category badge
	Color string
}

// CLICommand represents a CLI command with its description.
type CLICommand struct {
	// Command is the CLI command (e.g., "golive new myapp")
	Command string
	// Description explains what the command does
	Description string
}

// Step represents a getting started step.
type Step struct {
	// Number is the step number (1, 2, 3...)
	Number int
	// Title is the step title
	Title string
	// Command is the shell command to run
	Command string
	// Description provides additional context
	Description string
}

// CodeExample represents a code example with syntax highlighting.
type CodeExample struct {
	// Language is the programming language (e.g., "go", "bash")
	Language string
	// Code is the source code
	Code string
	// Title is an optional title for the code block
	Title string
}

// NavLink represents a navigation link.
type NavLink struct {
	// Label is the link text
	Label string
	// URL is the link destination
	URL string
	// External indicates if the link opens in a new tab
	External bool
}

// FooterConfig configures the footer section.
type FooterConfig struct {
	// GitHubURL is the GitHub repository URL
	GitHubURL string
	// DocsURL is the documentation URL
	DocsURL string
	// License is the license name (e.g., "MIT")
	License string
	// Copyright is the copyright text
	Copyright string
	// Links are additional footer links
	Links []NavLink
}

// ArchitectureDiagram configures the architecture diagram.
type ArchitectureDiagram struct {
	// Title is the diagram title
	Title string
	// ASCII is the ASCII art diagram
	ASCII string
}

// SocialLinks configures social media links.
type SocialLinks struct {
	GitHub   string
	Twitter  string
	Discord  string
	LinkedIn string
}

// BadgeConfig configures a badge/pill element.
type BadgeConfig struct {
	// Text is the badge text
	Text string
	// Color is the badge background color
	Color string
	// TextColor is the badge text color (default: white)
	TextColor string
	// Icon is an optional icon before the text
	Icon string
}

// CounterConfig configures the live counter demo.
type CounterConfig struct {
	// InitialValue is the starting value
	InitialValue int
	// ShowLatency shows the latency indicator
	ShowLatency bool
	// Label is the counter label
	Label string
}

// StatsConfig configures live statistics display.
type StatsConfig struct {
	// Visitors shows active visitors count
	Visitors int64
	// TotalClicks shows total click count
	TotalClicks int64
	// Uptime shows session duration
	Uptime string
}

// DefaultPageConfig returns a PageConfig with sensible defaults.
func DefaultPageConfig() PageConfig {
	return PageConfig{
		Language:   "en",
		ThemeColor: "#8B5CF6",
	}
}

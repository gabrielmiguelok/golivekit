// Package landing provides complete landing page templates.
package landing

import (
	"strings"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
)

// Options configures the framework landing page.
type Options struct {
	// Features to display in the features section
	Features []website.Feature
	// Packages to display in the package grid
	Packages []website.Package
	// CLICommands for the CLI section
	CLICommands []website.CLICommand
	// Steps for the getting started section
	Steps []website.Step
	// CodeExample for the code section
	CodeExample website.CodeExample

	// GitHubURL is the GitHub repository URL
	GitHubURL string
	// DocsURL is the documentation URL
	DocsURL string

	// ShowCounter shows the live counter in hero
	ShowCounter bool
	// CounterValue is the current counter value (for SSR)
	CounterValue int
	// CounterLabel is the label for the counter
	CounterLabel string

	// Stats for the live stats section
	Stats website.StatsConfig
	// ShowStats shows the stats section
	ShowStats bool

	// ShowArchitecture shows the architecture diagram
	ShowArchitecture bool

	// Badge text shown in navbar
	NavBadge string

	// CustomCSS is additional CSS to include
	CustomCSS string

	// Footer config
	FooterTagline string
}

// DefaultOptions returns options with sensible defaults for GoliveKit.
func DefaultOptions() Options {
	return Options{
		Features:         components.DefaultGoliveKitFeatures(),
		Packages:         components.DefaultGoliveKitPackages(),
		CLICommands:      components.DefaultGoliveKitCLI(),
		Steps:            components.DefaultGoliveKitSteps(),
		CodeExample:      components.DefaultGoliveKitCode(),
		GitHubURL:        "https://github.com/gabrielmiguelok/golivekit",
		ShowCounter:      true,
		CounterLabel:     "Live counter - click to test real-time updates",
		ShowStats:        true,
		ShowArchitecture: true,
		NavBadge:         "100% Go",
		FooterTagline:    "Phoenix LiveView for Go",
	}
}

// RenderFrameworkLanding generates a complete framework landing page.
func RenderFrameworkLanding(cfg website.PageConfig, opts Options) string {
	var body strings.Builder

	// Skip link + Navbar
	body.WriteString(components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: opts.GitHubURL,
		ShowBadge: opts.NavBadge != "",
		BadgeText: opts.NavBadge,
		Links: []website.NavLink{
			{Label: "Docs", URL: opts.DocsURL, External: true},
		},
	}))

	// Main content
	body.WriteString(`<main id="main-content">`)
	body.WriteString("\n")

	// Hero section
	heroOpts := components.HeroOptions{
		Badge:     "Instant like React, Server-side like Go",
		BadgeIcon: "⚡",
		TitleHTML: `<span class="text-gradient">GoliveKit</span>`,
		Subtitle:  "Build interactive real-time web apps in pure Go. &lt;10ms response on every click, zero JavaScript required.",
		PrimaryButton: components.HeroButton{
			Text: "Get Started",
			URL:  "#getting-started",
			Icon: "→",
		},
		SecondaryButton: components.HeroButton{
			Text: "GitHub",
			URL:  opts.GitHubURL,
			Icon: "⭐",
		},
		ShowCounter:  opts.ShowCounter,
		CounterLabel: opts.CounterLabel,
	}

	if opts.ShowCounter && opts.CounterValue != 0 {
		body.WriteString(components.RenderHeroWithCounter(heroOpts, opts.CounterValue))
	} else {
		body.WriteString(components.RenderHero(heroOpts))
	}

	// Stats section
	if opts.ShowStats {
		body.WriteString(components.RenderStats(components.StatsOptions{
			Title:     "Live Stats",
			Stats:     opts.Stats,
			ShowPulse: true,
		}))
	}

	// Features section
	if len(opts.Features) > 0 {
		body.WriteString(components.RenderFeatures(components.FeaturesOptions{
			Title:    "Why GoliveKit?",
			Subtitle: "Everything you need to build modern web applications",
			Features: opts.Features,
			Columns:  3,
		}))
	}

	// Why It's Fast section (performance diagram)
	whyFast := components.WhyItsFastArchitecture()
	body.WriteString(components.RenderArchitecture(whyFast))

	// Packages section
	if len(opts.Packages) > 0 {
		body.WriteString(components.RenderPackages(components.PackagesOptions{
			Title:          "29 Packages",
			Subtitle:       "Modular architecture for maximum flexibility",
			Packages:       opts.Packages,
			ShowCategories: true,
		}))
	}

	// Code section
	if opts.CodeExample.Code != "" {
		body.WriteString(components.RenderCodeSection(
			"Simple & Powerful",
			"Write your components in pure Go with a familiar lifecycle",
			opts.CodeExample,
		))
	}

	// Architecture section
	if opts.ShowArchitecture {
		arch := components.DefaultGoliveKitArchitecture()
		body.WriteString(components.RenderArchitecture(arch))
	}

	// CLI section
	if len(opts.CLICommands) > 0 {
		body.WriteString(components.RenderCLI(components.CLIOptions{
			Title:    "Developer Experience",
			Subtitle: "Powerful CLI for rapid development",
			Commands: opts.CLICommands,
		}))
	}

	// Getting started section
	if len(opts.Steps) > 0 {
		body.WriteString(`<div id="getting-started">`)
		body.WriteString("\n")
		body.WriteString(components.RenderGettingStarted(components.GettingStartedOptions{
			Title:    "Get Started in 3 Steps",
			Subtitle: "From zero to real-time in under a minute",
			Steps:    opts.Steps,
		}))
		body.WriteString(`</div>`)
		body.WriteString("\n")
	}

	body.WriteString(`</main>`)
	body.WriteString("\n")

	// Footer
	footerOpts := components.DefaultGoliveKitFooter()
	footerOpts.Config.GitHubURL = opts.GitHubURL
	footerOpts.Config.DocsURL = opts.DocsURL
	if opts.FooterTagline != "" {
		footerOpts.Tagline = opts.FooterTagline
	}
	body.WriteString(components.RenderFooter(footerOpts))

	// GoliveKit client script
	body.WriteString(`<script src="/_live/golivekit.js"></script>`)
	body.WriteString("\n")

	// Wrap in document
	return website.RenderDocument(cfg, opts.CustomCSS, body.String())
}

// RenderMinimalLanding generates a minimal landing page with just hero and features.
func RenderMinimalLanding(cfg website.PageConfig, opts Options) string {
	var body strings.Builder

	body.WriteString(`<main>`)
	body.WriteString("\n")

	// Hero
	body.WriteString(components.RenderHero(components.HeroOptions{
		TitleHTML: `<span class="text-gradient">` + cfg.Title + `</span>`,
		Subtitle:  cfg.Description,
		PrimaryButton: components.HeroButton{
			Text: "Get Started",
			URL:  opts.GitHubURL,
			Icon: "→",
		},
	}))

	// Features
	if len(opts.Features) > 0 {
		body.WriteString(components.RenderFeatures(components.FeaturesOptions{
			Title:    "Features",
			Features: opts.Features,
			Columns:  3,
		}))
	}

	body.WriteString(`</main>`)
	body.WriteString("\n")

	return website.RenderDocument(cfg, opts.CustomCSS, body.String())
}

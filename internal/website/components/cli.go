package components

import (
	"fmt"
	"html"
	"strings"

	"github.com/gabrielmiguelok/golivekit/internal/website"
)

// CLIOptions configures the CLI section.
type CLIOptions struct {
	// Title is the section title
	Title string
	// Subtitle is optional description
	Subtitle string
	// Commands is the list of CLI commands
	Commands []website.CLICommand
}

// RenderCLI generates a CLI commands section.
func RenderCLI(opts CLIOptions) string {
	var sb strings.Builder

	sb.WriteString(`<section class="section" aria-labelledby="cli-title">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container">`)
	sb.WriteString("\n")

	if opts.Title != "" {
		sb.WriteString(`<div class="text-center" style="margin-bottom:2rem">`)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<h2 id="cli-title">%s</h2>`, html.EscapeString(opts.Title)))
		sb.WriteString("\n")
		if opts.Subtitle != "" {
			sb.WriteString(fmt.Sprintf(`<p style="margin-top:0.5rem">%s</p>`, html.EscapeString(opts.Subtitle)))
			sb.WriteString("\n")
		}
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`<div class="code-block" style="max-width:700px;margin:0 auto">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="code-header">`)
	sb.WriteString(`<div class="code-dots" aria-hidden="true">`)
	sb.WriteString(`<span class="code-dot code-dot-red"></span>`)
	sb.WriteString(`<span class="code-dot code-dot-yellow"></span>`)
	sb.WriteString(`<span class="code-dot code-dot-green"></span>`)
	sb.WriteString(`</div>`)
	sb.WriteString(`<span class="code-title">Terminal</span>`)
	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	sb.WriteString(`<div class="code-content">`)
	sb.WriteString("\n")

	for _, cmd := range opts.Commands {
		sb.WriteString(`<div style="margin-bottom:1rem">`)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<div style="color:var(--color-success)">$ %s</div>`, html.EscapeString(cmd.Command)))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<div style="color:var(--color-textMuted);font-size:0.8rem;margin-top:0.25rem">→ %s</div>`,
			html.EscapeString(cmd.Description)))
		sb.WriteString("\n")
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</section>`)
	sb.WriteString("\n")

	return sb.String()
}

// DefaultGoliveKitCLI returns the standard CLI commands for GoliveKit.
func DefaultGoliveKitCLI() []website.CLICommand {
	return []website.CLICommand{
		{
			Command:     "golive new myapp",
			Description: "Creates project structure with routing, components, and templates",
		},
		{
			Command:     "golive dev",
			Description: "Starts dev server with hot reload and file watching",
		},
		{
			Command:     "golive build",
			Description: "Builds optimized production binary to dist/",
		},
		{
			Command:     "golive generate live Counter",
			Description: "Generates LiveView component with boilerplate",
		},
	}
}

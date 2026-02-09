package components

import (
	"fmt"
	"html"
	"strings"

	"github.com/gabrielmiguelok/golivekit/internal/website"
)

// GettingStartedOptions configures the getting started section.
type GettingStartedOptions struct {
	// Title is the section title
	Title string
	// Subtitle is optional description
	Subtitle string
	// Steps is the list of steps
	Steps []website.Step
}

// RenderGettingStarted generates a getting started section.
func RenderGettingStarted(opts GettingStartedOptions) string {
	var sb strings.Builder

	sb.WriteString(`<section class="section" aria-labelledby="start-title">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container">`)
	sb.WriteString("\n")

	if opts.Title != "" {
		sb.WriteString(`<div class="text-center" style="margin-bottom:3rem">`)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<h2 id="start-title">%s</h2>`, html.EscapeString(opts.Title)))
		sb.WriteString("\n")
		if opts.Subtitle != "" {
			sb.WriteString(fmt.Sprintf(`<p style="margin-top:0.5rem">%s</p>`, html.EscapeString(opts.Subtitle)))
			sb.WriteString("\n")
		}
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`<div class="grid grid-3 gap-xl" style="max-width:1000px;margin:0 auto">`)
	sb.WriteString("\n")

	for _, step := range opts.Steps {
		sb.WriteString(renderStep(step))
	}

	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</section>`)
	sb.WriteString("\n")

	return sb.String()
}

func renderStep(step website.Step) string {
	var sb strings.Builder

	sb.WriteString(`<div class="card">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="card-body">`)
	sb.WriteString("\n")

	// Step number
	sb.WriteString(fmt.Sprintf(`<div style="width:3rem;height:3rem;border-radius:50%%;background:var(--color-primary);color:white;display:flex;align-items:center;justify-content:center;font-size:1.25rem;font-weight:bold;margin-bottom:1rem">%d</div>`,
		step.Number))
	sb.WriteString("\n")

	// Title
	sb.WriteString(fmt.Sprintf(`<h3 style="margin-bottom:0.75rem">%s</h3>`, html.EscapeString(step.Title)))
	sb.WriteString("\n")

	// Command
	if step.Command != "" {
		sb.WriteString(`<div style="background:var(--color-bgCode);padding:0.75rem 1rem;border-radius:0.5rem;font-family:var(--font-mono);font-size:0.875rem;color:var(--color-success);margin-bottom:0.75rem;overflow-x:auto">`)
		sb.WriteString(html.EscapeString(step.Command))
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	// Description
	if step.Description != "" {
		sb.WriteString(fmt.Sprintf(`<p style="font-size:0.875rem">%s</p>`, html.EscapeString(step.Description)))
		sb.WriteString("\n")
	}

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	return sb.String()
}

// DefaultGoliveKitSteps returns the standard getting started steps.
func DefaultGoliveKitSteps() []website.Step {
	return []website.Step{
		{
			Number:      1,
			Title:       "Install",
			Command:     "go install github.com/gabrielmiguelok/golivekit/cmd/golive@latest",
			Description: "Install the GoliveKit CLI globally",
		},
		{
			Number:      2,
			Title:       "Create",
			Command:     "golive new myapp && cd myapp",
			Description: "Scaffold a new project with routing and components",
		},
		{
			Number:      3,
			Title:       "Run",
			Command:     "golive dev",
			Description: "Start the dev server at http://localhost:3000",
		},
	}
}

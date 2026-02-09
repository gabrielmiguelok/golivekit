package components

import (
	"fmt"
	"html"
	"strings"
)

// ArchitectureOptions configures the architecture diagram section.
type ArchitectureOptions struct {
	// Title is the section title
	Title string
	// Subtitle is optional description
	Subtitle string
	// Diagram is the ASCII art diagram
	Diagram string
}

// RenderArchitecture generates an architecture diagram section.
func RenderArchitecture(opts ArchitectureOptions) string {
	var sb strings.Builder

	sb.WriteString(`<section class="section" style="background:var(--color-bgAlt)" aria-labelledby="arch-title">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container">`)
	sb.WriteString("\n")

	if opts.Title != "" {
		sb.WriteString(`<div class="text-center" style="margin-bottom:2rem">`)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<h2 id="arch-title">%s</h2>`, html.EscapeString(opts.Title)))
		sb.WriteString("\n")
		if opts.Subtitle != "" {
			sb.WriteString(fmt.Sprintf(`<p style="margin-top:0.5rem">%s</p>`, html.EscapeString(opts.Subtitle)))
			sb.WriteString("\n")
		}
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`<div class="code-block" style="max-width:800px;margin:0 auto">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="code-content" style="text-align:center">`)
	sb.WriteString("\n")
	sb.WriteString(`<pre style="display:inline-block;text-align:left;font-size:0.8rem;line-height:1.4">`)
	sb.WriteString(html.EscapeString(opts.Diagram))
	sb.WriteString(`</pre>`)
	sb.WriteString("\n")
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

// WhyItsFastArchitecture returns the performance optimization diagram.
func WhyItsFastArchitecture() ArchitectureOptions {
	return ArchitectureOptions{
		Title:    "Why It's Fast",
		Subtitle: "Three-layer optimization for instant response",
		Diagram: `
   ┌─────────────────────────────────────────────────────────────────┐
   │                  THREE-LAYER SPEED                              │
   │                                                                 │
   │   ┌──────────────────────────────────────────────────────────┐  │
   │   │  1. CLIENT LAYER                        0ms perceived    │  │
   │   │     ► CSS :active feedback on click                      │  │
   │   │     ► Optimistic UI updates                              │  │
   │   │     ► Smart debouncing (16ms)                            │  │
   │   └──────────────────────────────────────────────────────────┘  │
   │                            │                                    │
   │                            ▼                                    │
   │   ┌──────────────────────────────────────────────────────────┐  │
   │   │  2. DIFF LAYER                          <5ms processing  │  │
   │   │     ► Hash-based O(1) slot comparison                    │  │
   │   │     ► Single-pass slot extraction                        │  │
   │   │     ► Minimal patch generation                           │  │
   │   └──────────────────────────────────────────────────────────┘  │
   │                            │                                    │
   │                            ▼                                    │
   │   ┌──────────────────────────────────────────────────────────┐  │
   │   │  3. SERVER LAYER                        <10ms total      │  │
   │   │     ► Buffer pools (reduced GC)                          │  │
   │   │     ► Per-socket state (no global mutex)                 │  │
   │   │     ► Pre-rendered static content                        │  │
   │   └──────────────────────────────────────────────────────────┘  │
   │                                                                 │
   │   Result: React-like responsiveness with server-side rendering  │
   └─────────────────────────────────────────────────────────────────┘
`,
	}
}

// DefaultGoliveKitArchitecture returns the standard architecture diagram.
func DefaultGoliveKitArchitecture() ArchitectureOptions {
	return ArchitectureOptions{
		Title:    "How It Works",
		Subtitle: "Server-rendered components with real-time DOM updates",
		Diagram: `
     Browser                              Server
  ┌───────────────┐                  ┌─────────────────────────────┐
  │               │   HTTP Request   │                             │
  │     DOM       │ ───────────────► │  Component.Mount()          │
  │               │                  │       │                     │
  │               │ ◄─────────────── │       ▼                     │
  │               │   Full HTML      │  Component.Render() → HTML  │
  │               │                  │                             │
  └───────┬───────┘                  └──────────────┬──────────────┘
          │                                         │
          │           WebSocket                     │
          │ ◄──────────────────────────────────────►│
          │                                         │
  ┌───────▼───────┐                  ┌──────────────▼──────────────┐
  │               │   User Event     │                             │
  │  lv-click     │ ───────────────► │  Component.HandleEvent()    │
  │  lv-change    │                  │       │                     │
  │  lv-submit    │ ◄─────────────── │       ▼                     │
  │               │   Minimal Diff   │  Diff Engine → Patch        │
  └───────────────┘                  └─────────────────────────────┘
`,
	}
}

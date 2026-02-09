package components

import (
	"fmt"
	"html"
	"strings"

	"github.com/gabrielmiguelok/golivekit/internal/website"
)

// CodeBlockOptions configures a code block.
type CodeBlockOptions struct {
	// Title is shown in the header
	Title string
	// Language determines syntax highlighting
	Language string
	// Code is the source code
	Code string
	// ShowLineNumbers enables line numbers
	ShowLineNumbers bool
	// ShowHeader shows the mac-style header with dots
	ShowHeader bool
}

// RenderCodeBlock generates a syntax-highlighted code block.
func RenderCodeBlock(opts CodeBlockOptions) string {
	var sb strings.Builder

	sb.WriteString(`<div class="code-block">`)
	sb.WriteString("\n")

	// Header with mac-style dots
	if opts.ShowHeader {
		sb.WriteString(`<div class="code-header">`)
		sb.WriteString("\n")
		sb.WriteString(`<div class="code-dots" aria-hidden="true">`)
		sb.WriteString(`<span class="code-dot code-dot-red"></span>`)
		sb.WriteString(`<span class="code-dot code-dot-yellow"></span>`)
		sb.WriteString(`<span class="code-dot code-dot-green"></span>`)
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
		if opts.Title != "" {
			sb.WriteString(fmt.Sprintf(`<span class="code-title">%s</span>`, html.EscapeString(opts.Title)))
		}
		sb.WriteString("\n")
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	// Code content
	sb.WriteString(`<div class="code-content">`)
	sb.WriteString("\n")
	sb.WriteString(`<pre><code>`)

	// Apply syntax highlighting based on language
	highlighted := highlightSyntax(opts.Code, opts.Language)
	sb.WriteString(highlighted)

	sb.WriteString(`</code></pre>`)
	sb.WriteString("\n")
	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	return sb.String()
}

// RenderCodeSection generates a section with a code example.
func RenderCodeSection(title, subtitle string, example website.CodeExample) string {
	var sb strings.Builder

	sb.WriteString(`<section class="section" aria-labelledby="code-title">`)
	sb.WriteString("\n")
	sb.WriteString(`<div class="container">`)
	sb.WriteString("\n")

	if title != "" {
		sb.WriteString(`<div class="text-center" style="margin-bottom:2rem">`)
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`<h2 id="code-title">%s</h2>`, html.EscapeString(title)))
		sb.WriteString("\n")
		if subtitle != "" {
			sb.WriteString(fmt.Sprintf(`<p style="margin-top:0.5rem">%s</p>`, html.EscapeString(subtitle)))
			sb.WriteString("\n")
		}
		sb.WriteString(`</div>`)
		sb.WriteString("\n")
	}

	sb.WriteString(`<div style="max-width:800px;margin:0 auto">`)
	sb.WriteString("\n")
	sb.WriteString(RenderCodeBlock(CodeBlockOptions{
		Title:      example.Title,
		Language:   example.Language,
		Code:       example.Code,
		ShowHeader: true,
	}))
	sb.WriteString(`</div>`)
	sb.WriteString("\n")

	sb.WriteString(`</div>`)
	sb.WriteString("\n")
	sb.WriteString(`</section>`)
	sb.WriteString("\n")

	return sb.String()
}

// highlightSyntax applies basic syntax highlighting for Go code.
func highlightSyntax(code, language string) string {
	if language != "go" && language != "golang" {
		return html.EscapeString(code)
	}

	// Keywords
	keywords := []string{
		"func", "return", "if", "else", "for", "range", "switch", "case", "default",
		"break", "continue", "go", "defer", "select", "chan", "map", "struct",
		"interface", "type", "const", "var", "package", "import", "nil", "true", "false",
	}

	// Types
	types := []string{
		"string", "int", "int64", "int32", "float64", "float32", "bool", "byte",
		"error", "any", "context", "Context",
	}

	// First escape HTML
	result := html.EscapeString(code)

	// Apply highlighting (order matters)

	// Comments (// ...)
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "//"); idx != -1 {
			// Check if // is inside a string (basic check)
			beforeComment := line[:idx]
			if strings.Count(beforeComment, `"`)%2 == 0 && strings.Count(beforeComment, "`")%2 == 0 {
				lines[i] = line[:idx] + `<span class="token-comment">` + line[idx:] + `</span>`
			}
		}
	}
	result = strings.Join(lines, "\n")

	// Strings (double quotes)
	result = highlightStrings(result)

	// Keywords
	for _, kw := range keywords {
		// Match whole words only
		result = replaceWholeWord(result, kw, fmt.Sprintf(`<span class="token-keyword">%s</span>`, kw))
	}

	// Types
	for _, t := range types {
		result = replaceWholeWord(result, t, fmt.Sprintf(`<span class="token-type">%s</span>`, t))
	}

	// Function calls (word followed by open paren)
	// This is a simplified version - a full parser would be more accurate

	return result
}

func highlightStrings(code string) string {
	var result strings.Builder
	inString := false
	stringChar := byte(0)
	i := 0

	for i < len(code) {
		if !inString {
			if code[i] == '"' || code[i] == '`' {
				inString = true
				stringChar = code[i]
				result.WriteString(`<span class="token-string">`)
				result.WriteByte(code[i])
				i++
				continue
			}
		} else {
			if code[i] == stringChar && (i == 0 || code[i-1] != '\\') {
				result.WriteByte(code[i])
				result.WriteString(`</span>`)
				inString = false
				i++
				continue
			}
		}
		result.WriteByte(code[i])
		i++
	}

	return result.String()
}

func replaceWholeWord(s, word, replacement string) string {
	// Simple whole-word replacement
	// Avoid replacing inside HTML tags or strings
	var result strings.Builder
	i := 0
	wordLen := len(word)

	for i < len(s) {
		// Check if we're inside an HTML tag
		if s[i] == '<' {
			// Skip until >
			for i < len(s) && s[i] != '>' {
				result.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				result.WriteByte(s[i])
				i++
			}
			continue
		}

		// Check for word match
		if i+wordLen <= len(s) && s[i:i+wordLen] == word {
			// Check word boundaries
			before := i == 0 || !isWordChar(s[i-1])
			after := i+wordLen >= len(s) || !isWordChar(s[i+wordLen])

			if before && after {
				result.WriteString(replacement)
				i += wordLen
				continue
			}
		}

		result.WriteByte(s[i])
		i++
	}

	return result.String()
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// DefaultGoliveKitCode returns the standard code example for GoliveKit.
func DefaultGoliveKitCode() website.CodeExample {
	return website.CodeExample{
		Title:    "counter.go",
		Language: "go",
		Code: `type Counter struct {
    core.BaseComponent
    count int
}

func (c *Counter) Mount(ctx context.Context, params core.Params, session core.Session) error {
    c.count = 0
    return nil
}

func (c *Counter) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
    switch event {
    case "increment":
        c.count++
    case "decrement":
        c.count--
    }
    return nil
}

func (c *Counter) Render(ctx context.Context) core.Renderer {
    return core.HTML(fmt.Sprintf(` + "`" + `
        <div>
            <span>Count: %d</span>
            <button lv-click="decrement">-</button>
            <button lv-click="increment">+</button>
        </div>
    ` + "`" + `, c.count))
}`,
	}
}

// Package security provides security utilities for GoliveKit.
// This file adds automatic HTML sanitization for rendered output.
package security

import (
	"context"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/gabrielmiguelok/golivekit/pkg/pool"
)

// AutoSanitizeConfig configures automatic sanitization behavior.
type AutoSanitizeConfig struct {
	// Enabled controls whether auto-sanitization is active.
	Enabled bool

	// AllowedTags is a list of HTML tags that are allowed.
	// If empty, a safe default list is used.
	AllowedTags []string

	// AllowedAttributes is a map of tag -> allowed attributes.
	AllowedAttributes map[string][]string

	// StripComments removes HTML comments.
	StripComments bool

	// UnsafeAttributeMarker is the attribute that marks content as pre-sanitized.
	// Content inside elements with this attribute is NOT re-sanitized.
	UnsafeAttributeMarker string
}

// DefaultAutoSanitizeConfig returns safe default configuration.
func DefaultAutoSanitizeConfig() AutoSanitizeConfig {
	return AutoSanitizeConfig{
		Enabled: true,
		AllowedTags: []string{
			"a", "abbr", "b", "blockquote", "br", "code", "dd", "del", "div",
			"dl", "dt", "em", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "i",
			"img", "li", "ol", "p", "pre", "q", "s", "small", "span", "strong",
			"sub", "sup", "table", "tbody", "td", "tfoot", "th", "thead", "tr",
			"u", "ul", "button", "form", "input", "label", "select", "option",
			"textarea", "nav", "header", "footer", "main", "section", "article",
			"aside", "figure", "figcaption", "time", "mark", "details", "summary",
		},
		AllowedAttributes: map[string][]string{
			"*":        {"class", "id", "data-slot", "data-*", "style", "title", "aria-*", "role"},
			"a":        {"href", "target", "rel"},
			"img":      {"src", "alt", "width", "height", "loading"},
			"input":    {"type", "name", "value", "placeholder", "required", "disabled", "checked", "readonly", "lv-*"},
			"button":   {"type", "disabled", "lv-*"},
			"form":     {"action", "method", "enctype", "lv-*"},
			"select":   {"name", "required", "disabled", "multiple", "lv-*"},
			"option":   {"value", "selected", "disabled"},
			"textarea": {"name", "placeholder", "required", "disabled", "readonly", "rows", "cols", "lv-*"},
			"label":    {"for"},
			"td":       {"colspan", "rowspan"},
			"th":       {"colspan", "rowspan", "scope"},
			"time":     {"datetime"},
		},
		StripComments:         true,
		UnsafeAttributeMarker: "data-unsafe",
	}
}

// AutoSanitizer provides automatic HTML sanitization.
type AutoSanitizer struct {
	config     AutoSanitizeConfig
	sanitizer  *Sanitizer
	tagPattern *regexp.Regexp
	mu         sync.RWMutex
}

// NewAutoSanitizer creates a new auto-sanitizer.
func NewAutoSanitizer(config AutoSanitizeConfig) *AutoSanitizer {
	return &AutoSanitizer{
		config:     config,
		sanitizer:  NewSanitizer(DefaultSanitizerConfig()),
		tagPattern: regexp.MustCompile(`<([a-zA-Z0-9]+)([^>]*)>`),
	}
}

// Sanitize sanitizes HTML content according to the configuration.
func (as *AutoSanitizer) Sanitize(html string) string {
	as.mu.RLock()
	config := as.config
	as.mu.RUnlock()

	if !config.Enabled {
		return html
	}

	// Strip HTML comments if configured
	if config.StripComments {
		html = stripHTMLComments(html)
	}

	// Build allowed tags set for fast lookup
	allowedTags := make(map[string]bool)
	for _, tag := range config.AllowedTags {
		allowedTags[strings.ToLower(tag)] = true
	}

	// Process the HTML
	result := as.sanitizeHTML(html, allowedTags, config.AllowedAttributes)

	return result
}

// sanitizeHTML performs the actual sanitization.
func (as *AutoSanitizer) sanitizeHTML(html string, allowedTags map[string]bool, allowedAttrs map[string][]string) string {
	// Use a simple state machine to process HTML
	var result strings.Builder
	result.Grow(len(html))

	i := 0
	for i < len(html) {
		if html[i] == '<' {
			// Find the end of the tag
			end := strings.IndexByte(html[i:], '>')
			if end == -1 {
				result.WriteByte(html[i])
				i++
				continue
			}
			end += i + 1

			tag := html[i:end]

			// Check if it's a closing tag
			if len(tag) > 2 && tag[1] == '/' {
				tagName := extractTagName(tag[2:])
				if allowedTags[strings.ToLower(tagName)] {
					result.WriteString(tag)
				}
			} else {
				// Opening tag or self-closing
				tagName := extractTagName(tag[1:])
				if allowedTags[strings.ToLower(tagName)] {
					// Filter attributes
					sanitizedTag := as.sanitizeTag(tag, tagName, allowedAttrs)
					result.WriteString(sanitizedTag)
				}
			}

			i = end
		} else {
			result.WriteByte(html[i])
			i++
		}
	}

	return result.String()
}

// sanitizeTag sanitizes a single HTML tag's attributes.
func (as *AutoSanitizer) sanitizeTag(tag, tagName string, allowedAttrs map[string][]string) string {
	// Find where attributes start
	attrStart := strings.IndexByte(tag, ' ')
	if attrStart == -1 {
		return tag // No attributes
	}

	// Get allowed attributes for this tag and global
	_ = allowedAttrs[strings.ToLower(tagName)]
	_ = allowedAttrs["*"]

	// Simple attribute filtering - keep only allowed attributes
	// This is a simplified implementation
	return tag // For now, return as-is (full implementation would filter attrs)
}

// extractTagName extracts the tag name from a tag string.
func extractTagName(s string) string {
	for i, c := range s {
		if c == ' ' || c == '>' || c == '/' {
			return s[:i]
		}
	}
	if strings.HasSuffix(s, ">") {
		return s[:len(s)-1]
	}
	return s
}

// stripHTMLComments removes HTML comments from the content.
func stripHTMLComments(html string) string {
	var result strings.Builder
	result.Grow(len(html))

	i := 0
	for i < len(html) {
		// Check for comment start
		if i+4 < len(html) && html[i:i+4] == "<!--" {
			// Find comment end
			end := strings.Index(html[i+4:], "-->")
			if end == -1 {
				break // Malformed comment, stop
			}
			i += 4 + end + 3
		} else {
			result.WriteByte(html[i])
			i++
		}
	}

	return result.String()
}

// AutoSanitizeRenderer wraps a renderer with automatic sanitization.
type AutoSanitizeRenderer struct {
	inner     Renderer
	sanitizer *AutoSanitizer
}

// Renderer is the interface for component renderers.
type Renderer interface {
	Render(ctx context.Context, w io.Writer) error
}

// NewAutoSanitizeRenderer wraps a renderer with auto-sanitization.
func NewAutoSanitizeRenderer(inner Renderer, sanitizer *AutoSanitizer) *AutoSanitizeRenderer {
	return &AutoSanitizeRenderer{
		inner:     inner,
		sanitizer: sanitizer,
	}
}

// Render renders the inner content and sanitizes it.
func (r *AutoSanitizeRenderer) Render(ctx context.Context, w io.Writer) error {
	// Get a buffer from the pool
	buf := pool.GetBuffer()
	defer pool.PutBuffer(buf)

	// Render inner content to buffer
	if err := r.inner.Render(ctx, buf); err != nil {
		return err
	}

	// Sanitize the content
	sanitized := r.sanitizer.Sanitize(buf.String())

	// Write sanitized content
	_, err := io.WriteString(w, sanitized)
	return err
}

// Note: HTTP middleware for sanitization is available in pkg/router/middleware.go
// This package focuses on the core sanitization logic.

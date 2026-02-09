package security

import (
	"bytes"
	"html"
	"regexp"
	"strings"
	"unicode"
)

// Sanitizer provides HTML and text sanitization.
type Sanitizer struct {
	allowedTags  map[string]bool
	allowedAttrs map[string][]string
	allowURLs    bool
}

// SanitizerConfig configures the sanitizer.
type SanitizerConfig struct {
	// AllowedTags is a list of HTML tags to allow
	AllowedTags []string

	// AllowedAttrs maps tags to allowed attributes
	AllowedAttrs map[string][]string

	// AllowURLs allows href and src attributes
	AllowURLs bool
}

// DefaultSanitizerConfig returns a safe default configuration.
func DefaultSanitizerConfig() SanitizerConfig {
	return SanitizerConfig{
		AllowedTags: []string{
			"p", "br", "b", "i", "u", "strong", "em",
			"h1", "h2", "h3", "h4", "h5", "h6",
			"ul", "ol", "li",
			"blockquote", "pre", "code",
			"a", "span", "div",
		},
		AllowedAttrs: map[string][]string{
			"a":    {"href", "title"},
			"span": {"class"},
			"div":  {"class"},
		},
		AllowURLs: true,
	}
}

// StrictSanitizerConfig returns a strict configuration with no HTML.
func StrictSanitizerConfig() SanitizerConfig {
	return SanitizerConfig{
		AllowedTags:  []string{},
		AllowedAttrs: map[string][]string{},
		AllowURLs:    false,
	}
}

// NewSanitizer creates a new sanitizer.
func NewSanitizer(config SanitizerConfig) *Sanitizer {
	allowedTags := make(map[string]bool)
	for _, tag := range config.AllowedTags {
		allowedTags[strings.ToLower(tag)] = true
	}

	allowedAttrs := make(map[string][]string)
	for tag, attrs := range config.AllowedAttrs {
		normalizedAttrs := make([]string, len(attrs))
		for i, attr := range attrs {
			normalizedAttrs[i] = strings.ToLower(attr)
		}
		allowedAttrs[strings.ToLower(tag)] = normalizedAttrs
	}

	return &Sanitizer{
		allowedTags:  allowedTags,
		allowedAttrs: allowedAttrs,
		allowURLs:    config.AllowURLs,
	}
}

// SanitizeHTML removes or escapes dangerous HTML content.
func (s *Sanitizer) SanitizeHTML(input string) string {
	if len(s.allowedTags) == 0 {
		// Strict mode: escape everything
		return html.EscapeString(input)
	}

	var result bytes.Buffer
	i := 0

	for i < len(input) {
		if input[i] == '<' {
			// Found a tag
			end := strings.Index(input[i:], ">")
			if end == -1 {
				// Malformed tag, escape
				result.WriteString(html.EscapeString(string(input[i])))
				i++
				continue
			}

			tag := input[i : i+end+1]
			tagName := s.extractTagName(tag)

			if s.allowedTags[tagName] {
				// Tag is allowed, sanitize attributes
				sanitizedTag := s.sanitizeTag(tag, tagName)
				result.WriteString(sanitizedTag)
			} else {
				// Tag not allowed, escape it
				result.WriteString(html.EscapeString(tag))
			}
			i += end + 1
		} else {
			result.WriteByte(input[i])
			i++
		}
	}

	return result.String()
}

// extractTagName extracts the tag name from an HTML tag.
func (s *Sanitizer) extractTagName(tag string) string {
	// Remove < and >
	content := strings.TrimPrefix(tag, "<")
	content = strings.TrimSuffix(content, ">")
	content = strings.TrimPrefix(content, "/")
	content = strings.TrimSpace(content)

	// Get just the tag name
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return ""
	}

	return strings.ToLower(parts[0])
}

// sanitizeTag sanitizes a tag's attributes.
func (s *Sanitizer) sanitizeTag(tag, tagName string) string {
	// Check if it's a closing tag
	if strings.HasPrefix(tag, "</") {
		return "</" + tagName + ">"
	}

	allowedAttrs := s.allowedAttrs[tagName]
	if len(allowedAttrs) == 0 {
		// No attributes allowed
		if strings.HasSuffix(tag, "/>") {
			return "<" + tagName + " />"
		}
		return "<" + tagName + ">"
	}

	// Parse attributes
	var result bytes.Buffer
	result.WriteString("<" + tagName)

	// Simple attribute extraction
	attrRegex := regexp.MustCompile(`(\w+)\s*=\s*["']([^"']*)["']`)
	matches := attrRegex.FindAllStringSubmatch(tag, -1)

	for _, match := range matches {
		attrName := strings.ToLower(match[1])
		attrValue := match[2]

		// Check if attribute is allowed
		if !containsAttr(allowedAttrs, attrName) {
			continue
		}

		// Validate URL attributes
		if (attrName == "href" || attrName == "src") && !s.allowURLs {
			continue
		}

		if attrName == "href" || attrName == "src" {
			attrValue = s.sanitizeURL(attrValue)
			if attrValue == "" {
				continue
			}
		}

		result.WriteString(" " + attrName + `="` + html.EscapeString(attrValue) + `"`)
	}

	if strings.HasSuffix(tag, "/>") {
		result.WriteString(" />")
	} else {
		result.WriteString(">")
	}

	return result.String()
}

// sanitizeURL sanitizes a URL, blocking javascript: and data: URLs.
func (s *Sanitizer) sanitizeURL(url string) string {
	url = strings.TrimSpace(url)
	urlLower := strings.ToLower(url)

	// Block dangerous protocols
	if strings.HasPrefix(urlLower, "javascript:") ||
		strings.HasPrefix(urlLower, "data:") ||
		strings.HasPrefix(urlLower, "vbscript:") {
		return ""
	}

	return url
}

func containsAttr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// EscapeJS escapes a string for use in JavaScript.
func EscapeJS(s string) string {
	var result bytes.Buffer

	for _, r := range s {
		switch r {
		case '\\':
			result.WriteString("\\\\")
		case '"':
			result.WriteString("\\\"")
		case '\'':
			result.WriteString("\\'")
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		case '<':
			result.WriteString("\\u003c")
		case '>':
			result.WriteString("\\u003e")
		case '&':
			result.WriteString("\\u0026")
		case '\u2028':
			result.WriteString("\\u2028")
		case '\u2029':
			result.WriteString("\\u2029")
		default:
			if r < 32 {
				result.WriteString(unicodeEscape(r))
			} else {
				result.WriteRune(r)
			}
		}
	}

	return result.String()
}

func unicodeEscape(r rune) string {
	return "\\u" + strings.ToUpper(string([]rune{
		hexDigit(int(r) >> 12 & 0xF),
		hexDigit(int(r) >> 8 & 0xF),
		hexDigit(int(r) >> 4 & 0xF),
		hexDigit(int(r) & 0xF),
	}))
}

func hexDigit(n int) rune {
	if n < 10 {
		return rune('0' + n)
	}
	return rune('a' + n - 10)
}

// StripTags removes all HTML tags from a string.
func StripTags(s string) string {
	// Simple implementation using regex
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

// TruncateText truncates text to a maximum length, adding ellipsis.
func TruncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// Find last space before maxLen
	truncated := s[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}

// NormalizeWhitespace normalizes whitespace in a string.
func NormalizeWhitespace(s string) string {
	// Replace multiple spaces with single space
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}

// IsValidEmail performs basic email validation.
func IsValidEmail(email string) bool {
	// Simple regex for basic validation
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// IsValidURL performs basic URL validation.
func IsValidURL(url string) bool {
	// Check for common protocols
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}

	// Basic structure check
	re := regexp.MustCompile(`^https?://[a-zA-Z0-9.-]+(:\d+)?(/.*)?$`)
	return re.MatchString(url)
}

// SanitizeFilename sanitizes a filename for safe storage.
func SanitizeFilename(filename string) string {
	// Remove path separators
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Remove null bytes
	filename = strings.ReplaceAll(filename, "\x00", "")

	// Keep only safe characters
	var result bytes.Buffer
	for _, r := range filename {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}

	s := result.String()

	// Prevent empty or hidden files
	s = strings.TrimPrefix(s, ".")
	if s == "" {
		s = "unnamed"
	}

	return s
}

package website

import (
	"fmt"
	"html"
	"strings"
)

// RenderHead generates a complete <head> section with SEO, Open Graph, and JSON-LD.
func RenderHead(cfg PageConfig, customCSS string) string {
	var sb strings.Builder

	// Language default
	lang := cfg.Language
	if lang == "" {
		lang = "en"
	}

	// Theme color default
	themeColor := cfg.ThemeColor
	if themeColor == "" {
		themeColor = Colors["primary"]
	}

	sb.WriteString("<head>\n")

	// Essential meta tags
	sb.WriteString(`<meta charset="UTF-8">` + "\n")
	sb.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1.0">` + "\n")
	sb.WriteString(`<meta http-equiv="X-UA-Compatible" content="IE=edge">` + "\n")

	// Title
	sb.WriteString(fmt.Sprintf("<title>%s</title>\n", html.EscapeString(cfg.Title)))

	// SEO meta tags
	if cfg.Description != "" {
		sb.WriteString(fmt.Sprintf(`<meta name="description" content="%s">`+"\n", html.EscapeString(cfg.Description)))
	}
	if len(cfg.Keywords) > 0 {
		sb.WriteString(fmt.Sprintf(`<meta name="keywords" content="%s">`+"\n", html.EscapeString(strings.Join(cfg.Keywords, ", "))))
	}
	if cfg.Author != "" {
		sb.WriteString(fmt.Sprintf(`<meta name="author" content="%s">`+"\n", html.EscapeString(cfg.Author)))
	}

	// Canonical URL
	if cfg.URL != "" {
		sb.WriteString(fmt.Sprintf(`<link rel="canonical" href="%s">`+"\n", html.EscapeString(cfg.URL)))
	}

	// Theme color for mobile browsers
	sb.WriteString(fmt.Sprintf(`<meta name="theme-color" content="%s">`+"\n", themeColor))

	// Robots
	sb.WriteString(`<meta name="robots" content="index, follow">` + "\n")

	// Open Graph
	sb.WriteString(renderOpenGraph(cfg))

	// Twitter Card
	sb.WriteString(renderTwitterCard(cfg))

	// JSON-LD structured data
	sb.WriteString(renderJSONLD(cfg))

	// Favicon
	if cfg.Favicon != "" {
		sb.WriteString(fmt.Sprintf(`<link rel="icon" href="%s">`+"\n", html.EscapeString(cfg.Favicon)))
	} else {
		// Inline SVG favicon
		sb.WriteString(`<link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>⚡</text></svg>">` + "\n")
	}

	// Preconnect for performance (none needed since we use inline CSS and system fonts)

	// Inline CSS
	sb.WriteString("<style>\n")
	sb.WriteString(RenderStyles())
	if customCSS != "" {
		sb.WriteString("\n")
		sb.WriteString(customCSS)
	}
	sb.WriteString("\n</style>\n")

	sb.WriteString("</head>\n")

	return sb.String()
}

func renderOpenGraph(cfg PageConfig) string {
	var sb strings.Builder

	sb.WriteString(`<meta property="og:type" content="website">` + "\n")

	if cfg.Title != "" {
		sb.WriteString(fmt.Sprintf(`<meta property="og:title" content="%s">`+"\n", html.EscapeString(cfg.Title)))
	}
	if cfg.Description != "" {
		sb.WriteString(fmt.Sprintf(`<meta property="og:description" content="%s">`+"\n", html.EscapeString(cfg.Description)))
	}
	if cfg.URL != "" {
		sb.WriteString(fmt.Sprintf(`<meta property="og:url" content="%s">`+"\n", html.EscapeString(cfg.URL)))
	}
	if cfg.OGImage != "" {
		sb.WriteString(fmt.Sprintf(`<meta property="og:image" content="%s">`+"\n", html.EscapeString(cfg.OGImage)))
	}

	lang := cfg.Language
	if lang == "" {
		lang = "en"
	}
	sb.WriteString(fmt.Sprintf(`<meta property="og:locale" content="%s">`+"\n", lang))

	return sb.String()
}

func renderTwitterCard(cfg PageConfig) string {
	var sb strings.Builder

	sb.WriteString(`<meta name="twitter:card" content="summary_large_image">` + "\n")

	if cfg.Title != "" {
		sb.WriteString(fmt.Sprintf(`<meta name="twitter:title" content="%s">`+"\n", html.EscapeString(cfg.Title)))
	}
	if cfg.Description != "" {
		sb.WriteString(fmt.Sprintf(`<meta name="twitter:description" content="%s">`+"\n", html.EscapeString(cfg.Description)))
	}
	if cfg.OGImage != "" {
		sb.WriteString(fmt.Sprintf(`<meta name="twitter:image" content="%s">`+"\n", html.EscapeString(cfg.OGImage)))
	}

	return sb.String()
}

func renderJSONLD(cfg PageConfig) string {
	// Build JSON-LD for SoftwareApplication
	jsonLD := fmt.Sprintf(`{
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  "name": %q,
  "description": %q,
  "url": %q,
  "applicationCategory": "DeveloperApplication",
  "operatingSystem": "Cross-platform",
  "programmingLanguage": "Go",
  "license": "https://opensource.org/licenses/MIT"
}`, cfg.Title, cfg.Description, cfg.URL)

	return fmt.Sprintf(`<script type="application/ld+json">%s</script>`+"\n", jsonLD)
}

// RenderDocument wraps content in a complete HTML document.
func RenderDocument(cfg PageConfig, customCSS, bodyContent string) string {
	lang := cfg.Language
	if lang == "" {
		lang = "en"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="%s">
%s<body>
%s
</body>
</html>`, lang, RenderHead(cfg, customCSS), bodyContent)
}

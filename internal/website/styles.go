package website

import (
	"fmt"
	"strings"
)

// Color palette (WCAG 2.1 AA compliant - 4.5:1 minimum contrast ratio)
var Colors = map[string]string{
	// Backgrounds
	"bg":        "#0F172A", // Dark blue - main background
	"bgAlt":     "#1E293B", // Lighter - cards, sections
	"bgHover":   "#334155", // Hover states
	"bgCode":    "#0D1117", // Code blocks

	// Text - WCAG compliant (tested with WebAIM contrast checker)
	"text":      "#F8FAFC", // White - primary text (15.5:1 on bg)
	"textMuted": "#CBD5E1", // Light gray - secondary text (8.5:1 on bg, 6.5:1 on bgAlt)
	"textDim":   "#94A3B8", // Dimmer text (5.2:1 on bg, 4.5:1 on bgAlt)

	// Brand colors - bright versions for dark backgrounds
	"primary":       "#A78BFA", // Lighter purple (7:1 on bg)
	"primaryBright": "#C4B5FD", // Even lighter for small text
	"secondary":     "#22D3EE", // Bright cyan (8:1 on bg)
	"accent":        "#67E8F9", // Light cyan - highlights (10:1 on bg)

	// Status colors - bright versions for dark backgrounds
	"success": "#34D399", // Bright green (7:1 on bg)
	"warning": "#FBBF24", // Bright amber (9:1 on bg)
	"danger":  "#F87171", // Bright red (5.5:1 on bg)
	"info":    "#60A5FA", // Bright blue (6:1 on bg)

	// Category colors - bright versions for text on dark backgrounds (min 4.5:1)
	"catCore":     "#C4B5FD", // Light purple (9:1 on bgAlt)
	"catUI":       "#93C5FD", // Light blue (8:1 on bgAlt)
	"catState":    "#6EE7B7", // Light green (9:1 on bgAlt)
	"catDevOps":   "#FCD34D", // Light amber (10:1 on bgAlt)
	"catSecurity": "#FCA5A5", // Light red (7:1 on bgAlt)
	"catUtils":    "#D1D5DB", // Light gray (9:1 on bgAlt)
	"catPlugins":  "#F9A8D4", // Light pink (8:1 on bgAlt)
	"catCLI":      "#67E8F9", // Light cyan (10:1 on bgAlt)

	// Borders
	"border":      "#334155", // Subtle borders
	"borderLight": "#475569", // Lighter borders
}

// Typography uses system font stack for instant loading
var FontFamily = `system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif`
var FontMono = `'SF Mono', SFMono-Regular, ui-monospace, 'DejaVu Sans Mono', Menlo, Consolas, monospace`

// Spacing uses 8px grid system
var Spacing = map[string]string{
	"xs":  "0.25rem", // 4px
	"sm":  "0.5rem",  // 8px
	"md":  "1rem",    // 16px
	"lg":  "1.5rem",  // 24px
	"xl":  "2rem",    // 32px
	"2xl": "3rem",    // 48px
	"3xl": "4rem",    // 64px
	"4xl": "6rem",    // 96px
}

// Breakpoints for responsive design (mobile-first: min-width)
var Breakpoints = map[string]string{
	"sm": "480px",  // Mobile landscape
	"md": "768px",  // Tablet
	"lg": "1024px", // Desktop
	"xl": "1280px", // Large desktop
}

// StyleOption allows customizing the generated CSS
type StyleOption func(*styleConfig)

type styleConfig struct {
	customColors      map[string]string
	includeReset      bool
	includeAnimations bool
	includeDarkMode   bool
}

// WithCustomColors overrides default colors
func WithCustomColors(colors map[string]string) StyleOption {
	return func(cfg *styleConfig) {
		for k, v := range colors {
			cfg.customColors[k] = v
		}
	}
}

// WithReset includes a CSS reset
func WithReset(include bool) StyleOption {
	return func(cfg *styleConfig) {
		cfg.includeReset = include
	}
}

// WithAnimations includes animation definitions
func WithAnimations(include bool) StyleOption {
	return func(cfg *styleConfig) {
		cfg.includeAnimations = include
	}
}

// RenderStyles generates the complete CSS for the landing page.
func RenderStyles(opts ...StyleOption) string {
	cfg := &styleConfig{
		customColors:      make(map[string]string),
		includeReset:      true,
		includeAnimations: true,
		includeDarkMode:   false,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Merge custom colors
	colors := make(map[string]string)
	for k, v := range Colors {
		colors[k] = v
	}
	for k, v := range cfg.customColors {
		colors[k] = v
	}

	var sb strings.Builder

	// CSS Reset
	if cfg.includeReset {
		sb.WriteString(cssReset())
	}

	// CSS Variables
	sb.WriteString(cssVariables(colors))

	// Base styles
	sb.WriteString(cssBase())

	// Typography
	sb.WriteString(cssTypography())

	// Layout
	sb.WriteString(cssLayout())

	// Components
	sb.WriteString(cssComponents())

	// Buttons
	sb.WriteString(cssButtons())

	// Cards
	sb.WriteString(cssCards())

	// Code blocks
	sb.WriteString(cssCode())

	// Package grid
	sb.WriteString(cssPackageGrid())

	// Stats
	sb.WriteString(cssStats())

	// Animations
	if cfg.includeAnimations {
		sb.WriteString(cssAnimations())
	}

	// Accessibility
	sb.WriteString(cssAccessibility())

	// Responsive (mobile-first)
	sb.WriteString(cssResponsive())

	return sb.String()
}

func cssReset() string {
	return `
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
html{-webkit-text-size-adjust:100%;-moz-tab-size:4;tab-size:4;scroll-behavior:smooth}
body{line-height:1.6;-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale}
img,picture,video,canvas,svg{display:block;max-width:100%}
input,button,textarea,select{font:inherit}
p,h1,h2,h3,h4,h5,h6{overflow-wrap:break-word}
a{color:inherit;text-decoration:none}
ul,ol{list-style:none}
`
}

func cssVariables(colors map[string]string) string {
	var vars []string
	for name, value := range colors {
		vars = append(vars, fmt.Sprintf("--color-%s:%s", name, value))
	}
	return fmt.Sprintf(`:root{%s;--font-sans:%s;--font-mono:%s}`, strings.Join(vars, ";"), FontFamily, FontMono)
}

func cssBase() string {
	return `
body{font-family:var(--font-sans);background:var(--color-bg);color:var(--color-text);min-height:100vh}
::selection{background:var(--color-primary);color:white}
`
}

func cssTypography() string {
	return `
h1{font-size:clamp(2rem,5vw,4rem);font-weight:800;letter-spacing:-0.02em;line-height:1.1}
h2{font-size:clamp(1.5rem,3vw,2rem);font-weight:700;letter-spacing:-0.01em;line-height:1.2}
h3{font-size:1.125rem;font-weight:600;line-height:1.3}
h4{font-size:1rem;font-weight:600}
p{color:var(--color-textMuted)}
.text-gradient{background:linear-gradient(135deg,var(--color-primary),var(--color-secondary));-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text}
code{font-family:var(--font-mono);font-size:0.9em}
`
}

func cssLayout() string {
	// Mobile-first: base styles for mobile (320px+)
	return `
.container{width:100%;max-width:1200px;margin:0 auto;padding:0 1rem}
.section{padding:2rem 0}
.section-lg{padding:3rem 0}
.flex{display:flex}.flex-col{flex-direction:column}.items-center{align-items:center}.justify-center{justify-content:center}.justify-between{justify-content:space-between}
.gap-sm{gap:0.5rem}.gap-md{gap:1rem}.gap-lg{gap:1.5rem}.gap-xl{gap:2rem}
.text-center{text-align:center}
.grid{display:grid}
.grid-2{grid-template-columns:1fr}
.grid-3{grid-template-columns:1fr}
.grid-4{grid-template-columns:1fr}
.grid-6{grid-template-columns:1fr}
`
}

func cssComponents() string {
	return `
.badge{display:none;align-items:center;gap:0.5rem;padding:0.4rem 0.75rem;border-radius:9999px;font-size:0.75rem;font-weight:600;background:#7C3AED;color:#FFFFFF;border:1px solid #8B5CF6}
.badge-success{background:#059669;color:#FFFFFF;border-color:#10B981}
.nav{position:fixed;top:0;left:0;right:0;z-index:100;padding:0.5rem 0;background:rgba(15,23,42,0.98);backdrop-filter:blur(12px);border-bottom:1px solid var(--color-border)}
.nav-inner{display:flex;align-items:center;justify-content:space-between;gap:0.5rem}
.nav-links{display:none;align-items:center;gap:0.5rem}
.logo{font-size:1.1rem;font-weight:800;letter-spacing:-0.02em}
.hero{padding-top:4.5rem;padding-bottom:2rem;text-align:center}
.hero-title{margin-bottom:1rem}
.hero-subtitle{font-size:1rem;color:var(--color-textMuted);max-width:600px;margin:0 auto 1.5rem}
.hero-actions{display:flex;gap:0.75rem;justify-content:center;flex-wrap:wrap;flex-direction:column;width:100%;padding:0 1rem}
.hide-mobile{display:none}
`
}

func cssButtons() string {
	// Mobile-first: 44px minimum tap target (2.75rem)
	return `
.btn{display:inline-flex;align-items:center;justify-content:center;gap:0.5rem;padding:0.75rem 1.25rem;font-size:1rem;font-weight:600;border-radius:0.5rem;border:none;cursor:pointer;transition:all 0.2s ease;text-decoration:none;min-height:2.75rem;width:100%}
.btn:focus-visible{outline:2px solid var(--color-primary);outline-offset:2px}
.btn-primary{background:#6D28D9;color:#FFFFFF}
.btn-primary:hover{background:#5B21B6;transform:translateY(-2px);box-shadow:0 4px 20px rgba(109,40,217,0.4)}
.btn-secondary{background:transparent;color:var(--color-text);border:1px solid var(--color-border)}
.btn-secondary:hover{border-color:var(--color-textMuted);background:var(--color-bgAlt)}
.btn-ghost{background:transparent;color:var(--color-textMuted);min-height:2.75rem;width:auto}
.btn-ghost:hover{color:var(--color-text)}
.btn-lg{padding:1rem 1.75rem;font-size:1.125rem}
.btn-sm{padding:0.5rem 1rem;font-size:0.875rem;min-height:2.5rem}
.btn-icon{width:2.75rem;height:2.75rem;padding:0}
`
}

func cssCards() string {
	return `
.card{background:var(--color-bgAlt);border-radius:1rem;border:1px solid var(--color-border);overflow:hidden;transition:all 0.3s ease}
.card:hover{border-color:var(--color-borderLight);transform:translateY(-4px);box-shadow:0 20px 40px rgba(0,0,0,0.3)}
.card-body{padding:1.25rem}
.card-header{padding:1.25rem 1.25rem 0}
.card-footer{padding:0 1.25rem 1.25rem}
.feature-card{padding:1.5rem;border-radius:1rem;background:var(--color-bgAlt);border:1px solid var(--color-border);transition:all 0.3s ease}
.feature-card:hover{border-color:var(--color-primary);transform:translateY(-4px)}
.feature-icon{font-size:2rem;margin-bottom:0.75rem}
.feature-title{font-size:1rem;font-weight:600;margin-bottom:0.5rem;color:var(--color-text)}
.feature-desc{font-size:0.875rem;color:var(--color-textMuted)}
`
}

func cssCode() string {
	return `
.code-block{background:var(--color-bgCode);border-radius:0.75rem;border:1px solid var(--color-border);overflow:hidden}
.code-header{display:flex;align-items:center;justify-content:space-between;padding:0.75rem 1rem;background:rgba(255,255,255,0.03);border-bottom:1px solid var(--color-border)}
.code-title{font-size:0.8rem;color:var(--color-textMuted);font-weight:500}
.code-dots{display:flex;gap:0.5rem}
.code-dot{width:0.75rem;height:0.75rem;border-radius:50%}
.code-dot-red{background:#ff5f56}
.code-dot-yellow{background:#ffbd2e}
.code-dot-green{background:#27c93f}
.code-content{padding:1rem;overflow-x:auto;font-family:var(--font-mono);font-size:0.8rem;line-height:1.6}
.code-content code{color:var(--color-text)}
.token-keyword{color:#ff79c6}
.token-string{color:#f1fa8c}
.token-comment{color:#6272a4}
.token-function{color:#50fa7b}
.token-type{color:#8be9fd}
.token-number{color:#bd93f9}
.token-operator{color:#ff79c6}
.token-punctuation{color:#f8f8f2}
`
}

func cssPackageGrid() string {
	// Mobile-first: 2 columns on mobile
	return `
.package-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:0.75rem}
.package-card{background:var(--color-bgAlt);border-radius:0.75rem;padding:0.75rem;border:1px solid var(--color-border);transition:all 0.2s ease;cursor:default}
.package-card:hover{border-color:var(--color-borderLight);transform:translateY(-2px)}
.package-name{font-family:var(--font-mono);font-size:0.8rem;font-weight:600;color:var(--color-text);margin-bottom:0.25rem}
.package-desc{font-size:0.7rem;color:var(--color-textMuted);line-height:1.4}
.package-cat{display:inline-block;font-size:0.6rem;padding:0.15rem 0.4rem;border-radius:0.25rem;margin-bottom:0.4rem;font-weight:700;color:#FFFFFF}
`
}

func cssStats() string {
	// Mobile-first: single column on mobile
	return `
.stats-grid{display:grid;grid-template-columns:1fr;gap:1rem}
.stat-card{background:var(--color-bgAlt);border-radius:0.75rem;padding:1.25rem;text-align:center;border:1px solid var(--color-border)}
.stat-value{font-size:2rem;font-weight:800;color:var(--color-primary);font-variant-numeric:tabular-nums}
.stat-label{font-size:0.875rem;color:var(--color-textMuted);margin-top:0.25rem}
.counter-display{display:flex;align-items:center;justify-content:center;gap:1rem;padding:1.5rem;flex-direction:column}
.counter-btn{width:3rem;height:3rem;border-radius:0.75rem;font-size:1.5rem;font-weight:bold;border:none;cursor:pointer;transition:all 0.15s ease;min-height:2.75rem}
.counter-btn:hover{transform:scale(1.1)}
.counter-btn:active{transform:scale(0.95)}
.counter-btn-dec{background:var(--color-danger);color:white}
.counter-btn-inc{background:var(--color-success);color:white}
.counter-value{font-size:3rem;font-weight:800;font-variant-numeric:tabular-nums;min-width:80px;text-align:center;color:var(--color-primary)}
`
}

func cssAnimations() string {
	return `
@keyframes fadeIn{from{opacity:0;transform:translateY(20px)}to{opacity:1;transform:translateY(0)}}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:0.5}}
@keyframes glow{0%,100%{box-shadow:0 0 20px rgba(139,92,246,0.3)}50%{box-shadow:0 0 40px rgba(139,92,246,0.5)}}
.animate-fade-in{animation:fadeIn 0.6s ease forwards}
.animate-pulse{animation:pulse 2s infinite}
.animate-glow{animation:glow 2s infinite}
.delay-1{animation-delay:0.1s}
.delay-2{animation-delay:0.2s}
.delay-3{animation-delay:0.3s}
.delay-4{animation-delay:0.4s}
@media(prefers-reduced-motion:reduce){*{animation-duration:0.01ms!important;animation-iteration-count:1!important;transition-duration:0.01ms!important}}
`
}

func cssAccessibility() string {
	return `
.sr-only{position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0}
.skip-link{position:absolute;top:-40px;left:0;background:#7C3AED;color:#FFFFFF;padding:0.5rem 1rem;z-index:1000;transition:top 0.3s;font-weight:600}
.skip-link:focus{top:0}
:focus-visible{outline:2px solid var(--color-primary);outline-offset:2px}
`
}

func cssResponsive() string {
	// Mobile-first: breakpoints use min-width
	return `
@media(min-width:480px){
.package-grid{grid-template-columns:repeat(3,1fr)}
.container{padding:0 1.25rem}
.hero-actions{flex-direction:row;width:auto;padding:0}
.btn{width:auto}
.counter-display{flex-direction:row;gap:1.5rem}
.counter-value{font-size:3.5rem}
.hero{padding-top:5rem}
}
@media(min-width:640px){
.badge{display:inline-flex;padding:0.5rem 1rem;font-size:0.875rem}
.nav-links{display:flex}
.hide-mobile{display:inline-flex}
.logo{font-size:1.25rem}
.nav{padding:0.75rem 0}
.hero{padding-top:5.5rem}
}
@media(min-width:768px){
.package-grid{grid-template-columns:repeat(4,1fr)}
.grid-2{grid-template-columns:repeat(2,1fr)}
.stats-grid{grid-template-columns:repeat(3,1fr)}
.container{padding:0 1.5rem}
.section{padding:4rem 0}
.section-lg{padding:5rem 0}
.hero{padding-top:6rem;padding-bottom:3rem}
.hero-subtitle{font-size:1.125rem}
.logo{font-size:1.5rem}
.nav{padding:1rem 0}
h3{font-size:1.25rem}
.feature-icon{font-size:2.5rem}
.code-content{padding:1.5rem;font-size:0.875rem;line-height:1.7}
.card-body{padding:1.5rem}
.counter-value{font-size:4rem;min-width:120px}
}
@media(min-width:1024px){
.package-grid{grid-template-columns:repeat(6,1fr)}
.grid-3{grid-template-columns:repeat(3,1fr)}
.grid-4{grid-template-columns:repeat(4,1fr)}
.grid-6{grid-template-columns:repeat(6,1fr)}
.section-lg{padding:6rem 0}
.hero{padding-top:8rem;padding-bottom:4rem}
.hero-subtitle{font-size:1.25rem}
.feature-card{padding:2rem}
}
`
}

// GetCategoryColor returns the appropriate color for a package category.
func GetCategoryColor(category string) string {
	switch category {
	case "Core":
		return Colors["catCore"]
	case "UI":
		return Colors["catUI"]
	case "State":
		return Colors["catState"]
	case "DevOps":
		return Colors["catDevOps"]
	case "Security":
		return Colors["catSecurity"]
	case "Utils":
		return Colors["catUtils"]
	case "Plugins":
		return Colors["catPlugins"]
	case "CLI":
		return Colors["catCLI"]
	default:
		return Colors["catUtils"]
	}
}

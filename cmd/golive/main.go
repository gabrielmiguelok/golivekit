// Command golive is the CLI tool for GoliveKit applications.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "new":
		if len(os.Args) < 3 {
			fmt.Println("Error: project name required")
			fmt.Println("Usage: golive new <project-name>")
			os.Exit(1)
		}
		if err := newProject(os.Args[2]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "dev":
		if err := runDev(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "build":
		if err := runBuild(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "generate", "gen":
		if len(os.Args) < 3 {
			fmt.Println("Error: generator type required")
			fmt.Println("Usage: golive generate <type> <name>")
			fmt.Println("Types: component, live, scaffold")
			os.Exit(1)
		}
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "version", "-v", "--version":
		fmt.Printf("GoliveKit CLI v%s\n", version)

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`GoliveKit CLI v%s

Usage: golive <command> [arguments]

Commands:
  new <name>           Create a new GoliveKit project
  dev                  Start development server with hot reload
  build                Build for production
  generate <type>      Generate code (component, live, scaffold)
  version              Show version
  help                 Show this help

Examples:
  golive new myapp
  golive dev
  golive build
  golive generate component Counter
  golive generate live ChatRoom

For more information, visit: https://github.com/gabrielmiguelok/golivekit
`, version)
}

func newProject(name string) error {
	fmt.Printf("Creating new GoliveKit project: %s\n", name)

	// Create project directory
	if err := os.MkdirAll(name, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create subdirectories
	dirs := []string{
		"cmd/server",
		"internal/components",
		"internal/handlers",
		"web/templates",
		"web/static/css",
		"web/static/js",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(name+"/"+dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Create main.go using router - mobile-first with accessibility
	mainGo := `package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gabrielmiguelok/golivekit/client"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/router"
)

// HomeComponent is the home page component.
type HomeComponent struct {
	core.BaseComponent
	count int
}

func NewHomeComponent() core.Component {
	return &HomeComponent{}
}

func (c *HomeComponent) Name() string {
	return "home"
}

func (c *HomeComponent) Mount(ctx context.Context, params core.Params, session core.Session) error {
	c.count = 0
	return nil
}

func (c *HomeComponent) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := fmt.Sprintf(` + "`" + `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="description" content="GoliveKit App - Real-time web application">
    <title>GoliveKit App</title>
    <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>⚡</text></svg>">
    <style>
/* Mobile-first CSS reset */
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
html{-webkit-text-size-adjust:100%;scroll-behavior:smooth}
body{font-family:system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;line-height:1.6;min-height:100vh;background:linear-gradient(135deg,#667eea 0%%,#764ba2 100%%)}

/* Skip link for accessibility */
.skip-link{position:absolute;top:-40px;left:0;background:#7C3AED;color:#fff;padding:0.5rem 1rem;z-index:1000;transition:top 0.3s}
.skip-link:focus{top:0}

/* Container - mobile-first */
.container{width:100%%;max-width:600px;margin:0 auto;padding:1rem}

/* Card */
.card{background:#fff;padding:1.5rem;border-radius:1rem;box-shadow:0 10px 40px rgba(0,0,0,0.2)}

/* Typography */
h1{font-size:1.5rem;color:#333;margin-bottom:0.5rem}
p{color:#666;margin-bottom:1rem}

/* Counter - mobile-first (vertical layout) */
.counter{display:flex;flex-direction:column;align-items:center;gap:1rem;padding:1rem}
.counter-value{font-size:3rem;font-weight:800;color:#7C3AED;min-width:80px;text-align:center}
.counter-buttons{display:flex;gap:1rem}

/* Buttons - 44px minimum tap target */
.btn{width:3rem;height:3rem;min-height:2.75rem;border-radius:0.75rem;font-size:1.5rem;font-weight:bold;border:none;cursor:pointer;transition:transform 0.15s}
.btn:hover{transform:scale(1.1)}
.btn:active{transform:scale(0.95)}
.btn:focus-visible{outline:2px solid #7C3AED;outline-offset:2px}
.btn-dec{background:#EF4444;color:#fff}
.btn-inc{background:#10B981;color:#fff}

/* Tablet+ breakpoint */
@media(min-width:480px){
.container{padding:2rem}
h1{font-size:2rem}
.counter{flex-direction:row;gap:1.5rem}
.counter-value{font-size:4rem;min-width:120px}
}

/* Reduced motion preference */
@media(prefers-reduced-motion:reduce){*{animation-duration:0.01ms!important;transition-duration:0.01ms!important}}
    </style>
</head>
<body>
    <a href="#main-content" class="skip-link">Skip to main content</a>
    <main id="main-content" class="container">
        <div data-live-view="home" class="card">
            <header>
                <h1>⚡ Welcome to GoliveKit!</h1>
                <p>Your real-time web app is ready. Try the counter below:</p>
            </header>
            <div class="counter" role="group" aria-label="Counter demo">
                <button class="btn btn-dec" lv-click="decrement" aria-label="Decrement counter">−</button>
                <output class="counter-value" aria-live="polite">%%d</output>
                <button class="btn btn-inc" lv-click="increment" aria-label="Increment counter">+</button>
            </div>
        </div>
    </main>
    <script src="/_live/golivekit.js"></script>
</body>
</html>` + "`" + `, c.count)
		_, err := w.Write([]byte(html))
		return err
	})
}

func (c *HomeComponent) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	case "increment":
		c.count++
	case "decrement":
		c.count--
	}
	return nil
}

func main() {
	r := router.New()

	// Serve static files
	r.Static("/static/", "web/static")

	// Serve GoliveKit client JS
	r.Handle("/_live/", http.StripPrefix("/_live/", client.Handler()))

	// Register LiveView routes
	r.Live("/", NewHomeComponent)

	fmt.Println("⚡ GoliveKit starting at http://localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", r))
}
`

	if err := os.WriteFile(name+"/cmd/server/main.go", []byte(mainGo), 0644); err != nil {
		return fmt.Errorf("failed to create main.go: %w", err)
	}

	// Create go.mod
	goMod := fmt.Sprintf(`module %s

go 1.21

require github.com/gabrielmiguelok/golivekit v0.1.0
`, name)

	if err := os.WriteFile(name+"/go.mod", []byte(goMod), 0644); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}

	// Create basic CSS (mobile-first)
	css := `/* GoliveKit App Styles - Mobile First */

/* Base reset */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html { -webkit-text-size-adjust: 100%; scroll-behavior: smooth; }

/* Body - mobile base */
body {
    font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    line-height: 1.6;
    min-height: 100vh;
    padding: 1rem;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

/* Live view container */
[data-live-view] {
    background: white;
    padding: 1.5rem;
    border-radius: 1rem;
    max-width: 600px;
    margin: 0 auto;
    box-shadow: 0 10px 40px rgba(0,0,0,0.2);
}

/* Typography */
h1 { color: #333; font-size: 1.5rem; margin-bottom: 0.5rem; }
p { color: #666; }

/* Buttons - 44px minimum tap target */
button {
    min-height: 2.75rem;
    padding: 0.75rem 1.5rem;
    font-size: 1rem;
    border: none;
    border-radius: 0.5rem;
    cursor: pointer;
    transition: transform 0.15s ease;
}
button:hover { transform: translateY(-2px); }
button:active { transform: scale(0.98); }
button:focus-visible { outline: 2px solid #7C3AED; outline-offset: 2px; }

/* Tablet+ */
@media (min-width: 480px) {
    body { padding: 2rem; }
    [data-live-view] { padding: 2rem; }
    h1 { font-size: 2rem; }
}

/* Reduced motion */
@media (prefers-reduced-motion: reduce) {
    *, *::before, *::after {
        animation-duration: 0.01ms !important;
        transition-duration: 0.01ms !important;
    }
}
`
	if err := os.WriteFile(name+"/web/static/css/app.css", []byte(css), 0644); err != nil {
		return fmt.Errorf("failed to create app.css: %w", err)
	}

	// Create basic JS
	js := `// GoliveKit Client - auto-loaded via /_live/golivekit.js
console.log('GoliveKit app loaded');
`
	if err := os.WriteFile(name+"/web/static/js/app.js", []byte(js), 0644); err != nil {
		return fmt.Errorf("failed to create app.js: %w", err)
	}

	fmt.Printf(`
✅ Project created successfully!

Next steps:
  cd %s
  go mod tidy
  go run cmd/server/main.go

Then open http://localhost:3000 in your browser.
`, name)

	return nil
}

func runDev() error {
	fmt.Println("🔥 GoliveKit Development Server")
	fmt.Println("================================")

	// Find main file
	mainFile := findMainFile()
	if mainFile == "" {
		return fmt.Errorf("could not find main.go (tried cmd/server/main.go, main.go)")
	}

	fmt.Printf("📁 Main file: %s\n", mainFile)
	fmt.Println("👀 Watching for file changes...")
	fmt.Println("🌐 Server will start at http://localhost:3000")
	fmt.Println("Press Ctrl+C to stop")

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start the application
	cmd, err := startApp(ctx, mainFile)
	if err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	// Simple file watcher using polling (for simplicity)
	// In production, use fsnotify for proper file watching
	go watchFiles(ctx, mainFile, func() {
		fmt.Println("\n🔄 Changes detected, restarting...")
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		cmd, _ = startApp(ctx, mainFile)
	})

	// Wait for interrupt
	<-sigCh
	fmt.Println("\n👋 Shutting down...")

	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
	}

	return nil
}

func findMainFile() string {
	candidates := []string{
		"cmd/server/main.go",
		"main.go",
		"cmd/main.go",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	return ""
}

func startApp(ctx context.Context, mainFile string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "go", "run", mainFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOLIVEKIT_DEV=1")

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}

func watchFiles(ctx context.Context, mainFile string, onChange func()) {
	dir := filepath.Dir(mainFile)
	if dir == "." {
		dir = "."
	}

	lastMod := time.Now()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed := false

			// Walk directory looking for changes
			filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}

				// Skip non-Go files and hidden directories
				if info.IsDir() {
					if len(info.Name()) > 0 && info.Name()[0] == '.' {
						return filepath.SkipDir
					}
					return nil
				}

				ext := filepath.Ext(path)
				if ext != ".go" && ext != ".html" && ext != ".css" && ext != ".js" {
					return nil
				}

				if info.ModTime().After(lastMod) {
					changed = true
					fmt.Printf("📝 Changed: %s\n", path)
				}

				return nil
			})

			if changed {
				lastMod = time.Now()
				onChange()
			}
		}
	}
}

func runBuild() error {
	fmt.Println("🏗️  Building for production...")

	// Check if we're in a Go project
	if _, err := os.Stat("go.mod"); os.IsNotExist(err) {
		return fmt.Errorf("not in a Go project directory (no go.mod found)")
	}

	// Create dist directory
	if err := os.MkdirAll("dist", 0755); err != nil {
		return fmt.Errorf("failed to create dist directory: %w", err)
	}

	// Find main file
	mainFile := findMainFile()
	if mainFile == "" {
		return fmt.Errorf("could not find main.go")
	}

	fmt.Printf("📁 Building: %s\n", mainFile)

	// Build for current platform
	cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", "dist/server", mainFile)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Copy static assets
	if _, err := os.Stat("web/static"); err == nil {
		fmt.Println("📦 Copying static assets...")
		cmd = exec.Command("cp", "-r", "web/static", "dist/static")
		if err := cmd.Run(); err != nil {
			fmt.Println("⚠️  Warning: failed to copy static assets")
		}
	}

	// Generate build info
	buildInfo := fmt.Sprintf("Build Time: %s\nVersion: %s\n", time.Now().Format(time.RFC3339), version)
	os.WriteFile("dist/BUILD_INFO", []byte(buildInfo), 0644)

	fmt.Println("\n✅ Build complete!")
	fmt.Println("📦 Output: dist/server")
	fmt.Println("\nTo run the production build:")
	fmt.Println("  ./dist/server")

	return nil
}

func runGenerate(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("name required")
	}

	genType := args[0]
	name := args[1]

	switch genType {
	case "component":
		return generateComponent(name)
	case "live":
		return generateLiveComponent(name)
	case "scaffold":
		return generateScaffold(name)
	default:
		return fmt.Errorf("unknown generator type: %s", genType)
	}
}

func generateComponent(name string) error {
	fmt.Printf("Generating component: %s\n", name)

	code := fmt.Sprintf(`package components

import (
	"context"
	"io"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// %s is a basic component.
type %s struct {
	core.BaseComponent
}

// New%s creates a new %s component.
func New%s() core.Component {
	return &%s{}
}

// Name returns the component name.
func (c *%s) Name() string {
	return "%s"
}

// Mount is called when the component is mounted.
func (c *%s) Mount(ctx context.Context, params core.Params, session core.Session) error {
	// Initialize component state here
	return nil
}

// Render returns the component's HTML.
func (c *%s) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte("<div>%s Component</div>"))
		return err
	})
}

// HandleEvent handles user events.
func (c *%s) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	// Handle events here
	}
	return nil
}
`, name, name, name, name, name, name, name, name, name, name, name, name)

	filename := fmt.Sprintf("internal/components/%s.go", toSnakeCase(name))

	// Ensure directory exists
	os.MkdirAll("internal/components", 0755)

	if err := os.WriteFile(filename, []byte(code), 0644); err != nil {
		return err
	}

	fmt.Printf("✅ Created %s\n", filename)
	return nil
}

func generateLiveComponent(name string) error {
	fmt.Printf("Generating live component: %s\n", name)
	return generateComponent(name)
}

func generateScaffold(name string) error {
	fmt.Printf("Generating scaffold for: %s\n", name)

	// Generate component
	if err := generateComponent(name); err != nil {
		return err
	}

	// Generate handler
	handlerCode := fmt.Sprintf(`package handlers

import (
	"myapp/internal/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/router"
)

// Register%sRoutes registers routes for %s.
func Register%sRoutes(r *router.Router) {
	r.Live("/%s", func() core.Component {
		return components.New%s()
	})
}
`, name, name, name, toSnakeCase(name), name)

	filename := fmt.Sprintf("internal/handlers/%s.go", toSnakeCase(name))
	os.MkdirAll("internal/handlers", 0755)

	if err := os.WriteFile(filename, []byte(handlerCode), 0644); err != nil {
		return err
	}

	fmt.Printf("✅ Created %s\n", filename)
	return nil
}

func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		if r >= 'A' && r <= 'Z' {
			result = append(result, r+32)
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

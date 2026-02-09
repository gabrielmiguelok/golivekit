// Package main provides a documentation page for GoliveKit.
package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/gabrielmiguelok/golivekit/internal/website"
	"github.com/gabrielmiguelok/golivekit/internal/website/components"
	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// contentCache holds pre-rendered content for each section (initialized once)
var contentCache = sync.OnceValue(func() map[string]string {
	return map[string]string{
		"getting-started": contentGettingStarted(),
		"core-concepts":   contentCoreConcepts(),
		"routing":         contentRouting(),
		"events":          contentEvents(),
		"forms":           contentForms(),
		"realtime":        contentRealtime(),
		"state":           contentState(),
		"security":        contentSecurity(),
		"performance":     contentPerformance(),
		"cli":             contentCLI(),
		"packages":        contentPackages(),
	}
})

// DocsComponent is the documentation page component.
type DocsComponent struct {
	core.BaseComponent
	CurrentSection string
}

// NewDocs creates a new documentation component.
func NewDocs() core.Component {
	return &DocsComponent{}
}

// Name returns the component name.
func (d *DocsComponent) Name() string {
	return "docs"
}

// Mount initializes the docs component.
func (d *DocsComponent) Mount(ctx context.Context, params core.Params, session core.Session) error {
	d.CurrentSection = "getting-started"
	if section, ok := params["section"]; ok && section != "" {
		d.CurrentSection = section
	}
	return nil
}

// Terminate handles cleanup.
func (d *DocsComponent) Terminate(ctx context.Context, reason core.TerminateReason) error {
	return nil
}

// HandleEvent handles user interactions.
func (d *DocsComponent) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	if event == "nav" {
		if section, ok := payload["section"].(string); ok {
			d.CurrentSection = section
		}
	}
	return nil
}

// Render returns the HTML representation.
func (d *DocsComponent) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := d.renderDocsPage()
		_, err := w.Write([]byte(html))
		return err
	})
}

// Section defines a documentation section
type Section struct {
	ID    string
	Title string
	Icon  string
}

var docsSections = []Section{
	{"getting-started", "Getting Started", "🚀"},
	{"core-concepts", "Core Concepts", "🧩"},
	{"routing", "Routing", "🔀"},
	{"events", "Events", "⚡"},
	{"forms", "Forms & Validation", "📝"},
	{"realtime", "Real-time Updates", "🔄"},
	{"state", "State Management", "💾"},
	{"security", "Security", "🔐"},
	{"performance", "Performance", "🏎️"},
	{"cli", "CLI Reference", "⌨️"},
	{"packages", "Package Reference", "📦"},
}

func (d *DocsComponent) renderDocsPage() string {
	cfg := website.PageConfig{
		Title:       fmt.Sprintf("%s | GoliveKit Docs", d.getSectionTitle()),
		Description: "Complete documentation for GoliveKit - Phoenix LiveView for Go.",
		URL:         "https://golivekit.cloud/docs",
		Keywords:    []string{"golivekit", "documentation", "liveview", "go", "golang"},
		Author:      "Gabriel Miguel",
		Language:    "en",
		ThemeColor:  "#8B5CF6",
	}

	var body strings.Builder

	// Navbar
	body.WriteString(components.RenderNavbar(components.NavbarOptions{
		Logo:      "GoliveKit",
		LogoIcon:  "⚡",
		GitHubURL: "https://github.com/gabrielmiguelok/golivekit",
		ShowBadge: true,
		BadgeText: "100% Go",
		Links: []website.NavLink{
			{Label: "Home", URL: "/"},
			{Label: "Docs", URL: "/docs"},
		},
	}))

	body.WriteString(`<main id="main-content" style="padding-top:5rem">`)
	body.WriteString(`<div data-live-view="docs" class="docs-layout container">`)

	// Sidebar with links (no WebSocket needed)
	body.WriteString(d.renderSidebar())

	// Content (data-slot for efficient updates)
	body.WriteString(`<div class="docs-content" data-slot="content">`)
	body.WriteString(d.renderContent())
	body.WriteString(`</div>`)

	body.WriteString(`</div>`)
	body.WriteString(`</main>`)

	// Footer
	footerOpts := components.DefaultGoliveKitFooter()
	footerOpts.Config.GitHubURL = "https://github.com/gabrielmiguelok/golivekit"
	body.WriteString(components.RenderFooter(footerOpts))

	// GoliveKit client script for WebSocket-powered navigation
	body.WriteString(`<script src="/_live/golivekit.js"></script>`)
	body.WriteString("\n")

	return website.RenderDocument(cfg, renderDocsCSS(), body.String())
}

func (d *DocsComponent) getSectionTitle() string {
	for _, s := range docsSections {
		if s.ID == d.CurrentSection {
			return s.Title
		}
	}
	return "Documentation"
}

func (d *DocsComponent) renderSidebar() string {
	var sb strings.Builder
	sb.WriteString(`<aside class="docs-sidebar" data-slot="sidebar"><nav class="docs-nav">`)
	sb.WriteString(`<h2 class="docs-nav-title">Documentation</h2>`)
	sb.WriteString(`<ul class="docs-nav-list">`)

	for _, section := range docsSections {
		activeClass := ""
		if section.ID == d.CurrentSection {
			activeClass = " docs-nav-item-active"
		}
		// Use lv-click for WebSocket-powered navigation (no page reload)
		sb.WriteString(fmt.Sprintf(`<li><button lv-click="nav" lv-value-section="%s" class="docs-nav-item%s">%s %s</button></li>`,
			section.ID, activeClass, section.Icon, section.Title))
	}

	sb.WriteString(`</ul></nav></aside>`)
	return sb.String()
}

func (d *DocsComponent) renderContent() string {
	// Use cached content (O(1) lookup instead of function call)
	cache := contentCache()
	if content, ok := cache[d.CurrentSection]; ok {
		return content
	}
	return cache["getting-started"]
}

func codeBlock(title, code string) string {
	return fmt.Sprintf(`<div class="code-block"><div class="code-header"><span class="code-title">%s</span><div class="code-dots"><span class="code-dot code-dot-red"></span><span class="code-dot code-dot-yellow"></span><span class="code-dot code-dot-green"></span></div></div><div class="code-content"><code>%s</code></div></div>`, title, code)
}

func contentGettingStarted() string {
	return `<article class="docs-article">
<h1>Getting Started</h1>
<p class="docs-lead">Get up and running with GoliveKit in under a minute.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#requirements">Requirements</a></li>
<li><a href="#installation">Installation</a></li>
<li><a href="#create-project">Create Your First Project</a></li>
<li><a href="#project-structure">Project Structure</a></li>
<li><a href="#hello-world">Hello World Example</a></li>
<li><a href="#next-steps">Next Steps</a></li>
</ul>
</nav>

<section id="requirements" class="docs-section">
<h2>Requirements</h2>
<ul class="docs-list">
<li><strong>Go 1.21+</strong> - GoliveKit uses generics and modern Go features</li>
<li><strong>Modern browser</strong> - WebSocket support required (all modern browsers)</li>
</ul>
</section>

<section id="installation" class="docs-section">
<h2>Installation</h2>
<p>Install the GoliveKit CLI globally:</p>
` + codeBlock("Terminal", `go install github.com/gabrielmiguelok/golivekit/cmd/golive@latest`) + `
<p>Or add the library to an existing project:</p>
` + codeBlock("Terminal", `go get github.com/gabrielmiguelok/golivekit`) + `
</section>

<section id="create-project" class="docs-section">
<h2>Create Your First Project</h2>
` + codeBlock("Terminal", `<span class="token-comment"># Create a new project</span>
golive new myapp

<span class="token-comment"># Enter the project directory</span>
cd myapp

<span class="token-comment"># Start the development server</span>
golive dev

<span class="token-comment"># Open http://localhost:3000 in your browser</span>`) + `
</section>

<section id="project-structure" class="docs-section">
<h2>Project Structure</h2>
` + codeBlock("Project Structure", `myapp/
├── main.go           <span class="token-comment"># Entry point with router setup</span>
├── components/       <span class="token-comment"># LiveView components</span>
│   └── counter.go    <span class="token-comment"># Example counter component</span>
├── templates/        <span class="token-comment"># HTML templates (optional)</span>
├── static/           <span class="token-comment"># Static assets (CSS, JS, images)</span>
└── go.mod            <span class="token-comment"># Go module file</span>`) + `
</section>

<section id="hello-world" class="docs-section">
<h2>Hello World Example</h2>
<p>Here's a minimal GoliveKit application:</p>
` + codeBlock("main.go", `<span class="token-keyword">package</span> main

<span class="token-keyword">import</span> (
    <span class="token-string">"context"</span>
    <span class="token-string">"fmt"</span>
    <span class="token-string">"io"</span>
    <span class="token-string">"net/http"</span>

    <span class="token-string">"github.com/gabrielmiguelok/golivekit/client"</span>
    <span class="token-string">"github.com/gabrielmiguelok/golivekit/pkg/core"</span>
    <span class="token-string">"github.com/gabrielmiguelok/golivekit/pkg/router"</span>
)

<span class="token-keyword">type</span> HelloWorld <span class="token-keyword">struct</span> {
    core.BaseComponent
    Name <span class="token-type">string</span>
}

<span class="token-keyword">func</span> NewHelloWorld() core.Component {
    <span class="token-keyword">return</span> &amp;HelloWorld{Name: <span class="token-string">"World"</span>}
}

<span class="token-keyword">func</span> (h *HelloWorld) Name() <span class="token-type">string</span> { <span class="token-keyword">return</span> <span class="token-string">"hello"</span> }

<span class="token-keyword">func</span> (h *HelloWorld) Mount(ctx context.Context, p core.Params, s core.Session) <span class="token-type">error</span> {
    <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
}

<span class="token-keyword">func</span> (h *HelloWorld) HandleEvent(ctx context.Context, event <span class="token-type">string</span>, payload <span class="token-type">map[string]any</span>) <span class="token-type">error</span> {
    <span class="token-keyword">if</span> event == <span class="token-string">"greet"</span> {
        <span class="token-keyword">if</span> name, ok := payload[<span class="token-string">"name"</span>].(<span class="token-type">string</span>); ok {
            h.Name = name
        }
    }
    <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
}

<span class="token-keyword">func</span> (h *HelloWorld) Render(ctx context.Context) core.Renderer {
    <span class="token-keyword">return</span> core.RendererFunc(<span class="token-keyword">func</span>(ctx context.Context, w io.Writer) <span class="token-type">error</span> {
        fmt.Fprintf(w, <span class="token-string">` + "`" + `&lt;div data-live-view="hello"&gt;
            &lt;h1&gt;Hello, %s!&lt;/h1&gt;
            &lt;input type="text" lv-input="greet" lv-value-name placeholder="Enter name"/&gt;
        &lt;/div&gt;` + "`" + `</span>, h.Name)
        <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
    })
}

<span class="token-keyword">func</span> (h *HelloWorld) Terminate(ctx context.Context, r core.TerminateReason) <span class="token-type">error</span> {
    <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
}

<span class="token-keyword">func</span> main() {
    r := router.New()
    r.Handle(<span class="token-string">"/_live/"</span>, http.StripPrefix(<span class="token-string">"/_live/"</span>, client.Handler()))
    r.Live(<span class="token-string">"/"</span>, NewHelloWorld)
    http.ListenAndServe(<span class="token-string">":3000"</span>, r)
}`) + `
</section>

<section id="next-steps" class="docs-section">
<h2>Next Steps</h2>
<ul class="docs-list">
<li><a href="/docs?section=core-concepts" class="docs-link">Core Concepts</a> - Understand the component lifecycle</li>
<li><a href="/docs?section=events" class="docs-link">Events</a> - Add interactivity to your components</li>
<li><a href="/docs?section=routing" class="docs-link">Routing</a> - Set up routes and middleware</li>
<li><a href="/docs?section=packages" class="docs-link">Package Reference</a> - Explore all 31 packages</li>
</ul>
</section>
</article>`
}

func contentCoreConcepts() string {
	return `<article class="docs-article">
<h1>Core Concepts</h1>
<p class="docs-lead">Understand the fundamental building blocks of GoliveKit.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#component-interface">Component Interface</a></li>
<li><a href="#lifecycle">Component Lifecycle</a></li>
<li><a href="#base-component">BaseComponent</a></li>
<li><a href="#assigns">Assigns (State)</a></li>
<li><a href="#socket">Socket</a></li>
<li><a href="#rendering">Rendering</a></li>
</ul>
</nav>

<section id="component-interface" class="docs-section">
<h2>Component Interface</h2>
<p>Every GoliveKit component implements the <code>core.Component</code> interface:</p>
` + codeBlock("pkg/core/component.go", `<span class="token-keyword">type</span> Component <span class="token-keyword">interface</span> {
    <span class="token-comment">// Name returns the unique component identifier</span>
    Name() <span class="token-type">string</span>

    <span class="token-comment">// Mount is called when the component is first loaded</span>
    Mount(ctx context.Context, params Params, session Session) <span class="token-type">error</span>

    <span class="token-comment">// Render returns the HTML representation</span>
    Render(ctx context.Context) Renderer

    <span class="token-comment">// HandleEvent processes user interactions</span>
    HandleEvent(ctx context.Context, event <span class="token-type">string</span>, payload <span class="token-type">map[string]any</span>) <span class="token-type">error</span>

    <span class="token-comment">// Terminate is called when the component is destroyed</span>
    Terminate(ctx context.Context, reason TerminateReason) <span class="token-type">error</span>
}`) + `
</section>

<section id="lifecycle" class="docs-section">
<h2>Component Lifecycle</h2>
<div class="docs-diagram"><code>┌─────────────────────────────────────────────────────────────┐
│                     HTTP REQUEST                             │
│                          │                                   │
│                          ▼                                   │
│                      Mount()                                 │
│                          │                                   │
│                          ▼                                   │
│                      Render()                                │
│                          │                                   │
│                          ▼                                   │
│                   HTML Response                              │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ WebSocket Connect
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                   INTERACTIVE PHASE                          │
│                          │                                   │
│           ┌──────────────┴──────────────┐                   │
│           │                             │                   │
│           ▼                             │                   │
│     HandleEvent()                       │                   │
│           │                             │                   │
│           ▼                             │                   │
│       Render()                          │                   │
│           │                             │                   │
│           ▼                             │                   │
│     Compute Diff ──────────────────────►│                   │
│           │                             │                   │
│           ▼                             │                   │
│    Send to Client                       │                   │
│           │                             │                   │
│           └─────────────────────────────┘                   │
│                          │                                   │
│                          │ Disconnect                        │
│                          ▼                                   │
│                    Terminate()                               │
└─────────────────────────────────────────────────────────────┘</code></div>
</section>

<section id="base-component" class="docs-section">
<h2>BaseComponent</h2>
<p>Use <code>core.BaseComponent</code> to get common functionality:</p>
` + codeBlock("counter.go", `<span class="token-keyword">type</span> Counter <span class="token-keyword">struct</span> {
    core.BaseComponent  <span class="token-comment">// Provides Assigns(), Socket(), etc.</span>
    Count <span class="token-type">int</span>
}

<span class="token-keyword">func</span> (c *Counter) Mount(ctx context.Context, params core.Params, session core.Session) <span class="token-type">error</span> {
    c.Count = <span class="token-number">0</span>
    <span class="token-comment">// Access assigns</span>
    c.Assigns().Set(<span class="token-string">"initialized"</span>, <span class="token-keyword">true</span>)
    <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
}`) + `
</section>

<section id="assigns" class="docs-section">
<h2>Assigns (State)</h2>
<p><code>Assigns</code> is a thread-safe key-value store for component state:</p>
` + codeBlock("assigns.go", `<span class="token-comment">// Set a value</span>
c.Assigns().Set(<span class="token-string">"user"</span>, user)
c.Assigns().Set(<span class="token-string">"items"</span>, items)
c.Assigns().Set(<span class="token-string">"loading"</span>, <span class="token-keyword">false</span>)

<span class="token-comment">// Get a value</span>
user := c.Assigns().Get(<span class="token-string">"user"</span>)

<span class="token-comment">// Get with type assertion</span>
<span class="token-keyword">if</span> user, ok := c.Assigns().Get(<span class="token-string">"user"</span>).(*User); ok {
    fmt.Println(user.Name)
}

<span class="token-comment">// Check if key exists</span>
<span class="token-keyword">if</span> c.Assigns().Has(<span class="token-string">"user"</span>) {
    <span class="token-comment">// Key exists</span>
}

<span class="token-comment">// Delete a key</span>
c.Assigns().Delete(<span class="token-string">"user"</span>)

<span class="token-comment">// Clone for safe iteration</span>
snapshot := c.Assigns().Clone()`) + `
</section>

<section id="socket" class="docs-section">
<h2>Socket</h2>
<p>The Socket provides access to the WebSocket connection:</p>
` + codeBlock("socket.go", `<span class="token-comment">// Send a message to the client</span>
c.Socket().Send(<span class="token-string">"event_name"</span>, <span class="token-type">map[string]any</span>{
    <span class="token-string">"message"</span>: <span class="token-string">"Hello!"</span>,
})

<span class="token-comment">// Push a redirect</span>
c.Socket().Redirect(<span class="token-string">"/dashboard"</span>)

<span class="token-comment">// Get the socket ID</span>
id := c.Socket().ID()`) + `
</section>

<section id="rendering" class="docs-section">
<h2>Rendering</h2>
<p>The <code>Render</code> method returns HTML using the <code>Renderer</code> interface:</p>
` + codeBlock("render.go", `<span class="token-keyword">func</span> (c *Counter) Render(ctx context.Context) core.Renderer {
    <span class="token-keyword">return</span> core.RendererFunc(<span class="token-keyword">func</span>(ctx context.Context, w io.Writer) <span class="token-type">error</span> {
        <span class="token-comment">// Use data-live-view to identify the component</span>
        <span class="token-comment">// Use data-slot for efficient partial updates</span>
        fmt.Fprintf(w, ` + "`" + `
            &lt;div data-live-view="counter"&gt;
                &lt;h1&gt;Count: &lt;span data-slot="count"&gt;%d&lt;/span&gt;&lt;/h1&gt;
                &lt;button lv-click="increment"&gt;+&lt;/button&gt;
                &lt;button lv-click="decrement"&gt;-&lt;/button&gt;
            &lt;/div&gt;
        ` + "`" + `, c.Count)
        <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
    })
}`) + `
<p><strong>Important attributes:</strong></p>
<ul class="docs-list">
<li><code>data-live-view="name"</code> - Identifies the LiveView component root</li>
<li><code>data-slot="id"</code> - Marks regions for efficient partial updates</li>
</ul>
</section>
</article>`
}

func contentRouting() string {
	return `<article class="docs-article">
<h1>Routing</h1>
<p class="docs-lead">GoliveKit's router supports LiveView components, HTTP handlers, and middleware.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#basic-routes">Basic Routes</a></li>
<li><a href="#liveview-routes">LiveView Routes</a></li>
<li><a href="#route-groups">Route Groups</a></li>
<li><a href="#middleware">Middleware</a></li>
<li><a href="#route-parameters">Route Parameters</a></li>
<li><a href="#static-files">Static Files</a></li>
</ul>
</nav>

<section id="basic-routes" class="docs-section">
<h2>Basic Routes</h2>
` + codeBlock("main.go", `r := router.New()

<span class="token-comment">// HTTP handler</span>
r.Handle(<span class="token-string">"/api/health"</span>, healthHandler)

<span class="token-comment">// HTTP handler function</span>
r.HandleFunc(<span class="token-string">"/api/status"</span>, <span class="token-keyword">func</span>(w http.ResponseWriter, r *http.Request) {
    w.Write([]<span class="token-type">byte</span>(<span class="token-string">"OK"</span>))
})

<span class="token-comment">// Method-specific routes (Go 1.22+)</span>
r.Get(<span class="token-string">"/users"</span>, listUsers)
r.Post(<span class="token-string">"/users"</span>, createUser)
r.Put(<span class="token-string">"/users/{id}"</span>, updateUser)
r.Delete(<span class="token-string">"/users/{id}"</span>, deleteUser)

http.ListenAndServe(<span class="token-string">":3000"</span>, r)`) + `
</section>

<section id="liveview-routes" class="docs-section">
<h2>LiveView Routes</h2>
<p>Use <code>r.Live()</code> to register LiveView components:</p>
` + codeBlock("main.go", `<span class="token-comment">// LiveView handles both HTTP (initial render) and WebSocket (updates)</span>
r.Live(<span class="token-string">"/"</span>, NewHomePage)
r.Live(<span class="token-string">"/dashboard"</span>, NewDashboard)
r.Live(<span class="token-string">"/users/{id}"</span>, NewUserProfile)

<span class="token-comment">// Don't forget the client JS</span>
r.Handle(<span class="token-string">"/_live/"</span>, http.StripPrefix(<span class="token-string">"/_live/"</span>, client.Handler()))`) + `
</section>

<section id="route-groups" class="docs-section">
<h2>Route Groups</h2>
` + codeBlock("routes.go", `<span class="token-comment">// Group routes with a common prefix</span>
r.Group(<span class="token-string">"/api"</span>, <span class="token-keyword">func</span>(g *router.RouteGroup) {
    g.Get(<span class="token-string">"/users"</span>, listUsers)
    g.Post(<span class="token-string">"/users"</span>, createUser)
    g.Get(<span class="token-string">"/users/{id}"</span>, getUser)
})

<span class="token-comment">// Nested groups</span>
r.Group(<span class="token-string">"/admin"</span>, <span class="token-keyword">func</span>(g *router.RouteGroup) {
    g.Use(requireAdmin)  <span class="token-comment">// Group middleware</span>

    g.Live(<span class="token-string">"/dashboard"</span>, NewAdminDashboard)
    g.Live(<span class="token-string">"/users"</span>, NewUserManagement)
})`) + `
</section>

<section id="middleware" class="docs-section">
<h2>Middleware</h2>
<p>Built-in middleware for common tasks:</p>
` + codeBlock("middleware.go", `r := router.New()

<span class="token-comment">// Request ID - adds X-Request-ID header</span>
r.Use(router.RequestID())

<span class="token-comment">// Logger - structured request logging</span>
r.Use(router.Logger())

<span class="token-comment">// Recovery - panic recovery with stack trace</span>
r.Use(router.Recovery())

<span class="token-comment">// CORS - Cross-Origin Resource Sharing</span>
r.Use(router.CORS(router.CORSConfig{
    AllowOrigins: []<span class="token-type">string</span>{<span class="token-string">"https://example.com"</span>},
    AllowMethods: []<span class="token-type">string</span>{<span class="token-string">"GET"</span>, <span class="token-string">"POST"</span>, <span class="token-string">"PUT"</span>, <span class="token-string">"DELETE"</span>},
    AllowHeaders: []<span class="token-type">string</span>{<span class="token-string">"Authorization"</span>, <span class="token-string">"Content-Type"</span>},
}))

<span class="token-comment">// Rate limiting</span>
r.Use(router.RateLimit(<span class="token-number">100</span>)) <span class="token-comment">// 100 requests per minute</span>

<span class="token-comment">// Secure headers (CSP, X-Frame-Options, etc.)</span>
r.Use(router.SecureHeaders())`) + `
</section>

<section id="route-parameters" class="docs-section">
<h2>Route Parameters</h2>
` + codeBlock("params.go", `<span class="token-comment">// Route: /users/{id}/posts/{postID}</span>

<span class="token-keyword">func</span> (c *PostView) Mount(ctx context.Context, params core.Params, session core.Session) <span class="token-type">error</span> {
    <span class="token-comment">// URL parameters</span>
    userID := params[<span class="token-string">"id"</span>]         <span class="token-comment">// "123"</span>
    postID := params[<span class="token-string">"postID"</span>]     <span class="token-comment">// "456"</span>

    <span class="token-comment">// Query parameters (from URL: ?sort=date&amp;page=2)</span>
    sort := params[<span class="token-string">"sort"</span>]        <span class="token-comment">// "date"</span>
    page := params[<span class="token-string">"page"</span>]        <span class="token-comment">// "2"</span>

    <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
}`) + `
</section>

<section id="static-files" class="docs-section">
<h2>Static Files</h2>
` + codeBlock("static.go", `<span class="token-comment">// Serve static files from a directory</span>
r.Static(<span class="token-string">"/static/"</span>, <span class="token-string">"web/static"</span>)

<span class="token-comment">// Or use http.FileServer</span>
r.Handle(<span class="token-string">"/assets/"</span>, http.StripPrefix(<span class="token-string">"/assets/"</span>,
    http.FileServer(http.Dir(<span class="token-string">"public/assets"</span>))))`) + `
</section>
</article>`
}

func contentEvents() string {
	return `<article class="docs-article">
<h1>Events</h1>
<p class="docs-lead">Handle user interactions with lv-* attributes — no JavaScript required.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#click-events">Click Events</a></li>
<li><a href="#form-events">Form Events</a></li>
<li><a href="#input-events">Input Events</a></li>
<li><a href="#passing-values">Passing Values</a></li>
<li><a href="#debounce-throttle">Debounce & Throttle</a></li>
<li><a href="#handling-events">Handling Events in Go</a></li>
<li><a href="#js-hooks">JavaScript Hooks</a></li>
</ul>
</nav>

<section id="click-events" class="docs-section">
<h2>Click Events</h2>
` + codeBlock("template.html", `<span class="token-comment">&lt;!-- Basic click --&gt;</span>
&lt;button <span class="token-keyword">lv-click</span>=<span class="token-string">"increment"</span>&gt;+1&lt;/button&gt;

<span class="token-comment">&lt;!-- With confirmation --&gt;</span>
&lt;button <span class="token-keyword">lv-click</span>=<span class="token-string">"delete"</span> <span class="token-keyword">lv-confirm</span>=<span class="token-string">"Are you sure?"</span>&gt;Delete&lt;/button&gt;

<span class="token-comment">&lt;!-- Disable during processing --&gt;</span>
&lt;button <span class="token-keyword">lv-click</span>=<span class="token-string">"save"</span> <span class="token-keyword">lv-disable-with</span>=<span class="token-string">"Saving..."</span>&gt;Save&lt;/button&gt;`) + `
</section>

<section id="form-events" class="docs-section">
<h2>Form Events</h2>
` + codeBlock("form.html", `<span class="token-comment">&lt;!-- Form submission --&gt;</span>
&lt;form <span class="token-keyword">lv-submit</span>=<span class="token-string">"save_user"</span>&gt;
    &lt;input name=<span class="token-string">"email"</span> type=<span class="token-string">"email"</span> required /&gt;
    &lt;input name=<span class="token-string">"password"</span> type=<span class="token-string">"password"</span> required /&gt;
    &lt;button type=<span class="token-string">"submit"</span>&gt;Sign Up&lt;/button&gt;
&lt;/form&gt;

<span class="token-comment">&lt;!-- Form with live validation --&gt;</span>
&lt;form <span class="token-keyword">lv-submit</span>=<span class="token-string">"create"</span> <span class="token-keyword">lv-change</span>=<span class="token-string">"validate"</span>&gt;
    &lt;input name=<span class="token-string">"username"</span> /&gt;
    &lt;span class="error"&gt;{{ .Errors.username }}&lt;/span&gt;
    &lt;button type=<span class="token-string">"submit"</span>&gt;Create&lt;/button&gt;
&lt;/form&gt;`) + `
</section>

<section id="input-events" class="docs-section">
<h2>Input Events</h2>
` + codeBlock("input.html", `<span class="token-comment">&lt;!-- Change event (fires on blur) --&gt;</span>
&lt;input <span class="token-keyword">lv-change</span>=<span class="token-string">"validate_email"</span> name=<span class="token-string">"email"</span> /&gt;

<span class="token-comment">&lt;!-- Input event (fires on every keystroke) --&gt;</span>
&lt;input <span class="token-keyword">lv-input</span>=<span class="token-string">"search"</span> name=<span class="token-string">"query"</span> placeholder=<span class="token-string">"Search..."</span> /&gt;

<span class="token-comment">&lt;!-- Focus events --&gt;</span>
&lt;input <span class="token-keyword">lv-focus</span>=<span class="token-string">"show_suggestions"</span> <span class="token-keyword">lv-blur</span>=<span class="token-string">"hide_suggestions"</span> /&gt;

<span class="token-comment">&lt;!-- Key events --&gt;</span>
&lt;input <span class="token-keyword">lv-keydown</span>=<span class="token-string">"handle_key"</span> <span class="token-keyword">lv-key</span>=<span class="token-string">"Enter"</span> /&gt;`) + `
</section>

<section id="passing-values" class="docs-section">
<h2>Passing Values</h2>
` + codeBlock("values.html", `<span class="token-comment">&lt;!-- Single value --&gt;</span>
&lt;button <span class="token-keyword">lv-click</span>=<span class="token-string">"select"</span> <span class="token-keyword">lv-value-id</span>=<span class="token-string">"123"</span>&gt;Select Item&lt;/button&gt;

<span class="token-comment">&lt;!-- Multiple values --&gt;</span>
&lt;button <span class="token-keyword">lv-click</span>=<span class="token-string">"action"</span>
        <span class="token-keyword">lv-value-id</span>=<span class="token-string">"123"</span>
        <span class="token-keyword">lv-value-type</span>=<span class="token-string">"delete"</span>
        <span class="token-keyword">lv-value-confirm</span>=<span class="token-string">"true"</span>&gt;
    Delete
&lt;/button&gt;

<span class="token-comment">&lt;!-- Dynamic values from elements --&gt;</span>
&lt;tr lv-click="select_row" lv-value-row-id="{{ .ID }}"&gt;
    &lt;td&gt;{{ .Name }}&lt;/td&gt;
&lt;/tr&gt;`) + `
</section>

<section id="debounce-throttle" class="docs-section">
<h2>Debounce & Throttle</h2>
` + codeBlock("debounce.html", `<span class="token-comment">&lt;!-- Debounce: wait 300ms after last keystroke --&gt;</span>
&lt;input <span class="token-keyword">lv-input</span>=<span class="token-string">"search"</span> <span class="token-keyword">lv-debounce</span>=<span class="token-string">"300"</span> /&gt;

<span class="token-comment">&lt;!-- Throttle: fire at most once per 500ms --&gt;</span>
&lt;div <span class="token-keyword">lv-scroll</span>=<span class="token-string">"load_more"</span> <span class="token-keyword">lv-throttle</span>=<span class="token-string">"500"</span>&gt;&lt;/div&gt;

<span class="token-comment">&lt;!-- Blur debounce (validate on field exit) --&gt;</span>
&lt;input <span class="token-keyword">lv-change</span>=<span class="token-string">"validate"</span> <span class="token-keyword">lv-debounce</span>=<span class="token-string">"blur"</span> /&gt;`) + `
</section>

<section id="handling-events" class="docs-section">
<h2>Handling Events in Go</h2>
` + codeBlock("handler.go", `<span class="token-keyword">func</span> (c *MyComponent) HandleEvent(ctx context.Context, event <span class="token-type">string</span>, payload <span class="token-type">map[string]any</span>) <span class="token-type">error</span> {
    <span class="token-keyword">switch</span> event {
    <span class="token-keyword">case</span> <span class="token-string">"increment"</span>:
        c.Count++

    <span class="token-keyword">case</span> <span class="token-string">"select"</span>:
        <span class="token-comment">// Get value from lv-value-id</span>
        id := payload[<span class="token-string">"id"</span>].(<span class="token-type">string</span>)
        c.SelectedID = id

    <span class="token-keyword">case</span> <span class="token-string">"save_user"</span>:
        <span class="token-comment">// Form data comes in payload</span>
        email := payload[<span class="token-string">"email"</span>].(<span class="token-type">string</span>)
        password := payload[<span class="token-string">"password"</span>].(<span class="token-type">string</span>)
        <span class="token-comment">// Process form...</span>

    <span class="token-keyword">case</span> <span class="token-string">"search"</span>:
        <span class="token-comment">// Input value comes as "value"</span>
        query := payload[<span class="token-string">"value"</span>].(<span class="token-type">string</span>)
        c.Results = c.search(query)

    <span class="token-keyword">case</span> <span class="token-string">"handle_key"</span>:
        <span class="token-comment">// Key info in payload</span>
        key := payload[<span class="token-string">"key"</span>].(<span class="token-type">string</span>)
        <span class="token-keyword">if</span> key == <span class="token-string">"Enter"</span> {
            c.submit()
        }
    }
    <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
}`) + `
</section>

<section id="js-hooks" class="docs-section">
<h2>JavaScript Hooks</h2>
<p>For advanced client-side behavior:</p>
` + codeBlock("hooks.html + hooks.js", `<span class="token-comment">&lt;!-- HTML --&gt;</span>
&lt;div <span class="token-keyword">lv-hook</span>=<span class="token-string">"Chart"</span> data-values=<span class="token-string">"[10,20,30,40]"</span>&gt;&lt;/div&gt;

<span class="token-comment">// JavaScript</span>
GoliveKit.hooks.Chart = {
    mounted() {
        <span class="token-keyword">const</span> data = JSON.parse(<span class="token-keyword">this</span>.el.dataset.values)
        <span class="token-keyword">this</span>.chart = <span class="token-keyword">new</span> Chart(<span class="token-keyword">this</span>.el, { data })
    },
    updated() {
        <span class="token-keyword">const</span> data = JSON.parse(<span class="token-keyword">this</span>.el.dataset.values)
        <span class="token-keyword">this</span>.chart.update(data)
    },
    destroyed() {
        <span class="token-keyword">this</span>.chart.destroy()
    }
}`) + `
</section>
</article>`
}

func contentForms() string {
	return `<article class="docs-article">
<h1>Forms & Validation</h1>
<p class="docs-lead">Ecto-style changesets for form handling and validation.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#creating-forms">Creating Forms</a></li>
<li><a href="#validation">Validation</a></li>
<li><a href="#changesets">Changesets</a></li>
</ul>
</nav>

<section id="creating-forms" class="docs-section">
<h2>Creating Forms</h2>
` + codeBlock("form.go", `<span class="token-keyword">import</span> <span class="token-string">"github.com/gabrielmiguelok/golivekit/pkg/forms"</span>

form := forms.NewForm(<span class="token-string">"user"</span>)

form.AddField(forms.Field{
    Name:     <span class="token-string">"email"</span>,
    Type:     forms.TypeEmail,
    Required: <span class="token-keyword">true</span>,
    Label:    <span class="token-string">"Email Address"</span>,
})

form.AddField(forms.Field{
    Name:      <span class="token-string">"password"</span>,
    Type:      forms.TypePassword,
    Required:  <span class="token-keyword">true</span>,
    MinLength: <span class="token-number">8</span>,
})`) + `
</section>

<section id="validation" class="docs-section">
<h2>Validation</h2>
` + codeBlock("validators.go", `forms.Required()           <span class="token-comment">// Field must have a value</span>
forms.MinLength(<span class="token-number">8</span>)         <span class="token-comment">// Minimum string length</span>
forms.MaxLength(<span class="token-number">100</span>)       <span class="token-comment">// Maximum string length</span>
forms.Email()              <span class="token-comment">// Valid email format</span>
forms.Pattern(<span class="token-string">"^[a-z]+$"</span>) <span class="token-comment">// Regex pattern</span>
forms.Min(<span class="token-number">0</span>)               <span class="token-comment">// Minimum number value</span>
forms.Max(<span class="token-number">100</span>)             <span class="token-comment">// Maximum number value</span>`) + `
</section>

<section id="changesets" class="docs-section">
<h2>Changesets</h2>
` + codeBlock("changeset.go", `changeset := form.Changeset(payload)

<span class="token-keyword">if</span> changeset.Valid() {
    email := changeset.Get(<span class="token-string">"email"</span>).(<span class="token-type">string</span>)
    <span class="token-comment">// Save to database...</span>
} <span class="token-keyword">else</span> {
    errors := changeset.Errors()
}`) + `
</section>
</article>`
}

func contentRealtime() string {
	return `<article class="docs-article">
<h1>Real-time Updates</h1>
<p class="docs-lead">WebSocket-powered DOM updates with minimal data transfer.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#how-it-works">How It Works</a></li>
<li><a href="#slots">Slots for Efficient Updates</a></li>
<li><a href="#pubsub">PubSub</a></li>
<li><a href="#presence">Presence Tracking</a></li>
</ul>
</nav>

<section id="how-it-works" class="docs-section">
<h2>How It Works</h2>
<ol class="docs-list docs-list-numbered">
<li><strong>Initial render</strong>: Full HTML sent to browser</li>
<li><strong>User interaction</strong>: Event sent via WebSocket</li>
<li><strong>Server processes</strong>: HandleEvent() called</li>
<li><strong>Re-render</strong>: Render() produces new HTML</li>
<li><strong>Diff</strong>: Engine computes minimal changes</li>
<li><strong>Update</strong>: Only changed parts sent to browser</li>
</ol>
</section>

<section id="slots" class="docs-section">
<h2>Slots for Efficient Updates</h2>
` + codeBlock("counter.html", `&lt;div data-live-view=<span class="token-string">"counter"</span>&gt;
    &lt;h1&gt;Counter&lt;/h1&gt;
    &lt;span <span class="token-keyword">data-slot</span>=<span class="token-string">"count"</span>&gt;0&lt;/span&gt;
    &lt;button lv-click=<span class="token-string">"increment"</span>&gt;+&lt;/button&gt;
&lt;/div&gt;`) + `
<p>Typical diff size: <strong>100-300 bytes</strong> vs full HTML.</p>
</section>

<section id="pubsub" class="docs-section">
<h2>PubSub</h2>
` + codeBlock("pubsub.go", `<span class="token-keyword">import</span> <span class="token-string">"github.com/gabrielmiguelok/golivekit/pkg/pubsub"</span>

<span class="token-comment">// Subscribe</span>
pubsub.Subscribe(<span class="token-string">"chat:room:123"</span>, c)

<span class="token-comment">// Broadcast</span>
pubsub.Broadcast(<span class="token-string">"chat:room:123"</span>, ChatMessage{Text: msg})

<span class="token-comment">// Handle in HandleInfo</span>
<span class="token-keyword">func</span> (c *Chat) HandleInfo(ctx context.Context, msg <span class="token-keyword">any</span>) <span class="token-type">error</span> {
    <span class="token-keyword">if</span> chatMsg, ok := msg.(ChatMessage); ok {
        c.Messages = <span class="token-function">append</span>(c.Messages, chatMsg)
    }
    <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
}`) + `
</section>

<section id="presence" class="docs-section">
<h2>Presence Tracking</h2>
` + codeBlock("presence.go", `<span class="token-keyword">import</span> <span class="token-string">"github.com/gabrielmiguelok/golivekit/pkg/presence"</span>

presence.Track(<span class="token-string">"room:123"</span>, user.ID, <span class="token-type">map[string]any</span>{<span class="token-string">"name"</span>: user.Name})
users := presence.List(<span class="token-string">"room:123"</span>)
presence.Untrack(<span class="token-string">"room:123"</span>, user.ID)`) + `
</section>
</article>`
}

func contentState() string {
	return `<article class="docs-article">
<h1>State Management</h1>
<p class="docs-lead">Managing component state, sessions, and persistent storage.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#assigns">Assigns</a></li>
<li><a href="#session">Session</a></li>
<li><a href="#state-store">State Store</a></li>
</ul>
</nav>

<section id="assigns" class="docs-section">
<h2>Assigns (Component State)</h2>
` + codeBlock("assigns.go", `c.Assigns().Set(<span class="token-string">"user"</span>, user)
c.Assigns().Get(<span class="token-string">"user"</span>)
c.Assigns().Has(<span class="token-string">"user"</span>)
c.Assigns().Delete(<span class="token-string">"user"</span>)
c.Assigns().Clone()`) + `
</section>

<section id="session" class="docs-section">
<h2>Session</h2>
` + codeBlock("session.go", `<span class="token-keyword">func</span> (c *MyComponent) Mount(ctx context.Context, params core.Params, session core.Session) <span class="token-type">error</span> {
    userID := session.Get(<span class="token-string">"user_id"</span>)
    <span class="token-keyword">if</span> !session.Has(<span class="token-string">"user_id"</span>) {
        <span class="token-keyword">return</span> core.Redirect(<span class="token-string">"/login"</span>)
    }
    <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
}`) + `
</section>

<section id="state-store" class="docs-section">
<h2>State Store</h2>
` + codeBlock("store.go", `<span class="token-keyword">import</span> <span class="token-string">"github.com/gabrielmiguelok/golivekit/pkg/state"</span>

store := state.NewMemoryStore()
<span class="token-comment">// or</span>
store := state.NewRedisStore(<span class="token-string">"localhost:6379"</span>)

store.Set(ctx, <span class="token-string">"key"</span>, value, <span class="token-number">1</span>*time.Hour)
value, _ := store.Get(ctx, <span class="token-string">"key"</span>)
store.Delete(ctx, <span class="token-string">"key"</span>)`) + `
</section>
</article>`
}

func contentSecurity() string {
	return `<article class="docs-article">
<h1>Security</h1>
<p class="docs-lead">Built-in security features for production applications.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#csrf">CSRF Protection</a></li>
<li><a href="#xss">XSS Prevention</a></li>
<li><a href="#auth">Authentication</a></li>
<li><a href="#rate-limiting">Rate Limiting</a></li>
</ul>
</nav>

<section id="csrf" class="docs-section">
<h2>CSRF Protection</h2>
` + codeBlock("csrf.go", `r.Use(security.CSRF(security.CSRFConfig{
    Secret:     []<span class="token-type">byte</span>(<span class="token-string">"your-secret-key"</span>),
    CookieName: <span class="token-string">"_csrf"</span>,
}))`) + `
</section>

<section id="xss" class="docs-section">
<h2>XSS Prevention</h2>
` + codeBlock("xss.go", `clean := security.Sanitize(userInput)
escaped := security.EscapeHTML(content)`) + `
</section>

<section id="auth" class="docs-section">
<h2>Authentication</h2>
` + codeBlock("auth.go", `r.Group(<span class="token-string">"/admin"</span>, <span class="token-keyword">func</span>(g *router.RouteGroup) {
    g.Use(security.RequireAuth())
    g.Live(<span class="token-string">"/dashboard"</span>, NewDashboard)
})

r.Group(<span class="token-string">"/super"</span>, <span class="token-keyword">func</span>(g *router.RouteGroup) {
    g.Use(security.RequireRoles(<span class="token-string">"admin"</span>))
    g.Live(<span class="token-string">"/settings"</span>, NewSettings)
})`) + `
</section>

<section id="rate-limiting" class="docs-section">
<h2>Rate Limiting</h2>
` + codeBlock("ratelimit.go", `r.Use(limits.RateLimit(limits.RateLimitConfig{
    Rate:    <span class="token-number">100</span>,
    Period:  time.Minute,
    KeyFunc: limits.ByIP,
}))`) + `
</section>
</article>`
}

func contentPerformance() string {
	return `<article class="docs-article">
<h1>Performance</h1>
<p class="docs-lead">GoliveKit achieves React-like responsiveness with server-side rendering through a three-layer optimization strategy.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#optimistic-ui">Optimistic UI</a></li>
<li><a href="#css-feedback">CSS Feedback Classes</a></li>
<li><a href="#hash-comparison">Hash-based Diff</a></li>
<li><a href="#server-optimizations">Server Optimizations</a></li>
<li><a href="#configuration">Configuration</a></li>
<li><a href="#best-practices">Best Practices</a></li>
</ul>
</nav>

<section id="optimistic-ui" class="docs-section">
<h2>Optimistic UI</h2>
<p>GoliveKit updates the UI <strong>instantly</strong> before the server responds. The server confirms (or rolls back) asynchronously.</p>
<div class="docs-diagram"><code>┌─────────────────────────────────────────────────────────────┐
│              THREE-LAYER SPEED                               │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│   CLICK ──► OPTIMISTIC UI ──► 0ms perceived                 │
│             (instant feedback)                               │
│                    │                                         │
│                    ▼                                         │
│            HASH DIFF ──► &lt;5ms                               │
│         (O(1) comparison)                                    │
│                    │                                         │
│                    ▼                                         │
│          SERVER ──► &lt;10ms total                             │
│       (buffer pools, per-socket)                             │
│                                                              │
└─────────────────────────────────────────────────────────────┘</code></div>
<p><strong>How it works:</strong></p>
<ol class="docs-list docs-list-numbered">
<li>User clicks a button (e.g., navigation tab)</li>
<li>CSS class <code>lv-pending</code> is added immediately (0ms)</li>
<li>Event is sent to server via WebSocket</li>
<li>Server processes and sends diff back (&lt;10ms)</li>
<li>DOM is patched with new content</li>
</ol>
</section>

<section id="css-feedback" class="docs-section">
<h2>CSS Feedback Classes</h2>
<p>GoliveKit automatically applies CSS classes during interactions:</p>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Class</th><th>When Applied</th><th>Use Case</th></tr></thead>
<tbody>
<tr><td><code>lv-pending</code></td><td>Event sent, waiting for response</td><td>Loading spinners, opacity changes</td></tr>
<tr><td><code>:active</code></td><td>Mouse/touch down</td><td>Button press feedback</td></tr>
</tbody>
</table>
</div>
` + codeBlock("CSS Example", `.docs-nav-item {
    transition: all 0.15s ease;
}

<span class="token-comment">/* Instant feedback on click */</span>
.docs-nav-item:active {
    transform: scale(0.98);
    opacity: 0.8;
}

<span class="token-comment">/* Pending state while server responds */</span>
.docs-nav-item.lv-pending {
    opacity: 0.7;
    pointer-events: none;
}`) + `
</section>

<section id="hash-comparison" class="docs-section">
<h2>Hash-based Diff</h2>
<p>Instead of comparing HTML strings character-by-character, GoliveKit uses hash-based slot comparison:</p>
` + codeBlock("How it works", `<span class="token-comment">// Traditional: O(n) string comparison</span>
oldHTML == newHTML  <span class="token-comment">// Slow for large strings</span>

<span class="token-comment">// GoliveKit: O(1) hash comparison</span>
oldSlot.Hash == newSlot.Hash  <span class="token-comment">// Instant</span>`) + `
<p><strong>Benefits:</strong></p>
<ul class="docs-list">
<li>O(1) comparison regardless of content size</li>
<li>Only changed slots are re-rendered</li>
<li>Typical diff size: 100-300 bytes</li>
</ul>
</section>

<section id="server-optimizations" class="docs-section">
<h2>Server Optimizations</h2>
<p>GoliveKit uses several techniques to minimize server-side latency:</p>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Technique</th><th>Description</th><th>Impact</th></tr></thead>
<tbody>
<tr><td><strong>Buffer Pools</strong></td><td>Reuse byte buffers instead of allocating</td><td>~30% less GC pressure</td></tr>
<tr><td><strong>Per-socket State</strong></td><td>No global mutex for socket operations</td><td>Better concurrency</td></tr>
<tr><td><strong>Single-pass Extract</strong></td><td>Extract slots in O(n) single pass</td><td>Faster rendering</td></tr>
<tr><td><strong>Cached Content</strong></td><td>Pre-render static content on startup</td><td>O(1) content lookup</td></tr>
</tbody>
</table>
</div>
</section>

<section id="configuration" class="docs-section">
<h2>Configuration</h2>
<p>Fine-tune performance behavior in the JavaScript client:</p>
` + codeBlock("golivekit.js", `GoliveKit.config({
    <span class="token-comment">// Debounce rapid clicks (ms)</span>
    eventDebounce: <span class="token-number">16</span>,

    <span class="token-comment">// Enable/disable optimistic UI</span>
    optimisticUpdates: <span class="token-keyword">true</span>,

    <span class="token-comment">// Auto scroll-to-top on navigation</span>
    scrollToTop: <span class="token-keyword">true</span>,
})`) + `
</section>

<section id="best-practices" class="docs-section">
<h2>Best Practices</h2>
<ul class="docs-list">
<li><strong>Use data-slot</strong> for dynamic content to enable efficient partial updates</li>
<li><strong>Keep event handlers fast</strong> - heavy processing should be async</li>
<li><strong>Add CSS transitions</strong> to all interactive elements for perceived speed</li>
<li><strong>Pre-render static content</strong> using <code>sync.OnceValue</code> pattern</li>
<li><strong>Avoid full page re-renders</strong> - update only what changed</li>
</ul>
` + codeBlock("Efficient Component", `<span class="token-comment">// Pre-compute static content once</span>
<span class="token-keyword">var</span> staticContent = sync.OnceValue(<span class="token-keyword">func</span>() <span class="token-type">string</span> {
    <span class="token-keyword">return</span> renderExpensiveContent()
})

<span class="token-keyword">func</span> (c *MyComponent) Render(ctx context.Context) core.Renderer {
    <span class="token-keyword">return</span> core.RendererFunc(<span class="token-keyword">func</span>(ctx context.Context, w io.Writer) <span class="token-type">error</span> {
        fmt.Fprintf(w, ` + "`" + `
            &lt;div data-live-view="my-component"&gt;
                %s
                &lt;span data-slot="dynamic"&gt;%s&lt;/span&gt;
            &lt;/div&gt;
        ` + "`" + `, staticContent(), c.DynamicValue)
        <span class="token-keyword">return</span> <span class="token-keyword">nil</span>
    })
}`) + `
</section>
</article>`
}

func contentCLI() string {
	return `<article class="docs-article">
<h1>CLI Reference</h1>
<p class="docs-lead">Command-line tools for rapid development.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#install">Installation</a></li>
<li><a href="#new">golive new</a></li>
<li><a href="#dev">golive dev</a></li>
<li><a href="#build">golive build</a></li>
<li><a href="#generate">golive generate</a></li>
</ul>
</nav>

<section id="install" class="docs-section">
<h2>Installation</h2>
` + codeBlock("Terminal", `go install github.com/gabrielmiguelok/golivekit/cmd/golive@latest`) + `
</section>

<section id="new" class="docs-section">
<h2>golive new</h2>
` + codeBlock("Terminal", `golive new myapp
<span class="token-comment"># Creates project structure with example counter</span>`) + `
</section>

<section id="dev" class="docs-section">
<h2>golive dev</h2>
` + codeBlock("Terminal", `golive dev       <span class="token-comment"># Start with hot reload on :3000</span>
golive dev -p 8080  <span class="token-comment"># Custom port</span>`) + `
</section>

<section id="build" class="docs-section">
<h2>golive build</h2>
` + codeBlock("Terminal", `golive build
<span class="token-comment"># Outputs optimized binary to dist/</span>`) + `
</section>

<section id="generate" class="docs-section">
<h2>golive generate</h2>
` + codeBlock("Terminal", `golive generate component Counter
golive generate live Dashboard
golive generate scaffold User`) + `
</section>

<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Command</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>golive new &lt;name&gt;</code></td><td>Create new project</td></tr>
<tr><td><code>golive dev</code></td><td>Start dev server with hot reload</td></tr>
<tr><td><code>golive build</code></td><td>Build production binary</td></tr>
<tr><td><code>golive generate component</code></td><td>Generate component</td></tr>
<tr><td><code>golive generate live</code></td><td>Generate LiveView component</td></tr>
<tr><td><code>golive generate scaffold</code></td><td>Generate component + handler</td></tr>
</tbody>
</table>
</div>
</article>`
}

func contentPackages() string {
	return `<article class="docs-article">
<h1>Package Reference</h1>
<p class="docs-lead">All 31 packages in GoliveKit's modular architecture.</p>

<nav class="docs-toc">
<h3>On this page</h3>
<ul>
<li><a href="#core-pkg">Core</a></li>
<li><a href="#rendering-pkg">Rendering</a></li>
<li><a href="#forms-pkg">Forms</a></li>
<li><a href="#realtime-pkg">Real-time</a></li>
<li><a href="#state-pkg">State</a></li>
<li><a href="#security-pkg">Security</a></li>
<li><a href="#extensibility-pkg">Extensibility</a></li>
<li><a href="#observability-pkg">Observability</a></li>
<li><a href="#infrastructure-pkg">Infrastructure</a></li>
<li><a href="#utilities-pkg">Utilities</a></li>
</ul>
</nav>

<section id="core-pkg" class="docs-section">
<h2>Core</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>core</code></td><td>Component interface, Socket, Assigns, Session, Params</td></tr>
<tr><td><code>router</code></td><td>HTTP routing, WebSocket upgrade, middleware</td></tr>
<tr><td><code>transport</code></td><td>WebSocket, SSE, Long-polling transports</td></tr>
<tr><td><code>protocol</code></td><td>Wire protocol, Phoenix-compatible codec</td></tr>
</tbody>
</table>
</div>
</section>

<section id="rendering-pkg" class="docs-section">
<h2>Rendering</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>diff</code></td><td>Hybrid HTML diff engine with slot caching</td></tr>
<tr><td><code>streaming</code></td><td>SSR with suspense boundaries</td></tr>
<tr><td><code>islands</code></td><td>Partial hydration (load, visible, idle, interaction, media)</td></tr>
</tbody>
</table>
</div>
</section>

<section id="forms-pkg" class="docs-section">
<h2>Forms & Validation</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>forms</code></td><td>Ecto-style changesets, validation</td></tr>
<tr><td><code>uploads</code></td><td>File uploads (multipart, chunked, S3/GCS)</td></tr>
</tbody>
</table>
</div>
</section>

<section id="realtime-pkg" class="docs-section">
<h2>Real-time</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>pubsub</code></td><td>Real-time pub/sub messaging</td></tr>
<tr><td><code>presence</code></td><td>User presence tracking</td></tr>
</tbody>
</table>
</div>
</section>

<section id="state-pkg" class="docs-section">
<h2>State & Storage</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>state</code></td><td>State persistence (Memory, Redis)</td></tr>
<tr><td><code>pool</code></td><td>Memory pooling, RingBuffer</td></tr>
</tbody>
</table>
</div>
</section>

<section id="security-pkg" class="docs-section">
<h2>Security</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>security</code></td><td>Auth, CSRF, XSS sanitization</td></tr>
<tr><td><code>limits</code></td><td>Rate limiting, backpressure</td></tr>
</tbody>
</table>
</div>
</section>

<section id="extensibility-pkg" class="docs-section">
<h2>Extensibility</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>plugin</code></td><td>Plugin system with 15+ hooks</td></tr>
<tr><td><code>js</code></td><td>JavaScript commands</td></tr>
</tbody>
</table>
</div>
</section>

<section id="observability-pkg" class="docs-section">
<h2>Observability</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>metrics</code></td><td>Prometheus-compatible metrics</td></tr>
<tr><td><code>logging</code></td><td>Structured logging (slog)</td></tr>
<tr><td><code>tracing</code></td><td>OpenTelemetry integration</td></tr>
</tbody>
</table>
</div>
</section>

<section id="infrastructure-pkg" class="docs-section">
<h2>Infrastructure</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>retry</code></td><td>Exponential backoff with jitter</td></tr>
<tr><td><code>shutdown</code></td><td>Graceful shutdown handler</td></tr>
</tbody>
</table>
</div>
</section>

<section id="utilities-pkg" class="docs-section">
<h2>Utilities</h2>
<div class="docs-table-wrapper">
<table class="docs-table">
<thead><tr><th>Package</th><th>Description</th></tr></thead>
<tbody>
<tr><td><code>i18n</code></td><td>Internationalization</td></tr>
<tr><td><code>a11y</code></td><td>Accessibility helpers</td></tr>
<tr><td><code>testing</code></td><td>Component testing utilities</td></tr>
</tbody>
</table>
</div>
</section>

<section class="docs-section">
<h2>External Dependencies</h2>
<p>GoliveKit uses only <strong>3 external dependencies</strong>:</p>
<ul class="docs-list">
<li><code>github.com/gorilla/websocket</code> - WebSocket</li>
<li><code>github.com/google/uuid</code> - UUID generation</li>
<li><code>github.com/vmihailenco/msgpack/v5</code> - MsgPack codec</li>
</ul>
</section>
</article>`
}

func renderDocsCSS() string {
	return `
.docs-layout{display:grid;grid-template-columns:1fr;gap:2rem;padding:2rem 0}
@media(min-width:768px){.docs-layout{grid-template-columns:260px 1fr}}
.docs-sidebar{position:sticky;top:5rem;height:fit-content;max-height:calc(100vh - 6rem);overflow-y:auto}
.docs-nav-title{font-size:0.8rem;font-weight:700;text-transform:uppercase;letter-spacing:0.05em;color:var(--color-textMuted);margin-bottom:1rem;padding-left:0.75rem}
.docs-nav-list{display:flex;flex-direction:column;gap:0.25rem;list-style:none;padding:0;margin:0}
.docs-nav-item{display:flex;align-items:center;gap:0.5rem;width:100%;padding:0.6rem 0.75rem;border-radius:0.5rem;font-size:0.875rem;color:var(--color-textMuted);background:transparent;text-decoration:none;transition:all 0.15s ease;border:none;cursor:pointer;font-family:inherit;text-align:left}
.docs-nav-item:hover{color:var(--color-text);background:var(--color-bgAlt)}
.docs-nav-item-active{color:var(--color-primary);background:rgba(139,92,246,0.1);font-weight:600}
.docs-content{min-width:0}
.docs-article{max-width:800px}
.docs-article h1{font-size:2.25rem;margin-bottom:0.75rem}
.docs-article h2{font-size:1.4rem;margin-top:2rem;margin-bottom:0.75rem;padding-bottom:0.5rem;border-bottom:1px solid var(--color-border)}
.docs-lead{font-size:1.1rem;color:var(--color-textMuted);margin-bottom:1.5rem}
.docs-section{margin-bottom:1.5rem}
.docs-section p{margin-bottom:0.75rem;line-height:1.6}
.docs-list{padding-left:1.25rem;margin-bottom:0.75rem;list-style:disc}
.docs-list li{margin-bottom:0.4rem;color:var(--color-textMuted)}
.docs-list-numbered{list-style:decimal}
.docs-link{color:var(--color-primary);text-decoration:underline;text-underline-offset:2px}
.docs-link:hover{color:var(--color-primaryBright)}
.docs-diagram{background:var(--color-bgCode);border-radius:0.75rem;padding:1rem;overflow-x:auto;margin:0.75rem 0}
.docs-diagram code{font-family:var(--font-mono);font-size:0.75rem;color:var(--color-text);white-space:pre;line-height:1.4}
.docs-table-wrapper{overflow-x:auto;margin:0.75rem 0}
.docs-table{width:100%;border-collapse:collapse}
.docs-table th,.docs-table td{padding:0.6rem 0.75rem;text-align:left;border-bottom:1px solid var(--color-border)}
.docs-table th{font-weight:600;color:var(--color-text);background:var(--color-bgAlt);font-size:0.875rem}
.docs-table td{color:var(--color-textMuted);font-size:0.875rem}
.docs-table code{background:var(--color-bgCode);padding:0.15rem 0.35rem;border-radius:0.25rem;font-size:0.8rem}
.docs-toc{background:var(--color-bgAlt);border-radius:0.75rem;padding:1rem 1.25rem;margin-bottom:1.5rem;border:1px solid var(--color-border)}
.docs-toc h3{font-size:0.8rem;font-weight:700;text-transform:uppercase;letter-spacing:0.05em;color:var(--color-textMuted);margin-bottom:0.5rem}
.docs-toc ul{list-style:none;padding:0;margin:0}
.docs-toc li{margin-bottom:0.25rem}
.docs-toc a{color:var(--color-primary);text-decoration:none;font-size:0.875rem}
.docs-toc a:hover{text-decoration:underline}
@media(max-width:767px){
.docs-sidebar{position:relative;top:0;max-height:none;padding:0.75rem;background:var(--color-bgAlt);border-radius:0.75rem;margin-bottom:1rem}
.docs-nav-list{flex-direction:row;flex-wrap:wrap;gap:0.4rem}
.docs-nav-item{padding:0.4rem 0.6rem;font-size:0.75rem}
.docs-nav-title{display:none}
}
`
}

# GoliveKit

**GoliveKit** is a LiveView framework for Go that enables building interactive, real-time web applications without writing JavaScript.

[![Go Reference](https://pkg.go.dev/badge/github.com/gabrielmiguelok/golivekit.svg)](https://pkg.go.dev/github.com/gabrielmiguelok/golivekit)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Live Demo

**Try it now**: [golivekit.cloud](https://golivekit.cloud)

**Documentation**: [golivekit.cloud/docs](https://golivekit.cloud/docs)

## Mission

GoliveKit exists so that any developer can create interactive, real-time web applications without having to write JavaScript.

Imagine building an application where users see instant changes—a chat, a live dashboard, a collaborative editor—writing only Go. Without worrying about syncing the server with the browser, without duplicating logic, without complex frontend frameworks. Simply describe how your app looks, and GoliveKit handles getting every change to the user in milliseconds.

## Features

- **Server-Rendered, Client-Interactive**: Write your UI in Go using templ templates
- **Hybrid Diff Engine**: Compile-time AST analysis + runtime change tracking for minimal updates
- **Islands Architecture**: Partial hydration with 5 strategies (load, visible, idle, interaction, media)
- **Streaming SSR**: Progressive rendering with suspense boundaries
- **Plugin System**: 15+ hook points for extensibility
- **Multiple Transports**: WebSocket (primary), SSE (fallback), long-polling (legacy)
- **Testing Utilities**: Test LiveView components without a browser

## ⚡ Instant UI Response

GoliveKit delivers React-like responsiveness with server-side rendering through a three-layer optimization:

| Layer | Technique | Result |
|-------|-----------|--------|
| **Client** | Optimistic UI + CSS feedback | 0ms perceived |
| **Diff** | Hash-based O(1) comparison | <5ms processing |
| **Server** | Buffer pools + per-socket state | <10ms total |

**Built-in features:**
- **Optimistic Updates**: UI changes instantly, server confirms asynchronously
- **CSS Feedback**: Automatic `:active` states and transition effects on all interactive elements
- **Smart Debouncing**: Prevents rapid duplicate clicks (16ms default)
- **Scroll Management**: Automatic scroll-to-top on navigation events

## Installation

```bash
go get github.com/gabrielmiguelok/golivekit
```

## CLI

```bash
# Install CLI
go install github.com/gabrielmiguelok/golivekit/cmd/golive@latest

# Create new project
golive new myapp

# Start development server (with hot reload)
cd myapp && golive dev

# Build for production
golive build

# Generate components
golive generate component Counter
golive generate live Dashboard
golive generate scaffold User
```

## Quick Start

### 1. Create a Component

```go
package components

import (
    "context"
    "github.com/gabrielmiguelok/golivekit/pkg/core"
)

type Counter struct {
    core.BaseComponent
    Count int
}

func NewCounter() core.Component {
    return &Counter{}
}

func (c *Counter) Name() string {
    return "counter"
}

func (c *Counter) Mount(ctx context.Context, params core.Params, session core.Session) error {
    c.Count = 0
    c.Assigns().Set("count", c.Count)
    return nil
}

func (c *Counter) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
    switch event {
    case "increment":
        c.Count++
    case "decrement":
        c.Count--
    }
    c.Assigns().Set("count", c.Count)
    return nil
}

func (c *Counter) Render(ctx context.Context) core.Renderer {
    return CounterTemplate(c.Count)
}
```

### 2. Create a Template (using templ)

```templ
// counter.templ
templ CounterTemplate(count int) {
    <div data-live-view="counter">
        <h1>Counter: <span data-slot="count">{ fmt.Sprint(count) }</span></h1>
        <button lv-click="decrement">-</button>
        <button lv-click="increment">+</button>
    </div>
}
```

### 3. Set Up the Server

```go
package main

import (
    "net/http"
    "github.com/gabrielmiguelok/golivekit/pkg/core"
    "github.com/gabrielmiguelok/golivekit/pkg/transport"
    "github.com/gabrielmiguelok/golivekit/client"
    "yourapp/components"
)

func main() {
    // Register components
    registry := core.NewComponentRegistry()
    registry.Register("counter", components.NewCounter)

    // WebSocket handler
    wsHandler := transport.NewWebSocketHandler(nil, func(t *transport.WebSocketTransport) {
        // Handle new connections
    })

    // Routes
    http.Handle("/_live/websocket", wsHandler)
    http.Handle("/_live/", client.Handler())
    http.HandleFunc("/", handleIndex)

    http.ListenAndServe(":3000", nil)
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    APPLICATION LAYER                         │
│  Components │ Pages │ Layouts │ Islands                      │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                      PLUGIN SYSTEM                           │
│  Hook Registry (15+ points) │ Auth │ Storage │ Analytics     │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                       CORE ENGINE                            │
│  Diff Engine │ State Manager │ Streaming Engine              │
│  (Hybrid)      (Change Track)   (Suspense)                   │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────▼───────────────────────────────┐
│                    TRANSPORT LAYER                           │
│  WebSocket (default) │ SSE (fallback) │ Long-polling         │
└─────────────────────────────────────────────────────────────┘
```

## Islands Architecture

GoliveKit supports partial hydration using islands:

```go
// Create an island with specific hydration strategy
island := islands.NewIsland("my-widget", "Widget",
    islands.WithHydration(islands.HydrateOnVisible),
    islands.WithPriority(islands.PriorityHigh),
    islands.WithProps(map[string]any{"title": "Hello"}),
)
```

Hydration strategies:
- `load`: Hydrate immediately on page load
- `visible`: Hydrate when visible (IntersectionObserver)
- `idle`: Hydrate when browser is idle (requestIdleCallback)
- `interaction`: Hydrate on first interaction
- `none`: Never hydrate (static content)

## Plugin System

Extend GoliveKit with custom plugins:

```go
type MyPlugin struct {
    plugin.BasePlugin
}

func (p *MyPlugin) Init(app *plugin.App) error {
    // Register hooks
    app.Hooks().Register(plugin.HookAfterMount, "myplugin", p.onMount)
    return nil
}
```

Available hooks:
- `beforeMount`, `afterMount`
- `beforeRender`, `afterRender`
- `beforeEvent`, `afterEvent`
- `onConnect`, `onDisconnect`, `onReconnect`
- `beforeAssign`, `afterAssign`
- `onError`, `onPanic`

## Testing

Test components without a browser:

```go
func TestCounter(t *testing.T) {
    testing.Mount(t, NewCounter()).
        AssertText("0").
        Click("[lv-click=increment]").
        AssertText("1").
        AssertAssign("count", 1)
}
```

## Performance Targets

| Metric | Target |
|--------|--------|
| Perceived latency | <10ms |
| Connections/node | 50,000 |
| Memory/connection | 20-50KB |
| Event latency p50 | 3ms |
| Render time p50 | 0.5ms |
| Diff size typical | 100-300 bytes |

## Examples

| Example | Description |
|---------|-------------|
| `counter/` | Basic state management with events |
| `chat/` | Real-time chat with PubSub |
| `todo/` | Forms, changesets, validation |
| `demo/` | 7 advanced demos (dashboard, game, editor, uploads, etc.) |

Run any example:
```bash
cd examples/counter && go run main.go
# Open http://localhost:3000
```

## Packages (29 active)

| Package | Description |
|---------|-------------|
| `core` | Component interface, Socket, Assigns, Session |
| `router` | HTTP routing, WebSocket upgrade, middleware |
| `transport` | WebSocket, SSE, Long-polling transports |
| `diff` | Hybrid HTML diff engine with slot caching |
| `protocol` | Wire protocol, Phoenix-compatible codec |
| `forms` | Ecto-style changesets and validation |
| `pubsub` | Real-time pub/sub messaging |
| `presence` | User presence tracking |
| `islands` | Partial hydration (5 strategies) |
| `streaming` | SSR with suspense boundaries |
| `plugin` | Plugin system with 15+ hooks |
| `security` | Auth, CSRF, XSS prevention, rate limiting |
| `state` | State persistence (Memory, Redis) |
| `pool` | Memory pooling, RingBuffer |
| `retry` | Exponential backoff with jitter |
| `shutdown` | Graceful shutdown handler |
| `limits` | Rate limiting, backpressure |
| `metrics` | Prometheus-compatible metrics |
| `logging` | Structured logging (slog) |
| `tracing` | OpenTelemetry integration |
| `i18n` | Internationalization |
| `uploads` | File uploads (multipart, chunked, S3/GCS) |
| `a11y` | Accessibility helpers |
| `js` | JavaScript commands |
| `testing` | Component testing utilities |
| `audit` | Security audit logging (12 event types) |
| `health` | Health check endpoints (Kubernetes-compatible) |
| `observability` | Prometheus-style metrics system |
| `recovery` | State recovery for reconnections |

## Project Structure

```
golivekit/
├── cmd/golive/        # CLI tool
├── pkg/               # All 29 packages
├── client/            # JavaScript client
├── examples/          # Example applications
└── internal/          # Internal website templates
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

Inspired by:
- [Phoenix LiveView](https://github.com/phoenixframework/phoenix_live_view)
- [Hotwire](https://hotwired.dev/)
- [Astro Islands](https://docs.astro.build/en/concepts/islands/)

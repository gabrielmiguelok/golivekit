# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is GoliveKit?

GoliveKit is a **Phoenix LiveView-inspired framework for Go** that enables building interactive, real-time web applications without writing JavaScript. Components run on the server, render HTML, and send minimal diffs over WebSocket to update the browser DOM.

## Build & Development Commands

```bash
# Build all packages
go build ./...

# Run tests
go test ./...

# Run tests with race detector (recommended)
go test ./... -race -v

# Run a single test
go test ./pkg/core -run TestSocketSend -v

# Run benchmarks
go test ./pkg/core -bench=. -benchmem

# Vet for issues
go vet ./...

# Run the CLI (from project root)
go run ./cmd/golive new myapp
go run ./cmd/golive dev
go run ./cmd/golive build
go run ./cmd/golive generate component Counter
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `golive new <name>` | Create new project with standard structure |
| `golive dev` | Start dev server with file watching and auto-restart |
| `golive build` | Build optimized production binary to `dist/` |
| `golive generate component <Name>` | Generate component boilerplate |
| `golive generate live <Name>` | Generate LiveView component |

## Architecture Overview

### Request Flow

```
Browser → HTTP Request → Router → Component.Mount() → Render HTML → Response
                ↓ (WebSocket connect)
Browser ← Diff ← Component.Render() ← HandleEvent() ← Event (click, input, etc.)
```

### Core Package Relationships

```
┌─────────────────────────────────────────────────────────────────┐
│                         pkg/router                               │
│                    (HTTP routing, middleware)                    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                         pkg/core                                 │
│   Component interface │ Socket │ Assigns │ Session │ Params     │
│                                                                  │
│   Component lifecycle:                                           │
│   Mount() → Render() → HandleEvent() → Render() → ...           │
└──────────────────────────┬──────────────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        ▼                  ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│  pkg/diff     │  │ pkg/transport │  │  pkg/state    │
│ (HTML differ) │  │ (WebSocket,   │  │ (State store) │
│               │  │  SSE, polling)│  │               │
└───────────────┘  └───────────────┘  └───────────────┘
```

### Key Packages

| Package | Purpose |
|---------|---------|
| `pkg/core` | Component interface, Socket, Assigns (state), Session, Params |
| `pkg/transport` | WebSocket, SSE, Long-Polling transports |
| `pkg/diff` | Hybrid diff engine for HTML changes |
| `pkg/protocol` | Wire protocol for messages (JSON/MsgPack) |
| `pkg/router` | HTTP router with LiveView route support |
| `pkg/forms` | Form handling with Ecto-style changesets |
| `pkg/pubsub` | PubSub for real-time broadcasts |
| `pkg/presence` | User presence tracking |
| `pkg/islands` | Partial hydration (islands architecture) |
| `pkg/streaming` | Streaming SSR with Suspense boundaries |
| `pkg/plugin` | Hook registry (15+ hook points) |
| `pkg/security` | CSRF, sanitization, auth helpers |
| `pkg/pool` | Buffer pools, RingBuffer for performance |
| `pkg/retry` | Retry with exponential backoff |
| `pkg/shutdown` | Graceful shutdown handler |
| `pkg/audit` | Security audit logging with 12 event types |
| `pkg/health` | Health check endpoints (liveness, readiness, Kubernetes) |
| `pkg/observability` | Prometheus-style metrics (Counter, Gauge, Histogram) |
| `pkg/recovery` | State recovery with HMAC-signed tokens and TTL |

### Component Interface

Every LiveView component implements:

```go
type Component interface {
    Name() string
    Mount(ctx context.Context, params Params, session Session) error
    Render(ctx context.Context) Renderer
    HandleEvent(ctx context.Context, event string, payload map[string]any) error
    HandleInfo(ctx context.Context, msg any) error  // For PubSub/internal messages
    Terminate(ctx context.Context, reason TerminateReason) error
}
```

Use `core.BaseComponent` for common functionality (Assigns, Socket access).

### Message Flow (WebSocket)

```go
// Client sends:
{"ref": "1", "topic": "lv:abc123", "event": "click", "payload": {"value": "increment"}}

// Server responds with diff:
{"ref": "1", "topic": "lv:abc123", "event": "diff", "payload": {"0": {"s": ["5"]}}}
```

## Concurrency Patterns

- **Socket.lastActivity** uses `atomic.Int64` for lock-free access
- **Socket.Send()** verifies `transport.IsConnected()` to protect against race with `Close()`
- **SocketManager.Broadcast()** uses worker pool (max 100 goroutines) to prevent leaks
- **Assigns.Clone()** performs deep copy for mutable values
- **Plugin hooks** use semaphore for bounded async execution
- **CircuitBreaker** protects against cascading failures (pkg/core/circuit_breaker.go)
- **Diff Engine metrics** use `atomic.Int64` for lock-free updates
- **LongPollingTransport.pendingMsgs** limited to 1000 (configurable) to prevent OOM
- Use `pkg/pool.BufferPool` for temporary buffers to reduce GC pressure

## Router Integration

Register LiveView routes using the router:

```go
r := router.New()
r.Live("/", NewCounter)                    // LiveView component
r.Handle("/_live/", client.Handler())      // JS client
r.Static("/static/", "web/static")         // Static files
r.Group("/api", func(g *RouteGroup) {      // REST API group
    g.Get("/users", handleUsers)           // Uses Go 1.22+ method patterns
    g.Post("/users", createUser)
})
http.ListenAndServe(":3000", r)
```

## Template Attributes

LiveView uses special attributes in templates:

```html
<button lv-click="increment">+</button>           <!-- Click event -->
<input lv-change="validate" lv-debounce="300"/>   <!-- Change with debounce -->
<form lv-submit="save">...</form>                 <!-- Form submit -->
<div lv-hook="Chart">...</div>                    <!-- JavaScript hook -->
```

## JavaScript Client

### HTML Attributes

| Attribute | Event | Example |
|-----------|-------|---------|
| `lv-click` | click | `<button lv-click="increment">+</button>` |
| `lv-change` | change | `<select lv-change="update">` |
| `lv-input` | input | `<input lv-input="search" lv-debounce="300">` |
| `lv-submit` | submit | `<form lv-submit="save">` |
| `lv-hook` | custom | `<div lv-hook="Chart">` |

### Slots for Efficient Updates

```html
<div data-live-view="counter">
    <span data-slot="count">0</span>
    <div data-list="items">...</div>
</div>
```

### JavaScript API

```javascript
window.liveView.pushEvent('increment', {id: 123})
window.liveView.registerHook('Chart', {
    mounted() { /* init chart */ },
    updated() { /* refresh */ }
})
```

## Testing Utilities

### Chaos Testing

```go
import "github.com/gabrielmiguelok/golivekit/pkg/testing"

// Inject latency, drops, errors
chaos := testing.NewChaosTransport(transport, testing.DefaultChaosConfig())
chaos.SetLatency(100*time.Millisecond, 50*time.Millisecond) // mean, stddev
chaos.SetDropRate(0.05) // 5% drop rate

// Fault injection
injector := testing.NewFaultInjector()
injector.InjectError("connect", errors.New("connection refused"))
```

### Fuzz Testing

```bash
go test -fuzz=FuzzParseMessage ./pkg/protocol/... -fuzztime=30s
```

### Mock Objects

- `MockSocket` - Implements core.Transport
- `MockPubSub` - Tracks published messages
- `MockTimer` - Manual time control

## Examples Location

- `examples/counter/` - Basic counter component
- `examples/chat/` - Real-time chat with PubSub
- `examples/todo/` - Todo list with forms/changesets

## WebSocket Protocol (Phoenix-compatible)

Events from client:
- `phx_join` - Join a LiveView topic
- `phx_leave` - Leave topic
- `heartbeat` / `phx_heartbeat` - Keep-alive ping

Server responses:
- `phx_reply` - Response with status and payload
- `diff` - DOM diff update

## Key Files for E2E Flow

| File | Role |
|------|------|
| `pkg/router/router.go` | HTTP routing, WebSocket upgrade, message loop |
| `pkg/router/session.go` | LiveViewSession links HTTP session to WebSocket |
| `pkg/router/transport_adapter.go` | Adapts transport.WebSocketTransport to core.Transport |
| `pkg/core/socket.go` | Socket with Send, Assign, Broadcast |
| `pkg/transport/websocket.go` | WebSocket connection handling |
| `pkg/diff/engine.go` | Diff computation with render state caching |
| `client/src/golivekit.js` | Browser-side JavaScript client |

## Running Examples

```bash
# Counter example
cd examples/counter && go run main.go
# Open http://localhost:3000

# Chat example (open multiple tabs to test)
cd examples/chat && go run main.go

# Todo example
cd examples/todo && go run main.go
```

## Testing Specific Packages

```bash
# Test core package with verbose output
go test ./pkg/core/... -v

# Test router with race detection
go test ./pkg/router/... -race -v

# Run specific test by name
go test ./pkg/core -run TestCircuitBreaker -v
```

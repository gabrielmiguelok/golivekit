# Architecture

GoliveKit is designed around a simple principle: **run your UI logic on the server, send minimal updates to the browser**.

## Overview

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

## Request Flow

### Initial HTTP Request

```
Browser → GET / → Router → Component.Mount() → Render HTML → Response
```

1. Browser requests a page
2. Router matches the URL to a LiveView component
3. Component's `Mount()` initializes state
4. `Render()` generates HTML
5. Full HTML sent to browser

### WebSocket Connection

```
Browser → WebSocket Connect → Session Created → Ready for Events
```

1. Browser's JavaScript client connects via WebSocket
2. Server creates a session linked to the component
3. Connection ready for bidirectional communication

### Event Handling

```
Browser Click → WebSocket → HandleEvent() → Render() → Diff → Browser Update
```

1. User clicks a button with `lv-click="increment"`
2. Event sent via WebSocket: `{event: "increment", payload: {}}`
3. `HandleEvent()` updates component state
4. `Render()` generates new HTML
5. Diff engine computes minimal changes
6. Only the diff is sent to browser
7. Browser applies diff to update DOM

## Component Lifecycle

```go
type Component interface {
    Name() string
    Mount(ctx context.Context, params Params, session Session) error
    Render(ctx context.Context) Renderer
    HandleEvent(ctx context.Context, event string, payload map[string]any) error
    HandleInfo(ctx context.Context, msg any) error
    Terminate(ctx context.Context, reason TerminateReason) error
}
```

### Lifecycle Stages

| Stage | Method | Purpose |
|-------|--------|---------|
| **Initialize** | `Mount()` | Set initial state, called once per session |
| **Render** | `Render()` | Generate HTML from current state |
| **Event** | `HandleEvent()` | Handle user interactions |
| **Message** | `HandleInfo()` | Handle PubSub messages |
| **Cleanup** | `Terminate()` | Cleanup when connection closes |

## Diff Engine

GoliveKit uses a hybrid diff algorithm for optimal performance:

### 1. Compile-Time Analysis

Templates are analyzed at compile time to identify:
- Static vs dynamic content
- Slot positions for targeted updates
- List structures for keyed diffing

### 2. Runtime Optimization

```go
// Fast path: O(1) comparison using content hashes
if oldHash == newHash {
    return nil // No changes
}

// Slot updates: Only diff specific regions
if slot := findSlot(element); slot != nil {
    return diffSlot(oldSlot, newSlot)
}

// Full diff: Standard tree comparison
return diffTrees(oldTree, newTree)
```

### 3. Wire Format

Diffs are sent as minimal JSON:

```json
{"0": {"s": ["5"]}}
```

This means: "At slot 0, replace static content with '5'"

## Transports

### WebSocket (Primary)

- Bidirectional, real-time communication
- Automatic reconnection with exponential backoff
- Heartbeat every 30 seconds

### Server-Sent Events (Fallback)

- One-way server-to-client
- Uses POST for client-to-server events
- Good for environments blocking WebSocket

### Long-Polling (Legacy)

- HTTP request/response cycle
- Maximum compatibility
- Higher latency

## Socket & State

### Socket

Each connection has a Socket that provides:

```go
socket.Send(message)           // Send to this client
socket.Broadcast(topic, event) // Send to all subscribers
socket.Assign("key", value)    // Set state
socket.Get("key")              // Get state
```

### Assigns

Component state stored in Assigns:

```go
// Thread-safe state management
c.Assigns().Set("count", 5)
c.Assigns().Set("user", user)

// Deep cloning for safety
clone := c.Assigns().Clone()
```

## PubSub

Real-time broadcasts across components:

```go
// Subscribe to a topic
pubsub.Subscribe("room:123")

// Broadcast to all subscribers
pubsub.Broadcast("room:123", "new_message", payload)

// Handle in component
func (c *Chat) HandleInfo(ctx context.Context, msg any) error {
    if m, ok := msg.(*Message); ok {
        c.Messages = append(c.Messages, m)
    }
    return nil
}
```

## Islands Architecture

Partial hydration for optimal performance:

```go
island := islands.NewIsland("widget", "Widget",
    islands.WithHydration(islands.HydrateOnVisible),
    islands.WithPriority(islands.PriorityHigh),
)
```

### Hydration Strategies

| Strategy | When to Hydrate |
|----------|-----------------|
| `load` | Immediately on page load |
| `visible` | When element becomes visible |
| `idle` | When browser is idle |
| `interaction` | On first user interaction |
| `none` | Never (static content) |

## Plugin System

Extend GoliveKit with hooks:

```go
type MyPlugin struct {
    plugin.BasePlugin
}

func (p *MyPlugin) Init(app *plugin.App) error {
    app.Hooks().Register(plugin.HookAfterMount, "myplugin", p.onMount)
    return nil
}
```

### Available Hooks

| Hook | Trigger |
|------|---------|
| `beforeMount` / `afterMount` | Component mounting |
| `beforeRender` / `afterRender` | Rendering |
| `beforeEvent` / `afterEvent` | Event handling |
| `onConnect` / `onDisconnect` | Connection lifecycle |
| `onError` / `onPanic` | Error handling |

## Performance Targets

| Metric | Target |
|--------|--------|
| Perceived latency | <10ms |
| Connections/node | 50,000 |
| Memory/connection | 20-50KB |
| Event latency p50 | 3ms |
| Render time p50 | 0.5ms |
| Diff size typical | 100-300 bytes |

## Concurrency Model

- **Socket.lastActivity**: Atomic operations for lock-free access
- **Socket.Send()**: Checks `IsConnected()` before sending
- **Broadcast**: Worker pool (max 100 goroutines)
- **Assigns.Clone()**: Deep copy for thread safety
- **CircuitBreaker**: Protects against cascading failures

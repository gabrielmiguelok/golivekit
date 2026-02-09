# Examples

GoliveKit includes several example applications demonstrating different features and patterns.

## Basic Examples

### Counter

**Location:** `examples/counter/`

A simple counter demonstrating:
- Component state management
- Event handling (`lv-click`)
- Real-time DOM updates

```bash
cd examples/counter && go run main.go
# Open http://localhost:3000
```

**Key concepts:**
- `Mount()` initializes state
- `HandleEvent()` updates state
- `Render()` generates HTML
- Diffs are sent automatically

### Chat

**Location:** `examples/chat/`

Real-time chat room demonstrating:
- PubSub for broadcasting messages
- Presence tracking (who's online)
- Multiple concurrent users

```bash
cd examples/chat && go run main.go
# Open http://localhost:3000 in multiple tabs
```

**Key concepts:**
- `pubsub.Subscribe()` joins a topic
- `pubsub.Broadcast()` sends to all subscribers
- `HandleInfo()` receives PubSub messages
- Presence tracks connected users

### Todo

**Location:** `examples/todo/`

Todo list demonstrating:
- Form handling with `lv-submit`
- Ecto-style changesets for validation
- CRUD operations
- Keyed lists with `data-key`

```bash
cd examples/todo && go run main.go
# Open http://localhost:3000
```

**Key concepts:**
- `forms.NewChangeset()` for validation
- Form data serialization
- List diffing with stable keys

## Advanced Demos

**Location:** `examples/demo/`

The demo application showcases advanced GoliveKit features through 7 interactive demonstrations plus a documentation hub.

```bash
cd examples/demo && go run main.go
# Open http://localhost:3000
```

### Demo Hub (`/demos`)

Central portal to access all demonstrations with live previews.

### Realtime Playlist (`/demos/realtime`)

**Features:**
- Multi-user playlist voting
- Real-time updates across all clients
- Presence showing active users
- PubSub broadcasting

**Concepts demonstrated:**
- `pubsub.Broadcast()` for real-time sync
- `presence.Track()` for user tracking
- Optimistic UI updates
- Voting with rate limiting

### Forms Wizard (`/demos/forms`)

**Features:**
- 4-step form wizard
- Async validation (email availability check)
- Step navigation
- Data persistence across steps

**Concepts demonstrated:**
- Multi-step form state
- `lv-change` for real-time validation
- `lv-debounce` for API calls
- Changeset validation patterns

### File Manager (`/demos/uploads`)

**Features:**
- Chunked file uploads
- Upload progress tracking
- Image previews
- File list management

**Concepts demonstrated:**
- `uploads.ChunkedUpload()` for large files
- Progress events via WebSocket
- File type validation
- Preview generation

### Live Dashboard (`/demos/dashboard`)

**Features:**
- Streaming data updates
- Multiple charts and metrics
- Real-time counters
- Server-sent events

**Concepts demonstrated:**
- `streaming.SSR()` with suspense
- Periodic server pushes
- Chart.js hook integration
- Dashboard layout patterns

### Snake Game (`/demos/game`)

**Features:**
- 60 FPS server-side game loop
- Keyboard input handling
- Collision detection
- Score tracking

**Concepts demonstrated:**
- High-frequency updates
- `lv-keydown` event handling
- Game state on server
- Minimal diff for performance

### Collaborative Editor (`/demos/editor`)

**Features:**
- Multi-cursor editing
- Real-time text sync
- User presence indicators
- Auto-save with debounce

**Concepts demonstrated:**
- Operational transforms
- Cursor position sync
- `lv-input` with debounce
- Conflict resolution

### Kitchen Sink (`/demos/showcase`)

**Features:**
- All GoliveKit features in one page
- Interactive examples
- Code snippets

**Concepts demonstrated:**
- Every `lv-*` attribute
- All hook types
- Form patterns
- State management patterns

## Interactive Documentation (`/docs`)

The demo includes 11 sections of interactive documentation:

| Section | Content |
|---------|---------|
| Getting Started | Installation, CLI, first component |
| Core Concepts | Lifecycle, state, events |
| Routing | Router, LiveView routes, middleware |
| Events | All event types and attributes |
| Forms | Changesets, validation, multi-step |
| Real-time | PubSub, Presence, broadcasting |
| State | Assigns, session, persistence |
| Security | CSRF, XSS, rate limiting |
| Performance | Optimization, profiling |
| CLI Reference | All commands and options |
| Package Reference | All 29 packages |

## Running Examples

### Prerequisites

1. Go 1.21 or later
2. templ (for template generation)

```bash
# Install templ
go install github.com/a-h/templ/cmd/templ@latest
```

### Steps

```bash
# Clone the repository
git clone https://github.com/gabrielmiguelok/golivekit
cd golivekit

# Run counter example
cd examples/counter
templ generate  # If templates were modified
go run main.go
```

### Development Mode

For hot reload during development:

```bash
# Install the CLI
go install ./cmd/golive

# Run with hot reload
cd examples/counter
golive dev
```

## Creating Your Own Example

1. **Create directory:**
```bash
mkdir examples/myexample
cd examples/myexample
```

2. **Initialize module:**
```bash
go mod init myexample
go get github.com/gabrielmiguelok/golivekit
```

3. **Create component:**
```go
// components/hello.go
package components

import (
    "context"
    "github.com/gabrielmiguelok/golivekit/pkg/core"
)

type Hello struct {
    core.BaseComponent
    Name string
}

func NewHello() core.Component {
    return &Hello{}
}

func (h *Hello) Name() string { return "hello" }

func (h *Hello) Mount(ctx context.Context, params core.Params, session core.Session) error {
    h.Name = "World"
    h.Assigns().Set("name", h.Name)
    return nil
}

func (h *Hello) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
    if name, ok := payload["name"].(string); ok {
        h.Name = name
        h.Assigns().Set("name", h.Name)
    }
    return nil
}

func (h *Hello) Render(ctx context.Context) core.Renderer {
    return HelloTemplate(h.Name)
}
```

4. **Create template:**
```templ
// components/hello.templ
package components

templ HelloTemplate(name string) {
    <div data-live-view="hello">
        <h1>Hello, <span data-slot="name">{ name }</span>!</h1>
        <input
            type="text"
            value={ name }
            lv-input="update_name"
            lv-debounce="300"
            placeholder="Enter your name"
        />
    </div>
}
```

5. **Create main:**
```go
// main.go
package main

import (
    "log"
    "net/http"

    "github.com/gabrielmiguelok/golivekit/pkg/router"
    "github.com/gabrielmiguelok/golivekit/client"
    "myexample/components"
)

func main() {
    r := router.New()
    r.Live("/", components.NewHello)
    r.Handle("/_live/", client.Handler())

    log.Println("Running at http://localhost:3000")
    log.Fatal(http.ListenAndServe(":3000", r))
}
```

6. **Run:**
```bash
templ generate
go run main.go
```

# Getting Started

This guide will help you get up and running with GoliveKit in under 5 minutes.

## Prerequisites

- Go 1.21 or later
- Basic knowledge of Go and HTML

## Installation

### Option 1: Using the CLI (Recommended)

```bash
# Install the CLI
go install github.com/gabrielmiguelok/golivekit/cmd/golive@latest

# Create a new project
golive new myapp

# Start the development server
cd myapp && golive dev
```

Open http://localhost:3000 in your browser.

### Option 2: Manual Setup

```bash
# Create a new Go module
mkdir myapp && cd myapp
go mod init myapp

# Install GoliveKit
go get github.com/gabrielmiguelok/golivekit
```

## Project Structure

A typical GoliveKit project looks like this:

```
myapp/
├── main.go              # Entry point
├── components/          # LiveView components
│   ├── counter.go       # Component logic
│   └── counter.templ    # Template (using templ)
├── web/
│   └── static/          # Static assets (CSS, images)
└── go.mod
```

## Your First Component

### 1. Create the Component

```go
// components/counter.go
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

### 2. Create the Template

Using [templ](https://templ.guide):

```templ
// components/counter.templ
package components

import "fmt"

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
// main.go
package main

import (
    "log"
    "net/http"

    "github.com/gabrielmiguelok/golivekit/pkg/router"
    "github.com/gabrielmiguelok/golivekit/client"
    "myapp/components"
)

func main() {
    r := router.New()

    // Register LiveView component
    r.Live("/", components.NewCounter)

    // Serve JavaScript client
    r.Handle("/_live/", client.Handler())

    // Static files
    r.Static("/static/", "web/static")

    log.Println("Server running at http://localhost:3000")
    log.Fatal(http.ListenAndServe(":3000", r))
}
```

### 4. Run

```bash
# Generate templ files
templ generate

# Run the server
go run main.go
```

Visit http://localhost:3000 and click the buttons to see real-time updates!

## How It Works

1. **Initial Request**: Browser requests `/`, server renders the counter HTML
2. **WebSocket Connect**: Browser connects via WebSocket for real-time updates
3. **User Clicks**: Click event sent to server via WebSocket
4. **State Update**: `HandleEvent` updates the count
5. **Diff Sent**: Server computes minimal diff and sends to browser
6. **DOM Update**: Browser applies diff to update only changed elements

## Next Steps

- [Architecture](./architecture.md) - Understand how GoliveKit works
- [JavaScript Client](./javascript-client.md) - Learn about browser-side features
- [Examples](./examples.md) - Explore sample applications
- [Testing](./testing.md) - Test your components

## CLI Reference

| Command | Description |
|---------|-------------|
| `golive new <name>` | Create a new project |
| `golive dev` | Start dev server with hot reload |
| `golive build` | Build for production |
| `golive generate component <Name>` | Generate component boilerplate |
| `golive generate live <Name>` | Generate LiveView component |

// Package main demonstrates a simple counter component using GoliveKit.
package main

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

// Counter is a simple counter component.
type Counter struct {
	core.BaseComponent
	Count int
}

// NewCounter creates a new counter component.
func NewCounter() core.Component {
	return &Counter{}
}

// Name returns the component name.
func (c *Counter) Name() string {
	return "counter"
}

// Mount initializes the counter.
func (c *Counter) Mount(ctx context.Context, params core.Params, session core.Session) error {
	c.Count = 0
	c.Assigns().Set("count", c.Count)
	return nil
}

// HandleEvent handles user interactions.
func (c *Counter) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	switch event {
	case "increment":
		c.Count++
	case "decrement":
		c.Count--
	case "reset":
		c.Count = 0
	}
	c.Assigns().Set("count", c.Count)
	return nil
}

// Render returns the HTML representation.
func (c *Counter) Render(ctx context.Context) core.Renderer {
	return core.RendererFunc(func(ctx context.Context, w io.Writer) error {
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>GoliveKit Counter Example</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
        }
        .counter {
            background: white;
            padding: 3rem;
            border-radius: 1rem;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            text-align: center;
        }
        h1 {
            font-size: 3rem;
            margin: 0 0 2rem;
            color: #333;
        }
        .buttons {
            display: flex;
            gap: 1rem;
        }
        .btn {
            padding: 0.75rem 1.5rem;
            font-size: 1rem;
            border: none;
            border-radius: 0.5rem;
            cursor: pointer;
            transition: transform 0.1s, box-shadow 0.1s;
        }
        .btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        }
        .btn:active {
            transform: translateY(0);
        }
        .btn-red { background: #ef4444; color: white; }
        .btn-green { background: #22c55e; color: white; }
        .btn-gray { background: #6b7280; color: white; }
    </style>
</head>
<body>
    <div data-live-view="counter">
        <div class="counter" data-slot="counter">
            <h1>Count: <span data-slot="count">%d</span></h1>
            <div class="buttons">
                <button lv-click="decrement" class="btn btn-red">- Decrement</button>
                <button lv-click="reset" class="btn btn-gray">Reset</button>
                <button lv-click="increment" class="btn btn-green">+ Increment</button>
            </div>
        </div>
    </div>
    <script src="/_live/golivekit.js"></script>
</body>
</html>`, c.Count)
		_, err := w.Write([]byte(html))
		return err
	})
}

func main() {
	// Create router
	r := router.New()

	// Serve GoliveKit client JS
	r.Handle("/_live/", http.StripPrefix("/_live/", client.Handler()))

	// Register LiveView route
	r.Live("/", NewCounter)

	log.Println("🚀 Counter example starting at http://localhost:3000")
	log.Println("Press Ctrl+C to stop")
	log.Fatal(http.ListenAndServe(":3000", r))
}

// Package devserver provides a development server for GoliveKit.
package devserver

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// DevServer is a development server with hot reload.
type DevServer struct {
	addr        string
	watchDirs   []string
	buildCmd    string
	clients     map[string]chan struct{}
	router      http.Handler
	buildError  error
	mu          sync.RWMutex
}

// Config configures the development server.
type Config struct {
	Addr      string
	WatchDirs []string
	BuildCmd  string
	Router    http.Handler
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		Addr:      ":3000",
		WatchDirs: []string{".", "web", "internal"},
		BuildCmd:  "go build -o ./tmp/main ./cmd/server",
	}
}

// New creates a new development server.
func New(config *Config) *DevServer {
	if config == nil {
		config = DefaultConfig()
	}
	return &DevServer{
		addr:      config.Addr,
		watchDirs: config.WatchDirs,
		buildCmd:  config.BuildCmd,
		clients:   make(map[string]chan struct{}),
		router:    config.Router,
	}
}

// Start starts the development server.
func (ds *DevServer) Start(ctx context.Context) error {
	// Initial build
	if err := ds.build(); err != nil {
		ds.mu.Lock()
		ds.buildError = err
		ds.mu.Unlock()
		fmt.Printf("Build error: %v\n", err)
	}

	// Start file watcher
	go ds.watch(ctx)

	// Create HTTP server
	mux := http.NewServeMux()

	// Hot reload endpoint
	mux.HandleFunc("/_dev/reload", ds.handleReload)

	// Error overlay endpoint
	mux.HandleFunc("/_dev/error", ds.handleError)

	// Main handler
	mux.HandleFunc("/", ds.handleMain)

	fmt.Printf("Development server starting at http://localhost%s\n", ds.addr)
	fmt.Println("Hot reload enabled")

	server := &http.Server{
		Addr:    ds.addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	return server.ListenAndServe()
}

func (ds *DevServer) build() error {
	fmt.Println("Building...")
	start := time.Now()

	cmd := exec.Command("sh", "-c", ds.buildCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Printf("Build completed in %v\n", time.Since(start))
	return nil
}

func (ds *DevServer) watch(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastMod time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check for file changes
			currentMod := ds.getLatestMod()
			if currentMod.After(lastMod) {
				lastMod = currentMod
				fmt.Println("File change detected, rebuilding...")

				if err := ds.build(); err != nil {
					ds.mu.Lock()
					ds.buildError = err
					ds.mu.Unlock()
					fmt.Printf("Build error: %v\n", err)
				} else {
					ds.mu.Lock()
					ds.buildError = nil
					ds.mu.Unlock()
				}

				ds.notifyClients()
			}
		}
	}
}

func (ds *DevServer) getLatestMod() time.Time {
	var latest time.Time

	for _, dir := range ds.watchDirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			// Only watch Go and template files
			ext := filepath.Ext(path)
			if ext == ".go" || ext == ".html" || ext == ".templ" {
				if info.ModTime().After(latest) {
					latest = info.ModTime()
				}
			}
			return nil
		})
	}

	return latest
}

func (ds *DevServer) notifyClients() {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	for _, ch := range ds.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (ds *DevServer) handleReload(w http.ResponseWriter, r *http.Request) {
	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Create channel for this client
	clientID := fmt.Sprintf("%d", time.Now().UnixNano())
	ch := make(chan struct{}, 1)

	ds.mu.Lock()
	ds.clients[clientID] = ch
	ds.mu.Unlock()

	defer func() {
		ds.mu.Lock()
		delete(ds.clients, clientID)
		ds.mu.Unlock()
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	// Send initial connection message
	fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()

	// Wait for reload events
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprintf(w, "data: reload\n\n")
			flusher.Flush()
		}
	}
}

func (ds *DevServer) handleError(w http.ResponseWriter, r *http.Request) {
	ds.mu.RLock()
	err := ds.buildError
	ds.mu.RUnlock()

	if err == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `{"error": "%s"}`, err.Error())
}

func (ds *DevServer) handleMain(w http.ResponseWriter, r *http.Request) {
	// Check for build error
	ds.mu.RLock()
	err := ds.buildError
	ds.mu.RUnlock()

	if err != nil {
		ds.renderErrorOverlay(w, err)
		return
	}

	// Pass to main router if available
	if ds.router != nil {
		ds.router.ServeHTTP(w, r)
		return
	}

	// Default response
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(devPageHTML))
}

func (ds *DevServer) renderErrorOverlay(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusInternalServerError)

	tmpl := template.Must(template.New("error").Parse(errorOverlayHTML))
	tmpl.Execute(w, map[string]string{
		"Error": err.Error(),
	})
}

const devPageHTML = `<!DOCTYPE html>
<html>
<head>
    <title>GoliveKit Dev Server</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            padding: 2rem;
            max-width: 800px;
            margin: 0 auto;
        }
        h1 { color: #333; }
        code { background: #f4f4f4; padding: 0.2rem 0.4rem; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>GoliveKit Development Server</h1>
    <p>Server is running. Add routes to see your application.</p>
    <p>Hot reload is <strong>enabled</strong>.</p>
    <script>
        const evtSource = new EventSource('/_dev/reload');
        evtSource.onmessage = function(event) {
            if (event.data === 'reload') {
                window.location.reload();
            }
        };
    </script>
</body>
</html>`

const errorOverlayHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Build Error</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #1a1a2e;
            color: #eee;
            padding: 2rem;
            margin: 0;
        }
        .error-container {
            background: #16213e;
            border-left: 4px solid #e74c3c;
            padding: 1rem;
            border-radius: 4px;
        }
        h1 { color: #e74c3c; margin-top: 0; }
        pre {
            background: #0f0f23;
            padding: 1rem;
            overflow-x: auto;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <div class="error-container">
        <h1>Build Error</h1>
        <pre>{{.Error}}</pre>
    </div>
    <script>
        const evtSource = new EventSource('/_dev/reload');
        evtSource.onmessage = function(event) {
            if (event.data === 'reload') {
                window.location.reload();
            }
        };
    </script>
</body>
</html>`

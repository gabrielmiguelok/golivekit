// Package shutdown provides graceful shutdown handling for GoliveKit applications.
package shutdown

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"
)

// Common shutdown errors.
var (
	ErrShutdownTimeout = errors.New("shutdown timed out")
	ErrAlreadyClosed   = errors.New("shutdown handler already closed")
)

// Hook represents a shutdown hook.
type Hook struct {
	// Name identifies the hook for logging.
	Name string

	// Priority determines execution order (lower = earlier).
	Priority int

	// Fn is the function to execute during shutdown.
	Fn func(ctx context.Context) error
}

// Config configures the shutdown handler.
type Config struct {
	// Timeout is the maximum time to wait for graceful shutdown.
	Timeout time.Duration

	// Signals are the OS signals to listen for.
	Signals []os.Signal

	// OnStart is called when the handler starts listening.
	OnStart func()

	// OnShutdown is called when shutdown begins.
	OnShutdown func()

	// OnHookComplete is called when a hook completes.
	OnHookComplete func(name string, err error, duration time.Duration)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Timeout: 30 * time.Second,
		Signals: []os.Signal{os.Interrupt, syscall.SIGTERM},
	}
}

// Handler manages graceful shutdown.
type Handler struct {
	config *Config
	hooks  []Hook
	done   chan struct{}
	closed bool
	mu     sync.Mutex
}

// NewHandler creates a new shutdown handler.
func NewHandler(config *Config) *Handler {
	if config == nil {
		config = DefaultConfig()
	}

	return &Handler{
		config: config,
		hooks:  make([]Hook, 0),
		done:   make(chan struct{}),
	}
}

// Register adds a shutdown hook.
func (h *Handler) Register(hook Hook) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hooks = append(h.hooks, hook)
}

// RegisterFunc is a convenience method to register a function as a hook.
func (h *Handler) RegisterFunc(name string, priority int, fn func(ctx context.Context) error) {
	h.Register(Hook{
		Name:     name,
		Priority: priority,
		Fn:       fn,
	})
}

// Wait blocks until a shutdown signal is received and then performs graceful shutdown.
func (h *Handler) Wait() error {
	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, h.config.Signals...)

	if h.config.OnStart != nil {
		h.config.OnStart()
	}

	// Wait for signal
	select {
	case <-sigCh:
		signal.Stop(sigCh)
	case <-h.done:
		return nil
	}

	return h.Shutdown()
}

// Shutdown performs graceful shutdown without waiting for signals.
func (h *Handler) Shutdown() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return ErrAlreadyClosed
	}
	h.closed = true
	close(h.done)

	// Sort hooks by priority
	hooks := make([]Hook, len(h.hooks))
	copy(hooks, h.hooks)
	h.mu.Unlock()

	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Priority < hooks[j].Priority
	})

	if h.config.OnShutdown != nil {
		h.config.OnShutdown()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), h.config.Timeout)
	defer cancel()

	// Execute hooks
	var errs []error
	for _, hook := range hooks {
		start := time.Now()
		err := hook.Fn(ctx)
		duration := time.Since(start)

		if h.config.OnHookComplete != nil {
			h.config.OnHookComplete(hook.Name, err, duration)
		}

		if err != nil {
			errs = append(errs, err)
		}

		// Check if context is done (timeout)
		select {
		case <-ctx.Done():
			return errors.Join(append(errs, ErrShutdownTimeout)...)
		default:
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Done returns a channel that's closed when shutdown is complete.
func (h *Handler) Done() <-chan struct{} {
	return h.done
}

// IsClosed returns true if the handler has been closed.
func (h *Handler) IsClosed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

// Global handler for convenience

var (
	globalHandler *Handler
	globalOnce    sync.Once
)

// Global returns the global shutdown handler.
func Global() *Handler {
	globalOnce.Do(func() {
		globalHandler = NewHandler(nil)
	})
	return globalHandler
}

// Register adds a hook to the global handler.
func Register(hook Hook) {
	Global().Register(hook)
}

// RegisterFunc adds a function hook to the global handler.
func RegisterFunc(name string, priority int, fn func(ctx context.Context) error) {
	Global().RegisterFunc(name, priority, fn)
}

// Wait blocks on the global handler.
func Wait() error {
	return Global().Wait()
}

// Shutdown triggers shutdown on the global handler.
func Shutdown() error {
	return Global().Shutdown()
}

// Done returns the done channel from the global handler.
func Done() <-chan struct{} {
	return Global().Done()
}

// Common hook priorities

const (
	// PriorityFirst runs earliest
	PriorityFirst = 0

	// PriorityHTTP for HTTP server shutdown
	PriorityHTTP = 100

	// PriorityWebSocket for WebSocket cleanup
	PriorityWebSocket = 200

	// PriorityDB for database connections
	PriorityDB = 300

	// PriorityCache for cache cleanup
	PriorityCache = 400

	// PriorityLast runs latest
	PriorityLast = 1000
)

// Helpers for common shutdown tasks

// HTTPServerHook creates a hook for shutting down an HTTP server.
func HTTPServerHook(name string, shutdownFn func(ctx context.Context) error) Hook {
	return Hook{
		Name:     name,
		Priority: PriorityHTTP,
		Fn:       shutdownFn,
	}
}

// CloseableHook creates a hook for anything with a Close() method.
func CloseableHook(name string, priority int, closer interface{ Close() error }) Hook {
	return Hook{
		Name:     name,
		Priority: priority,
		Fn: func(ctx context.Context) error {
			return closer.Close()
		},
	}
}

// TimeoutHook wraps a hook function with a specific timeout.
func TimeoutHook(hook Hook, timeout time.Duration) Hook {
	return Hook{
		Name:     hook.Name,
		Priority: hook.Priority,
		Fn: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			return hook.Fn(ctx)
		},
	}
}

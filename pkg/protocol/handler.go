package protocol

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Common handler errors.
var (
	ErrHandlerNotFound = errors.New("handler not found for message type")
	ErrHandlerTimeout  = errors.New("handler timeout")
	ErrHandlerPanic    = errors.New("handler panicked")
)

// MessageHandler processes protocol messages.
type MessageHandler interface {
	// HandleMessage processes a message and returns a response.
	HandleMessage(ctx context.Context, msg *Message) (*Message, error)
}

// MessageHandlerFunc is an adapter to allow functions as MessageHandler.
type MessageHandlerFunc func(ctx context.Context, msg *Message) (*Message, error)

// HandleMessage implements MessageHandler.
func (f MessageHandlerFunc) HandleMessage(ctx context.Context, msg *Message) (*Message, error) {
	return f(ctx, msg)
}

// Dispatcher routes messages to appropriate handlers.
type Dispatcher struct {
	handlers    map[MessageType]MessageHandler
	middleware  []MiddlewareFunc
	metrics     *DispatcherMetrics
	timeout     time.Duration
	mu          sync.RWMutex
}

// MiddlewareFunc is middleware that wraps message handling.
type MiddlewareFunc func(next MessageHandler) MessageHandler

// DispatcherMetrics tracks dispatcher performance.
type DispatcherMetrics struct {
	MessagesReceived  int64
	MessagesProcessed int64
	MessagesErrored   int64
	TotalLatency      time.Duration
	mu                sync.Mutex
}

// NewDispatcher creates a new message dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers:   make(map[MessageType]MessageHandler),
		middleware: make([]MiddlewareFunc, 0),
		metrics:    &DispatcherMetrics{},
		timeout:    30 * time.Second,
	}
}

// SetTimeout sets the handler timeout.
func (d *Dispatcher) SetTimeout(timeout time.Duration) {
	d.timeout = timeout
}

// Register adds a handler for a message type.
func (d *Dispatcher) Register(msgType MessageType, handler MessageHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[msgType] = handler
}

// RegisterFunc adds a handler function for a message type.
func (d *Dispatcher) RegisterFunc(msgType MessageType, fn func(ctx context.Context, msg *Message) (*Message, error)) {
	d.Register(msgType, MessageHandlerFunc(fn))
}

// Use adds middleware to the dispatcher.
func (d *Dispatcher) Use(mw MiddlewareFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.middleware = append(d.middleware, mw)
}

// Dispatch routes a message to its handler.
func (d *Dispatcher) Dispatch(ctx context.Context, msg *Message) (*Message, error) {
	d.mu.RLock()
	handler, ok := d.handlers[msg.Type]
	middleware := make([]MiddlewareFunc, len(d.middleware))
	copy(middleware, d.middleware)
	d.mu.RUnlock()

	// Update metrics
	d.metrics.mu.Lock()
	d.metrics.MessagesReceived++
	d.metrics.mu.Unlock()

	if !ok {
		d.metrics.mu.Lock()
		d.metrics.MessagesErrored++
		d.metrics.mu.Unlock()
		return nil, fmt.Errorf("%w: %s", ErrHandlerNotFound, msg.Type)
	}

	// Apply middleware (reverse order)
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}

	// Execute with timeout
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	result, err := d.executeWithRecovery(ctx, handler, msg)
	elapsed := time.Since(start)

	// Update metrics
	d.metrics.mu.Lock()
	d.metrics.TotalLatency += elapsed
	if err != nil {
		d.metrics.MessagesErrored++
	} else {
		d.metrics.MessagesProcessed++
	}
	d.metrics.mu.Unlock()

	return result, err
}

// executeWithRecovery executes a handler with panic recovery.
func (d *Dispatcher) executeWithRecovery(ctx context.Context, handler MessageHandler, msg *Message) (result *Message, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrHandlerPanic, r)
		}
	}()

	return handler.HandleMessage(ctx, msg)
}

// Metrics returns the dispatcher metrics.
func (d *Dispatcher) Metrics() DispatcherMetrics {
	d.metrics.mu.Lock()
	defer d.metrics.mu.Unlock()

	return DispatcherMetrics{
		MessagesReceived:  d.metrics.MessagesReceived,
		MessagesProcessed: d.metrics.MessagesProcessed,
		MessagesErrored:   d.metrics.MessagesErrored,
		TotalLatency:      d.metrics.TotalLatency,
	}
}

// Common middleware

// LoggingMiddleware logs message handling.
func LoggingMiddleware(logger func(string, ...any)) MiddlewareFunc {
	return func(next MessageHandler) MessageHandler {
		return MessageHandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
			start := time.Now()
			logger("handling message type=%s topic=%s event=%s", msg.Type, msg.Topic, msg.Event)

			result, err := next.HandleMessage(ctx, msg)

			if err != nil {
				logger("message error type=%s duration=%v error=%v", msg.Type, time.Since(start), err)
			} else {
				logger("message handled type=%s duration=%v", msg.Type, time.Since(start))
			}

			return result, err
		})
	}
}

// RecoveryMiddleware recovers from panics.
func RecoveryMiddleware(onPanic func(any)) MiddlewareFunc {
	return func(next MessageHandler) MessageHandler {
		return MessageHandlerFunc(func(ctx context.Context, msg *Message) (result *Message, err error) {
			defer func() {
				if r := recover(); r != nil {
					if onPanic != nil {
						onPanic(r)
					}
					err = fmt.Errorf("handler panic: %v", r)
				}
			}()
			return next.HandleMessage(ctx, msg)
		})
	}
}

// TimeoutMiddleware adds timeout to handlers.
func TimeoutMiddleware(timeout time.Duration) MiddlewareFunc {
	return func(next MessageHandler) MessageHandler {
		return MessageHandlerFunc(func(ctx context.Context, msg *Message) (*Message, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			done := make(chan struct {
				msg *Message
				err error
			}, 1)

			go func() {
				result, err := next.HandleMessage(ctx, msg)
				done <- struct {
					msg *Message
					err error
				}{result, err}
			}()

			select {
			case result := <-done:
				return result.msg, result.err
			case <-ctx.Done():
				return nil, ErrHandlerTimeout
			}
		})
	}
}

// Router provides event-based routing within a dispatcher.
type Router struct {
	routes map[string]MessageHandler
	mu     sync.RWMutex
}

// NewRouter creates a new event router.
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]MessageHandler),
	}
}

// On registers a handler for an event.
func (r *Router) On(event string, handler MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[event] = handler
}

// OnFunc registers a handler function for an event.
func (r *Router) OnFunc(event string, fn func(ctx context.Context, msg *Message) (*Message, error)) {
	r.On(event, MessageHandlerFunc(fn))
}

// HandleMessage implements MessageHandler.
func (r *Router) HandleMessage(ctx context.Context, msg *Message) (*Message, error) {
	r.mu.RLock()
	handler, ok := r.routes[msg.Event]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no handler for event: %s", msg.Event)
	}

	return handler.HandleMessage(ctx, msg)
}

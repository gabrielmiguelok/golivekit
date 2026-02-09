// Package plugin provides the extensibility system for GoliveKit.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// HookPoint defines extension points in the component lifecycle.
type HookPoint string

const (
	// Lifecycle hooks
	HookBeforeMount  HookPoint = "beforeMount"
	HookAfterMount   HookPoint = "afterMount"
	HookBeforeRender HookPoint = "beforeRender"
	HookAfterRender  HookPoint = "afterRender"
	HookBeforeEvent  HookPoint = "beforeEvent"
	HookAfterEvent   HookPoint = "afterEvent"

	// Connection hooks
	HookOnConnect    HookPoint = "onConnect"
	HookOnDisconnect HookPoint = "onDisconnect"
	HookOnReconnect  HookPoint = "onReconnect"

	// State hooks
	HookBeforeAssign   HookPoint = "beforeAssign"
	HookAfterAssign    HookPoint = "afterAssign"
	HookOnStateRestore HookPoint = "onStateRestore"

	// Transport hooks
	HookBeforeSend   HookPoint = "beforeSend"
	HookAfterReceive HookPoint = "afterReceive"

	// Error hooks
	HookOnError HookPoint = "onError"
	HookOnPanic HookPoint = "onPanic"
)

// AllHookPoints returns all available hook points.
func AllHookPoints() []HookPoint {
	return []HookPoint{
		HookBeforeMount, HookAfterMount,
		HookBeforeRender, HookAfterRender,
		HookBeforeEvent, HookAfterEvent,
		HookOnConnect, HookOnDisconnect, HookOnReconnect,
		HookBeforeAssign, HookAfterAssign, HookOnStateRestore,
		HookBeforeSend, HookAfterReceive,
		HookOnError, HookOnPanic,
	}
}

// HookContext contains information for hook execution.
type HookContext struct {
	Context    context.Context
	Component  core.Component
	Socket     *core.Socket
	Event      *core.Event      // Only for event hooks
	RenderData *core.RenderData // Only for render hooks
	Error      error            // Only for error hooks
	Metadata   map[string]any
}

// NewHookContext creates a new hook context.
func NewHookContext(ctx context.Context) *HookContext {
	return &HookContext{
		Context:  ctx,
		Metadata: make(map[string]any),
	}
}

// WithComponent sets the component.
func (hc *HookContext) WithComponent(comp core.Component) *HookContext {
	hc.Component = comp
	return hc
}

// WithSocket sets the socket.
func (hc *HookContext) WithSocket(socket *core.Socket) *HookContext {
	hc.Socket = socket
	return hc
}

// WithEvent sets the event.
func (hc *HookContext) WithEvent(event *core.Event) *HookContext {
	hc.Event = event
	return hc
}

// WithError sets the error.
func (hc *HookContext) WithError(err error) *HookContext {
	hc.Error = err
	return hc
}

// HookFunc is the signature of a hook function.
type HookFunc func(ctx *HookContext) error

// HookRegistry manages all registered hooks.
type HookRegistry struct {
	hooks              map[HookPoint][]hookEntry
	mu                 sync.RWMutex
	metrics            *HookMetrics
	maxConcurrentHooks int // Maximum concurrent async hooks
}

type hookEntry struct {
	name     string
	fn       HookFunc
	priority int  // Lower = executes first
	async    bool // If can execute in parallel
}

// HookMetrics tracks hook execution metrics.
type HookMetrics struct {
	ExecutionCount    map[HookPoint]int64
	ExecutionDuration map[HookPoint]time.Duration
	ErrorCount        map[HookPoint]int64
	mu                sync.Mutex
}

// NewHookMetrics creates new hook metrics.
func NewHookMetrics() *HookMetrics {
	return &HookMetrics{
		ExecutionCount:    make(map[HookPoint]int64),
		ExecutionDuration: make(map[HookPoint]time.Duration),
		ErrorCount:        make(map[HookPoint]int64),
	}
}

// RecordExecution records a hook execution.
func (hm *HookMetrics) RecordExecution(point HookPoint, duration time.Duration) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.ExecutionCount[point]++
	hm.ExecutionDuration[point] += duration
}

// RecordError records a hook error.
func (hm *HookMetrics) RecordError(point HookPoint) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.ErrorCount[point]++
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks:              make(map[HookPoint][]hookEntry),
		metrics:            NewHookMetrics(),
		maxConcurrentHooks: 50, // Default max concurrent hooks
	}
}

// NewHookRegistryWithConfig creates a new hook registry with custom config.
func NewHookRegistryWithConfig(maxConcurrent int) *HookRegistry {
	if maxConcurrent <= 0 {
		maxConcurrent = 50
	}
	return &HookRegistry{
		hooks:              make(map[HookPoint][]hookEntry),
		metrics:            NewHookMetrics(),
		maxConcurrentHooks: maxConcurrent,
	}
}

// SetMaxConcurrentHooks sets the maximum number of concurrent async hooks.
func (r *HookRegistry) SetMaxConcurrentHooks(max int) {
	if max > 0 {
		r.maxConcurrentHooks = max
	}
}

// HookOption configures a hook registration.
type HookOption func(*hookEntry)

// WithPriority sets the hook priority (lower executes first).
func WithPriority(p int) HookOption {
	return func(e *hookEntry) {
		e.priority = p
	}
}

// WithAsync marks the hook as safe for parallel execution.
func WithAsync() HookOption {
	return func(e *hookEntry) {
		e.async = true
	}
}

// Register adds a hook to the registry.
func (r *HookRegistry) Register(point HookPoint, name string, fn HookFunc, opts ...HookOption) {
	entry := hookEntry{name: name, fn: fn, priority: 100}
	for _, opt := range opts {
		opt(&entry)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.hooks[point] = append(r.hooks[point], entry)

	// Sort by priority
	sort.Slice(r.hooks[point], func(i, j int) bool {
		return r.hooks[point][i].priority < r.hooks[point][j].priority
	})
}

// Unregister removes a hook by name.
func (r *HookRegistry) Unregister(point HookPoint, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	hooks := r.hooks[point]
	for i, h := range hooks {
		if h.name == name {
			r.hooks[point] = append(hooks[:i], hooks[i+1:]...)
			return
		}
	}
}

// Execute runs all hooks for a given point.
// Uses a worker pool to limit concurrent async hooks and prevent goroutine explosion.
func (r *HookRegistry) Execute(point HookPoint, ctx *HookContext) error {
	r.mu.RLock()
	hooks := make([]hookEntry, len(r.hooks[point]))
	copy(hooks, r.hooks[point])
	maxConcurrent := r.maxConcurrentHooks
	r.mu.RUnlock()

	if len(hooks) == 0 {
		return nil
	}

	start := time.Now()
	defer func() {
		r.metrics.RecordExecution(point, time.Since(start))
	}()

	// Separate sync and async hooks
	var syncHooks, asyncHooks []hookEntry
	for _, h := range hooks {
		if h.async {
			asyncHooks = append(asyncHooks, h)
		} else {
			syncHooks = append(syncHooks, h)
		}
	}

	// Execute async hooks with worker pool (limited concurrency)
	var wg sync.WaitGroup
	errChan := make(chan error, len(asyncHooks))
	sem := make(chan struct{}, maxConcurrent) // Semaphore for limiting concurrency

	for _, h := range asyncHooks {
		wg.Add(1)
		sem <- struct{}{} // Acquire slot

		go func(hook hookEntry) {
			defer func() {
				<-sem // Release slot
				wg.Done()
			}()

			// Recover from panics in async hooks
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("hook %s panicked: %v", hook.name, r)
				}
			}()

			if err := hook.fn(ctx); err != nil {
				errChan <- fmt.Errorf("hook %s: %w", hook.name, err)
			}
		}(h)
	}

	// Execute sync hooks sequentially
	for _, h := range syncHooks {
		if err := h.fn(ctx); err != nil {
			r.metrics.RecordError(point)
			return fmt.Errorf("hook %s: %w", h.name, err)
		}
	}

	// Wait for async hooks
	wg.Wait()
	close(errChan)

	// Collect async errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
		r.metrics.RecordError(point)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// ExecuteWithTimeout runs hooks with a timeout.
func (r *HookRegistry) ExecuteWithTimeout(point HookPoint, ctx *HookContext, timeout time.Duration) error {
	done := make(chan error, 1)

	go func() {
		done <- r.Execute(point, ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("hook execution timeout for %s", point)
	case <-ctx.Context.Done():
		return ctx.Context.Err()
	}
}

// HasHooks returns true if hooks are registered for the point.
func (r *HookRegistry) HasHooks(point HookPoint) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks[point]) > 0
}

// HookCount returns the number of hooks for a point.
func (r *HookRegistry) HookCount(point HookPoint) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks[point])
}

// Metrics returns the hook metrics.
func (r *HookRegistry) Metrics() *HookMetrics {
	return r.metrics
}

// Clear removes all hooks (useful for testing).
func (r *HookRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = make(map[HookPoint][]hookEntry)
}

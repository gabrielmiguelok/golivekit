// Package limits provides rate limiting and backpressure for GoliveKit.
package limits

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// Common errors.
var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrBackpressure      = errors.New("backpressure applied")
)

// RateLimiter limits the rate of operations.
type RateLimiter interface {
	// Allow returns true if the operation is allowed.
	Allow(key string) bool

	// AllowN returns true if n operations are allowed.
	AllowN(key string, n int) bool

	// Wait blocks until the operation is allowed or context is cancelled.
	Wait(ctx context.Context, key string) error
}

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	rate       float64        // Tokens per second
	burst      int            // Maximum tokens (bucket size)
	buckets    sync.Map       // key -> *bucket
	cleanupInt time.Duration  // Cleanup interval
}

type bucket struct {
	tokens   float64
	lastFill time.Time
	mu       sync.Mutex
}

// NewTokenBucket creates a new token bucket rate limiter.
func NewTokenBucket(rate float64, burst int) *TokenBucket {
	tb := &TokenBucket{
		rate:       rate,
		burst:      burst,
		cleanupInt: time.Minute,
	}

	// Start cleanup goroutine
	go tb.cleanupLoop()

	return tb
}

// Allow checks if an operation is allowed for the given key.
func (tb *TokenBucket) Allow(key string) bool {
	return tb.AllowN(key, 1)
}

// AllowN checks if n operations are allowed for the given key.
func (tb *TokenBucket) AllowN(key string, n int) bool {
	b := tb.getBucket(key)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * tb.rate
	if b.tokens > float64(tb.burst) {
		b.tokens = float64(tb.burst)
	}
	b.lastFill = now

	// Check if we have enough tokens
	if b.tokens >= float64(n) {
		b.tokens -= float64(n)
		return true
	}

	return false
}

// Wait blocks until an operation is allowed or context is cancelled.
func (tb *TokenBucket) Wait(ctx context.Context, key string) error {
	for {
		if tb.Allow(key) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 100):
			// Retry
		}
	}
}

func (tb *TokenBucket) getBucket(key string) *bucket {
	if b, ok := tb.buckets.Load(key); ok {
		return b.(*bucket)
	}

	newBucket := &bucket{
		tokens:   float64(tb.burst),
		lastFill: time.Now(),
	}

	actual, _ := tb.buckets.LoadOrStore(key, newBucket)
	return actual.(*bucket)
}

func (tb *TokenBucket) cleanupLoop() {
	ticker := time.NewTicker(tb.cleanupInt)
	defer ticker.Stop()

	for range ticker.C {
		// Remove buckets that haven't been used recently
		now := time.Now()
		tb.buckets.Range(func(key, value any) bool {
			b := value.(*bucket)
			b.mu.Lock()
			if now.Sub(b.lastFill) > time.Hour {
				tb.buckets.Delete(key)
			}
			b.mu.Unlock()
			return true
		})
	}
}

// SlidingWindow implements a sliding window rate limiter.
type SlidingWindow struct {
	limit      int
	window     time.Duration
	windows    sync.Map // key -> *windowState
}

type windowState struct {
	timestamps []time.Time
	mu         sync.Mutex
}

// NewSlidingWindow creates a new sliding window rate limiter.
func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		limit:  limit,
		window: window,
	}
}

// Allow checks if an operation is allowed.
func (sw *SlidingWindow) Allow(key string) bool {
	return sw.AllowN(key, 1)
}

// AllowN checks if n operations are allowed.
func (sw *SlidingWindow) AllowN(key string, n int) bool {
	ws := sw.getWindow(key)

	ws.mu.Lock()
	defer ws.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.window)

	// Remove old timestamps
	validTimestamps := make([]time.Time, 0)
	for _, ts := range ws.timestamps {
		if ts.After(cutoff) {
			validTimestamps = append(validTimestamps, ts)
		}
	}
	ws.timestamps = validTimestamps

	// Check limit
	if len(ws.timestamps)+n > sw.limit {
		return false
	}

	// Add new timestamps
	for i := 0; i < n; i++ {
		ws.timestamps = append(ws.timestamps, now)
	}

	return true
}

// Wait blocks until allowed or context cancelled.
func (sw *SlidingWindow) Wait(ctx context.Context, key string) error {
	for {
		if sw.Allow(key) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 100):
		}
	}
}

func (sw *SlidingWindow) getWindow(key string) *windowState {
	if ws, ok := sw.windows.Load(key); ok {
		return ws.(*windowState)
	}

	newWS := &windowState{
		timestamps: make([]time.Time, 0),
	}

	actual, _ := sw.windows.LoadOrStore(key, newWS)
	return actual.(*windowState)
}

// BackpressureConfig configures backpressure behavior.
type BackpressureConfig struct {
	// MaxPendingMessages is the maximum messages in queue
	MaxPendingMessages int

	// MaxPendingEvents is the maximum events waiting to be processed
	MaxPendingEvents int

	// Action is what to do when limits are exceeded
	Action SlowConsumerAction

	// GracePeriod is how long to wait before applying action
	GracePeriod time.Duration
}

// SlowConsumerAction defines what to do with slow consumers.
type SlowConsumerAction int

const (
	// ActionDrop drops messages for slow consumers.
	ActionDrop SlowConsumerAction = iota

	// ActionBlock blocks sending to slow consumers.
	ActionBlock

	// ActionDisconnect disconnects slow consumers.
	ActionDisconnect
)

// DefaultBackpressureConfig returns default backpressure configuration.
func DefaultBackpressureConfig() BackpressureConfig {
	return BackpressureConfig{
		MaxPendingMessages: 1000,
		MaxPendingEvents:   100,
		Action:             ActionDrop,
		GracePeriod:        time.Second * 5,
	}
}

// Backpressure manages backpressure for connections.
type Backpressure struct {
	config    BackpressureConfig
	consumers sync.Map // id -> *consumerState
}

type consumerState struct {
	pendingMessages int64
	pendingEvents   int64
	slowSince       time.Time
	mu              sync.Mutex
}

// NewBackpressure creates a new backpressure manager.
func NewBackpressure(config BackpressureConfig) *Backpressure {
	return &Backpressure{config: config}
}

// CanSend returns true if a message can be sent to the consumer.
func (bp *Backpressure) CanSend(consumerID string) (bool, SlowConsumerAction) {
	cs := bp.getConsumer(consumerID)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.pendingMessages < int64(bp.config.MaxPendingMessages) {
		cs.pendingMessages++
		cs.slowSince = time.Time{} // Reset slow timer
		return true, 0
	}

	// Consumer is slow
	if cs.slowSince.IsZero() {
		cs.slowSince = time.Now()
	}

	// Check if grace period has passed
	if time.Since(cs.slowSince) < bp.config.GracePeriod {
		return false, ActionBlock // Still in grace period
	}

	return false, bp.config.Action
}

// MessageSent marks a message as sent.
func (bp *Backpressure) MessageSent(consumerID string) {
	cs := bp.getConsumer(consumerID)
	cs.mu.Lock()
	cs.pendingMessages++
	cs.mu.Unlock()
}

// MessageAcked marks a message as acknowledged.
func (bp *Backpressure) MessageAcked(consumerID string) {
	cs := bp.getConsumer(consumerID)
	cs.mu.Lock()
	if cs.pendingMessages > 0 {
		cs.pendingMessages--
	}
	cs.mu.Unlock()
}

// RemoveConsumer removes a consumer's state.
func (bp *Backpressure) RemoveConsumer(consumerID string) {
	bp.consumers.Delete(consumerID)
}

func (bp *Backpressure) getConsumer(id string) *consumerState {
	if cs, ok := bp.consumers.Load(id); ok {
		return cs.(*consumerState)
	}

	newCS := &consumerState{}
	actual, _ := bp.consumers.LoadOrStore(id, newCS)
	return actual.(*consumerState)
}

// RateLimitMiddleware returns HTTP middleware for rate limiting.
func RateLimitMiddleware(limiter RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)

			if !limiter.Allow(key) {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IPKeyFunc returns a key function that uses client IP.
func IPKeyFunc(r *http.Request) string {
	// Check X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

// PathKeyFunc returns a key function that uses the request path.
func PathKeyFunc(r *http.Request) string {
	return r.URL.Path
}

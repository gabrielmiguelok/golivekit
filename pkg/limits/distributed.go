// Package limits provides rate limiting and connection limiting for GoliveKit.
package limits

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Common errors for distributed rate limiting.
var (
	ErrDistributedRateLimitExceeded = errors.New("distributed rate limit exceeded")
	ErrRedisUnavailable             = errors.New("redis unavailable")
)

// DistributedRateLimiter is the interface for distributed rate limiting.
// Implementations can use Redis, Memcached, or other distributed stores.
type DistributedRateLimiter interface {
	// Allow checks if a request is allowed under the rate limit.
	// Returns true if allowed, false if rate limit exceeded.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)

	// AllowN checks if N requests are allowed.
	AllowN(ctx context.Context, key string, n, limit int, window time.Duration) (bool, error)

	// Reset resets the rate limit for a key.
	Reset(ctx context.Context, key string) error

	// Close closes the rate limiter.
	Close() error
}

// MemoryRateLimiter is an in-memory sliding window rate limiter.
// Suitable for single-node deployments.
type MemoryRateLimiter struct {
	windows map[string]*slidingWindow
	mu      sync.RWMutex
}

type slidingWindow struct {
	requests []int64 // Timestamps of requests
	mu       sync.Mutex
}

// NewMemoryRateLimiter creates a new in-memory rate limiter.
func NewMemoryRateLimiter() *MemoryRateLimiter {
	rl := &MemoryRateLimiter{
		windows: make(map[string]*slidingWindow),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request is allowed.
func (rl *MemoryRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	return rl.AllowN(ctx, key, 1, limit, window)
}

// AllowN checks if N requests are allowed.
func (rl *MemoryRateLimiter) AllowN(ctx context.Context, key string, n, limit int, window time.Duration) (bool, error) {
	rl.mu.Lock()
	w, exists := rl.windows[key]
	if !exists {
		w = &slidingWindow{
			requests: make([]int64, 0, limit),
		}
		rl.windows[key] = w
	}
	rl.mu.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now().UnixNano()
	windowStart := now - int64(window)

	// Remove expired requests (sliding window)
	newRequests := make([]int64, 0, len(w.requests))
	for _, ts := range w.requests {
		if ts > windowStart {
			newRequests = append(newRequests, ts)
		}
	}
	w.requests = newRequests

	// Check if adding N requests would exceed limit
	if len(w.requests)+n > limit {
		return false, nil
	}

	// Add the new requests
	for i := 0; i < n; i++ {
		w.requests = append(w.requests, now)
	}

	return true, nil
}

// Reset resets the rate limit for a key.
func (rl *MemoryRateLimiter) Reset(ctx context.Context, key string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.windows, key)
	return nil
}

// Close closes the rate limiter.
func (rl *MemoryRateLimiter) Close() error {
	return nil
}

// cleanup periodically removes old entries.
func (rl *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now().UnixNano()
		oneHourAgo := now - int64(time.Hour)

		for key, w := range rl.windows {
			w.mu.Lock()
			// Check if all requests are older than 1 hour
			if len(w.requests) == 0 || w.requests[len(w.requests)-1] < oneHourAgo {
				delete(rl.windows, key)
			}
			w.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// GetCount returns the current request count for a key.
func (rl *MemoryRateLimiter) GetCount(key string, window time.Duration) int {
	rl.mu.RLock()
	w, exists := rl.windows[key]
	rl.mu.RUnlock()

	if !exists {
		return 0
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now().UnixNano()
	windowStart := now - int64(window)

	count := 0
	for _, ts := range w.requests {
		if ts > windowStart {
			count++
		}
	}

	return count
}

// RateLimitConfig configures rate limiting.
type RateLimitConfig struct {
	// Requests per window
	Limit int

	// Time window for the limit
	Window time.Duration

	// Key function extracts the rate limit key from context
	KeyFunc func(ctx context.Context) string

	// OnLimitExceeded is called when rate limit is exceeded
	OnLimitExceeded func(ctx context.Context, key string)
}

// DefaultRateLimitConfig returns default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Limit:  100,
		Window: time.Minute,
		KeyFunc: func(ctx context.Context) string {
			return "global"
		},
	}
}

// MessageRateLimitConfig returns rate limit config for WebSocket messages.
func MessageRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Limit:  100, // 100 messages per second
		Window: time.Second,
	}
}

// SimpleTokenBucket implements a simple in-memory token bucket for testing.
// For production with keyed rate limiting, use TokenBucket from ratelimit.go.
type SimpleTokenBucket struct {
	rate       float64 // tokens per second
	bucketSize int     // maximum tokens
	tokens     float64
	lastFill   time.Time
	mu         sync.Mutex
}

// NewSimpleTokenBucket creates a simple token bucket.
func NewSimpleTokenBucket(rate float64, bucketSize int) *SimpleTokenBucket {
	return &SimpleTokenBucket{
		rate:       rate,
		bucketSize: bucketSize,
		tokens:     float64(bucketSize),
		lastFill:   time.Now(),
	}
}

// Allow checks if a request is allowed.
func (tb *SimpleTokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN checks if N requests are allowed.
func (tb *SimpleTokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastFill).Seconds()
	tb.lastFill = now

	// Refill tokens
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.bucketSize) {
		tb.tokens = float64(tb.bucketSize)
	}

	// Check if we have enough tokens
	if tb.tokens < float64(n) {
		return false
	}

	tb.tokens -= float64(n)
	return true
}

// Available returns the number of available tokens.
func (tb *SimpleTokenBucket) Available() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastFill).Seconds()

	tokens := tb.tokens + elapsed*tb.rate
	if tokens > float64(tb.bucketSize) {
		tokens = float64(tb.bucketSize)
	}

	return int(tokens)
}

// SimpleLeakyBucket implements a simple leaky bucket rate limiter.
type SimpleLeakyBucket struct {
	rate     float64 // leak rate (requests per second)
	capacity int     // bucket capacity
	water    float64 // current water level
	lastLeak time.Time
	mu       sync.Mutex
}

// NewSimpleLeakyBucket creates a new leaky bucket rate limiter.
func NewSimpleLeakyBucket(rate float64, capacity int) *SimpleLeakyBucket {
	return &SimpleLeakyBucket{
		rate:     rate,
		capacity: capacity,
		water:    0,
		lastLeak: time.Now(),
	}
}

// Allow checks if a request is allowed.
func (lb *SimpleLeakyBucket) Allow() bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(lb.lastLeak).Seconds()
	lb.lastLeak = now

	// Leak water
	lb.water -= elapsed * lb.rate
	if lb.water < 0 {
		lb.water = 0
	}

	// Check if bucket has room
	if lb.water >= float64(lb.capacity) {
		return false
	}

	lb.water++
	return true
}

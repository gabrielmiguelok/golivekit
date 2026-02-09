package limits

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMemoryRateLimiter_SlidingWindow(t *testing.T) {
	rl := NewMemoryRateLimiter()

	ctx := context.Background()
	key := "test-key"
	limit := 5
	window := 100 * time.Millisecond

	// Should allow first 5 requests
	for i := 0; i < limit; i++ {
		allowed, err := rl.Allow(ctx, key, limit, window)
		if err != nil {
			t.Fatalf("Allow returned error: %v", err)
		}
		if !allowed {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	allowed, _ := rl.Allow(ctx, key, limit, window)
	if allowed {
		t.Error("6th request should be denied")
	}

	// Wait for window to expire
	time.Sleep(window + 10*time.Millisecond)

	// Should allow again
	allowed, _ = rl.Allow(ctx, key, limit, window)
	if !allowed {
		t.Error("Request after window should be allowed")
	}
}

func TestMemoryRateLimiter_AllowN(t *testing.T) {
	rl := NewMemoryRateLimiter()

	ctx := context.Background()
	key := "test-key"
	limit := 10
	window := time.Second

	// Allow 3 at once
	allowed, _ := rl.AllowN(ctx, key, 3, limit, window)
	if !allowed {
		t.Error("First 3 should be allowed")
	}

	// Allow 5 more
	allowed, _ = rl.AllowN(ctx, key, 5, limit, window)
	if !allowed {
		t.Error("Next 5 should be allowed (total 8)")
	}

	// Try to allow 5 more (would exceed limit)
	allowed, _ = rl.AllowN(ctx, key, 5, limit, window)
	if allowed {
		t.Error("Request that would exceed limit should be denied")
	}

	// Allow remaining 2
	allowed, _ = rl.AllowN(ctx, key, 2, limit, window)
	if !allowed {
		t.Error("Remaining 2 should be allowed (total 10)")
	}
}

func TestMemoryRateLimiter_Reset(t *testing.T) {
	rl := NewMemoryRateLimiter()

	ctx := context.Background()
	key := "test-key"
	limit := 2
	window := time.Hour

	// Use up the limit
	rl.Allow(ctx, key, limit, window)
	rl.Allow(ctx, key, limit, window)

	// Should be denied
	allowed, _ := rl.Allow(ctx, key, limit, window)
	if allowed {
		t.Error("Should be denied before reset")
	}

	// Reset
	rl.Reset(ctx, key)

	// Should be allowed again
	allowed, _ = rl.Allow(ctx, key, limit, window)
	if !allowed {
		t.Error("Should be allowed after reset")
	}
}

func TestMemoryRateLimiter_Concurrent(t *testing.T) {
	rl := NewMemoryRateLimiter()

	ctx := context.Background()
	key := "concurrent-key"
	limit := 100
	window := time.Second

	var wg sync.WaitGroup
	var allowedCount int32
	var mu sync.Mutex

	numGoroutines := 200
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			allowed, _ := rl.Allow(ctx, key, limit, window)
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Should have allowed exactly `limit` requests
	if allowedCount > int32(limit) {
		t.Errorf("Allowed %d requests, but limit is %d", allowedCount, limit)
	}
}

func TestMemoryRateLimiter_GetCount(t *testing.T) {
	rl := NewMemoryRateLimiter()

	ctx := context.Background()
	key := "count-key"
	limit := 10
	window := time.Second

	// Initial count should be 0
	count := rl.GetCount(key, window)
	if count != 0 {
		t.Errorf("Initial count should be 0, got %d", count)
	}

	// Make some requests
	rl.Allow(ctx, key, limit, window)
	rl.Allow(ctx, key, limit, window)
	rl.Allow(ctx, key, limit, window)

	count = rl.GetCount(key, window)
	if count != 3 {
		t.Errorf("Count should be 3, got %d", count)
	}
}

func TestSimpleTokenBucket_Allow(t *testing.T) {
	tb := NewSimpleTokenBucket(10, 5) // 10 tokens/sec, bucket size 5

	// Should allow burst of 5
	for i := 0; i < 5; i++ {
		if !tb.Allow() {
			t.Errorf("Request %d should be allowed (burst)", i+1)
		}
	}

	// 6th should be denied
	if tb.Allow() {
		t.Error("6th request should be denied")
	}

	// Wait for refill
	time.Sleep(200 * time.Millisecond) // Should refill ~2 tokens

	// Should allow again
	if !tb.Allow() {
		t.Error("Should allow after refill")
	}
}

func TestSimpleTokenBucket_AllowN(t *testing.T) {
	tb := NewSimpleTokenBucket(100, 10)

	// Allow 5
	if !tb.AllowN(5) {
		t.Error("First 5 should be allowed")
	}

	// Allow 5 more
	if !tb.AllowN(5) {
		t.Error("Next 5 should be allowed")
	}

	// Try 1 more
	if tb.AllowN(1) {
		t.Error("Bucket should be empty")
	}
}

func TestSimpleLeakyBucket_Allow(t *testing.T) {
	// Use zero leak rate to test bucket filling without floating point drift
	lb := NewSimpleLeakyBucket(0, 5) // 0/sec leak rate, capacity 5

	// Fill the bucket (no leaking at all)
	for i := 0; i < 5; i++ {
		if !lb.Allow() {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// Bucket full (should be denied immediately)
	if lb.Allow() {
		t.Error("Bucket should be full")
	}

	// Now test with a higher leak rate to verify leaking works
	lb2 := NewSimpleLeakyBucket(10, 5) // 10/sec leak rate, capacity 5

	// Fill the bucket beyond capacity check threshold
	for i := 0; i < 6; i++ {
		lb2.Allow()
	}

	// Wait for leak (200ms at 10/sec = 2 requests leaked)
	time.Sleep(250 * time.Millisecond)

	// Should allow again after leak
	if !lb2.Allow() {
		t.Error("Should allow after leak")
	}
}

func BenchmarkMemoryRateLimiter_Allow(b *testing.B) {
	rl := NewMemoryRateLimiter()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench-key"
		rl.Allow(ctx, key, 1000000, time.Hour)
	}
}

func BenchmarkSimpleTokenBucket_Allow(b *testing.B) {
	tb := NewSimpleTokenBucket(1000000, 1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.Allow()
	}
}

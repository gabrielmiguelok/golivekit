package state

import (
	"context"
	"path/filepath"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of Store.
// Suitable for single-node deployments and testing.
type MemoryStore struct {
	items    map[string]*memoryItem
	mu       sync.RWMutex
	closed   bool
	cleanupCh chan struct{}
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	ms := &MemoryStore{
		items:     make(map[string]*memoryItem),
		cleanupCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	go ms.cleanupLoop()

	return ms
}

// Get retrieves a value.
func (ms *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.closed {
		return nil, ErrStoreClosed
	}

	item, ok := ms.items[key]
	if !ok {
		return nil, ErrKeyNotFound
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return nil, ErrKeyNotFound
	}

	// Return a copy
	result := make([]byte, len(item.value))
	copy(result, item.value)

	return result, nil
}

// Set stores a value.
func (ms *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.closed {
		return ErrStoreClosed
	}

	// Make a copy of the value
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	item := &memoryItem{
		value: valueCopy,
	}

	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}

	ms.items[key] = item
	return nil
}

// Delete removes a key.
func (ms *MemoryStore) Delete(ctx context.Context, key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.closed {
		return ErrStoreClosed
	}

	delete(ms.items, key)
	return nil
}

// Exists checks if a key exists.
func (ms *MemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.closed {
		return false, ErrStoreClosed
	}

	item, ok := ms.items[key]
	if !ok {
		return false, nil
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return false, nil
	}

	return true, nil
}

// Keys returns keys matching a pattern.
// Supports basic glob patterns: * matches any sequence.
func (ms *MemoryStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.closed {
		return nil, ErrStoreClosed
	}

	var keys []string
	now := time.Now()

	for key, item := range ms.items {
		// Skip expired items
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			continue
		}

		// Match pattern
		matched, err := filepath.Match(pattern, key)
		if err != nil {
			continue
		}
		if matched {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Close closes the store.
func (ms *MemoryStore) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.closed {
		return nil
	}

	ms.closed = true
	close(ms.cleanupCh)

	return nil
}

// cleanupLoop periodically removes expired items.
func (ms *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.cleanup()
		case <-ms.cleanupCh:
			return
		}
	}
}

// cleanup removes expired items.
func (ms *MemoryStore) cleanup() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	for key, item := range ms.items {
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			delete(ms.items, key)
		}
	}
}

// Len returns the number of items in the store.
func (ms *MemoryStore) Len() int {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return len(ms.items)
}

// Clear removes all items from the store.
func (ms *MemoryStore) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.items = make(map[string]*memoryItem)
}

// Stats returns store statistics.
func (ms *MemoryStore) Stats() MemoryStoreStats {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	stats := MemoryStoreStats{
		ItemCount: len(ms.items),
	}

	now := time.Now()
	for _, item := range ms.items {
		stats.TotalSize += int64(len(item.value))
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			stats.ExpiredCount++
		}
	}

	return stats
}

// MemoryStoreStats contains memory store statistics.
type MemoryStoreStats struct {
	ItemCount    int
	ExpiredCount int
	TotalSize    int64
}

// Snapshot creates a snapshot of the store for debugging.
func (ms *MemoryStore) Snapshot() map[string][]byte {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	snapshot := make(map[string][]byte, len(ms.items))
	now := time.Now()

	for key, item := range ms.items {
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			continue
		}

		valueCopy := make([]byte, len(item.value))
		copy(valueCopy, item.value)
		snapshot[key] = valueCopy
	}

	return snapshot
}

// Restore loads a snapshot into the store.
func (ms *MemoryStore) Restore(snapshot map[string][]byte) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for key, value := range snapshot {
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		ms.items[key] = &memoryItem{value: valueCopy}
	}
}

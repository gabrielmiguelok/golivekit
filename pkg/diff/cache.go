package diff

import (
	"container/list"
	"sync"
	"time"
)

// Cache provides an LRU cache for rendered content.
type Cache struct {
	capacity  int
	items     map[string]*cacheItem
	order     *list.List
	mu        sync.RWMutex
	stats     *CacheStats
	ttl       time.Duration
	cleanupCh chan struct{}
}

type cacheItem struct {
	key       string
	value     []byte
	hash      uint64
	element   *list.Element
	createdAt time.Time
	accessedAt time.Time
	accessCount int64
}

// CacheStats tracks cache statistics.
type CacheStats struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	Size       int64 // Total bytes
	ItemCount  int64
	mu         sync.Mutex
}

// NewCache creates a new cache with the given capacity.
func NewCache(capacity int, ttl time.Duration) *Cache {
	c := &Cache{
		capacity:  capacity,
		items:     make(map[string]*cacheItem),
		order:     list.New(),
		stats:     &CacheStats{},
		ttl:       ttl,
		cleanupCh: make(chan struct{}),
	}

	// Start cleanup goroutine if TTL is set
	if ttl > 0 {
		go c.cleanupLoop()
	}

	return c
}

// Get retrieves a value from the cache.
func (c *Cache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		c.stats.mu.Lock()
		c.stats.Misses++
		c.stats.mu.Unlock()
		return nil, false
	}

	// Check TTL
	if c.ttl > 0 && time.Since(item.createdAt) > c.ttl {
		c.removeItem(item)
		c.stats.mu.Lock()
		c.stats.Misses++
		c.stats.mu.Unlock()
		return nil, false
	}

	// Move to front (most recently used)
	c.order.MoveToFront(item.element)
	item.accessedAt = time.Now()
	item.accessCount++

	c.stats.mu.Lock()
	c.stats.Hits++
	c.stats.mu.Unlock()

	return item.value, true
}

// GetHash retrieves the hash of a cached value without the content.
func (c *Cache) GetHash(key string) (uint64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return 0, false
	}

	if c.ttl > 0 && time.Since(item.createdAt) > c.ttl {
		return 0, false
	}

	return item.hash, true
}

// Set stores a value in the cache.
func (c *Cache) Set(key string, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if item, ok := c.items[key]; ok {
		c.stats.mu.Lock()
		c.stats.Size -= int64(len(item.value))
		c.stats.mu.Unlock()

		item.value = value
		item.hash = hashBytes(value)
		item.createdAt = time.Now()
		item.accessedAt = time.Now()
		c.order.MoveToFront(item.element)

		c.stats.mu.Lock()
		c.stats.Size += int64(len(value))
		c.stats.mu.Unlock()
		return
	}

	// Evict if at capacity
	for c.order.Len() >= c.capacity {
		c.evictOldest()
	}

	// Add new item
	item := &cacheItem{
		key:        key,
		value:      value,
		hash:       hashBytes(value),
		createdAt:  time.Now(),
		accessedAt: time.Now(),
	}
	item.element = c.order.PushFront(item)
	c.items[key] = item

	c.stats.mu.Lock()
	c.stats.Size += int64(len(value))
	c.stats.ItemCount++
	c.stats.mu.Unlock()
}

// SetWithHash stores a value with a pre-computed hash.
func (c *Cache) SetWithHash(key string, value []byte, hash uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.items[key]; ok {
		c.stats.mu.Lock()
		c.stats.Size -= int64(len(item.value))
		c.stats.mu.Unlock()

		item.value = value
		item.hash = hash
		item.createdAt = time.Now()
		item.accessedAt = time.Now()
		c.order.MoveToFront(item.element)

		c.stats.mu.Lock()
		c.stats.Size += int64(len(value))
		c.stats.mu.Unlock()
		return
	}

	for c.order.Len() >= c.capacity {
		c.evictOldest()
	}

	item := &cacheItem{
		key:        key,
		value:      value,
		hash:       hash,
		createdAt:  time.Now(),
		accessedAt: time.Now(),
	}
	item.element = c.order.PushFront(item)
	c.items[key] = item

	c.stats.mu.Lock()
	c.stats.Size += int64(len(value))
	c.stats.ItemCount++
	c.stats.mu.Unlock()
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, ok := c.items[key]; ok {
		c.removeItem(item)
	}
}

// removeItem removes an item (must hold lock).
func (c *Cache) removeItem(item *cacheItem) {
	c.order.Remove(item.element)
	delete(c.items, item.key)

	c.stats.mu.Lock()
	c.stats.Size -= int64(len(item.value))
	c.stats.ItemCount--
	c.stats.mu.Unlock()
}

// evictOldest removes the least recently used item (must hold lock).
func (c *Cache) evictOldest() {
	elem := c.order.Back()
	if elem == nil {
		return
	}

	item := elem.Value.(*cacheItem)
	c.removeItem(item)

	c.stats.mu.Lock()
	c.stats.Evictions++
	c.stats.mu.Unlock()
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
	c.order.Init()

	c.stats.mu.Lock()
	c.stats.Size = 0
	c.stats.ItemCount = 0
	c.stats.mu.Unlock()
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Stats returns cache statistics.
func (c *Cache) Stats() CacheStats {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	return CacheStats{
		Hits:      c.stats.Hits,
		Misses:    c.stats.Misses,
		Evictions: c.stats.Evictions,
		Size:      c.stats.Size,
		ItemCount: c.stats.ItemCount,
	}
}

// HitRate returns the cache hit rate.
func (c *Cache) HitRate() float64 {
	c.stats.mu.Lock()
	defer c.stats.mu.Unlock()

	total := c.stats.Hits + c.stats.Misses
	if total == 0 {
		return 0
	}
	return float64(c.stats.Hits) / float64(total)
}

// cleanupLoop periodically removes expired items.
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.cleanupCh:
			return
		}
	}
}

// cleanupExpired removes all expired items.
func (c *Cache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var toRemove []*cacheItem

	for _, item := range c.items {
		if now.Sub(item.createdAt) > c.ttl {
			toRemove = append(toRemove, item)
		}
	}

	for _, item := range toRemove {
		c.removeItem(item)
	}
}

// Close stops the cleanup goroutine.
func (c *Cache) Close() {
	if c.ttl > 0 {
		close(c.cleanupCh)
	}
}

// Contains checks if a key exists in the cache.
func (c *Cache) Contains(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return false
	}

	if c.ttl > 0 && time.Since(item.createdAt) > c.ttl {
		return false
	}

	return true
}

// Keys returns all cache keys.
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key := range c.items {
		keys = append(keys, key)
	}
	return keys
}

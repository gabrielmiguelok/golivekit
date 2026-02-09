// Package pool provides memory pooling utilities for GoliveKit.
// It reduces GC pressure by reusing allocations for hot paths.
package pool

import (
	"bytes"
	"sync"
)

// BufferPool is a pool of bytes.Buffer for reducing allocations.
var BufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// GetBuffer retrieves a buffer from the pool, resetting it for use.
func GetBuffer() *bytes.Buffer {
	buf := BufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutBuffer returns a buffer to the pool.
// Buffers larger than 64KB are discarded to avoid holding too much memory.
func PutBuffer(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	// Don't recycle buffers larger than 64KB to avoid memory bloat
	if buf.Cap() > 64*1024 {
		return
	}
	BufferPool.Put(buf)
}

// ByteSlicePool pools byte slices of common sizes.
type ByteSlicePool struct {
	small  sync.Pool // 1KB
	medium sync.Pool // 8KB
	large  sync.Pool // 64KB
}

// DefaultByteSlicePool is the default byte slice pool.
var DefaultByteSlicePool = NewByteSlicePool()

// NewByteSlicePool creates a new byte slice pool.
func NewByteSlicePool() *ByteSlicePool {
	return &ByteSlicePool{
		small: sync.Pool{
			New: func() any {
				b := make([]byte, 1024)
				return &b
			},
		},
		medium: sync.Pool{
			New: func() any {
				b := make([]byte, 8*1024)
				return &b
			},
		},
		large: sync.Pool{
			New: func() any {
				b := make([]byte, 64*1024)
				return &b
			},
		},
	}
}

// Get retrieves a byte slice of at least the requested size.
func (p *ByteSlicePool) Get(size int) []byte {
	if size <= 1024 {
		buf := p.small.Get().(*[]byte)
		return (*buf)[:size]
	}
	if size <= 8*1024 {
		buf := p.medium.Get().(*[]byte)
		return (*buf)[:size]
	}
	if size <= 64*1024 {
		buf := p.large.Get().(*[]byte)
		return (*buf)[:size]
	}
	// Too large for pool, allocate directly
	return make([]byte, size)
}

// Put returns a byte slice to the pool.
func (p *ByteSlicePool) Put(b []byte) {
	if b == nil {
		return
	}
	cap := cap(b)
	if cap == 1024 {
		p.small.Put(&b)
	} else if cap == 8*1024 {
		p.medium.Put(&b)
	} else if cap == 64*1024 {
		p.large.Put(&b)
	}
	// Other sizes are discarded
}

// GetBytes retrieves a byte slice from the default pool.
func GetBytes(size int) []byte {
	return DefaultByteSlicePool.Get(size)
}

// PutBytes returns a byte slice to the default pool.
func PutBytes(b []byte) {
	DefaultByteSlicePool.Put(b)
}

// StringBuilderPool pools strings.Builder instances.
var StringBuilderPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// RingBuffer is a fixed-size circular buffer for messages.
type RingBuffer[T any] struct {
	data  []T
	head  int
	tail  int
	count int
	cap   int
	mu    sync.Mutex
}

// NewRingBuffer creates a new ring buffer with the given capacity.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	return &RingBuffer[T]{
		data: make([]T, capacity),
		cap:  capacity,
	}
}

// Push adds an item to the buffer. If full, overwrites the oldest item.
func (rb *RingBuffer[T]) Push(item T) (overwritten bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == rb.cap {
		// Buffer full, overwrite oldest
		rb.data[rb.tail] = item
		rb.tail = (rb.tail + 1) % rb.cap
		rb.head = (rb.head + 1) % rb.cap
		return true
	}

	rb.data[rb.tail] = item
	rb.tail = (rb.tail + 1) % rb.cap
	rb.count++
	return false
}

// Pop removes and returns the oldest item.
func (rb *RingBuffer[T]) Pop() (T, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var zero T
	if rb.count == 0 {
		return zero, false
	}

	item := rb.data[rb.head]
	rb.head = (rb.head + 1) % rb.cap
	rb.count--
	return item, true
}

// Peek returns the oldest item without removing it.
func (rb *RingBuffer[T]) Peek() (T, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	var zero T
	if rb.count == 0 {
		return zero, false
	}

	return rb.data[rb.head], true
}

// Len returns the number of items in the buffer.
func (rb *RingBuffer[T]) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

// Cap returns the buffer capacity.
func (rb *RingBuffer[T]) Cap() int {
	return rb.cap
}

// IsFull returns true if the buffer is full.
func (rb *RingBuffer[T]) IsFull() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count == rb.cap
}

// IsEmpty returns true if the buffer is empty.
func (rb *RingBuffer[T]) IsEmpty() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count == 0
}

// Clear empties the buffer.
func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.tail = 0
	rb.count = 0
}

// Drain returns all items and clears the buffer.
func (rb *RingBuffer[T]) Drain() []T {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return nil
	}

	result := make([]T, rb.count)
	for i := 0; i < rb.count; i++ {
		idx := (rb.head + i) % rb.cap
		result[i] = rb.data[idx]
	}

	rb.head = 0
	rb.tail = 0
	rb.count = 0

	return result
}

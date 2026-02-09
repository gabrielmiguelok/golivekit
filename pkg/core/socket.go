package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Common socket errors.
var (
	ErrSocketClosed   = errors.New("socket is closed")
	ErrSocketNotFound = errors.New("socket not found")
	ErrSendFailed     = errors.New("failed to send message")
	ErrInvalidMessage = errors.New("invalid message format")
)

// Socket represents a WebSocket connection to a client.
// It provides methods for sending messages and managing connection state.
type Socket struct {
	// Unique identifier for this socket
	id string

	// Connection state
	connected   bool
	connectedAt time.Time

	// lastActivity as atomic int64 (Unix nanoseconds) to avoid race conditions
	lastActivity atomic.Int64

	// Component state
	assigns *Assigns
	uploads map[string]*Upload

	// Transport layer (WebSocket, SSE, etc.)
	transport Transport

	// Subscriptions to topics
	subscriptions map[string]bool

	// Metadata
	metadata map[string]any

	// Error count for circuit breaker
	errorCount int

	// Mutex for thread safety (not used for lastActivity anymore)
	mu sync.RWMutex
}

// Transport is the interface for underlying connection transports.
type Transport interface {
	Send(msg Message) error
	Close() error
	IsConnected() bool
}

// Message represents a message sent over the socket.
type Message struct {
	Ref     string         `json:"ref,omitempty"`
	Topic   string         `json:"topic"`
	Event   string         `json:"event"`
	Payload map[string]any `json:"payload,omitempty"`
}

// Upload represents a file upload in progress.
type Upload struct {
	Config  UploadConfig
	Entries []UploadEntry
}

// UploadConfig configures upload behavior.
type UploadConfig struct {
	Name        string
	Accept      []string
	MaxFileSize int64
	MaxEntries  int
	AutoUpload  bool
}

// UploadEntry represents a single file being uploaded.
type UploadEntry struct {
	UUID        string
	FileName    string
	Size        int64
	ContentType string
	Progress    int
	Errors      []string
	Done        bool
	URL         string
}

// NewSocket creates a new socket with the given ID and transport.
func NewSocket(id string, transport Transport) *Socket {
	now := time.Now()
	s := &Socket{
		id:            id,
		connected:     true,
		connectedAt:   now,
		assigns:       NewAssigns(),
		uploads:       make(map[string]*Upload),
		subscriptions: make(map[string]bool),
		metadata:      make(map[string]any),
		transport:     transport,
	}
	s.lastActivity.Store(now.UnixNano())
	return s
}

// ID returns the socket's unique identifier.
func (s *Socket) ID() string {
	return s.id
}

// IsConnected returns true if the socket is connected.
func (s *Socket) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected && s.transport != nil && s.transport.IsConnected()
}

// ConnectedAt returns when the socket connected.
func (s *Socket) ConnectedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connectedAt
}

// LastActivity returns the time of last activity.
func (s *Socket) LastActivity() time.Time {
	return time.Unix(0, s.lastActivity.Load())
}

// UpdateActivity updates the last activity timestamp.
func (s *Socket) UpdateActivity() {
	s.lastActivity.Store(time.Now().UnixNano())
}

// Assigns returns the socket's assigns store.
func (s *Socket) Assigns() *Assigns {
	return s.assigns
}

// Uploads returns the socket's upload configurations.
func (s *Socket) Uploads() map[string]*Upload {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.uploads
}

// Send sends a message to the client.
// This method is thread-safe and uses atomic operations for lastActivity.
// Protected against race condition with Close() by verifying transport state.
func (s *Socket) Send(msg Message) error {
	s.mu.RLock()
	connected := s.connected
	transport := s.transport
	s.mu.RUnlock()

	// Early exit if socket is closed or transport is nil
	if !connected || transport == nil {
		return ErrSocketClosed
	}

	// Verify transport is still connected (protects against Close() race)
	// This check is not under lock, but IsConnected() is thread-safe
	if !transport.IsConnected() {
		return ErrSocketClosed
	}

	// Update activity atomically - no lock needed
	s.lastActivity.Store(time.Now().UnixNano())

	// Send the message. If Close() happens concurrently, transport.Send()
	// will fail gracefully (transport implementations must handle this)
	if err := transport.Send(msg); err != nil {
		// Re-check if socket was closed - return appropriate error
		s.mu.RLock()
		stillConnected := s.connected
		s.mu.RUnlock()
		if !stillConnected {
			return ErrSocketClosed
		}
		return fmt.Errorf("%w: %v", ErrSendFailed, err)
	}

	return nil
}

// Push sends an event to the client.
func (s *Socket) Push(event string, payload map[string]any) error {
	return s.Send(Message{
		Topic:   "lv:" + s.id,
		Event:   event,
		Payload: payload,
	})
}

// PushEvent is an alias for Push.
func (s *Socket) PushEvent(event string, payload map[string]any) error {
	return s.Push(event, payload)
}

// ListOp represents a single list operation for the client.
// Used in DiffPayload.ListOps for efficient list updates.
type ListOp struct {
	Op      string `json:"o"`           // "i"=insert, "d"=delete, "m"=move, "u"=update
	Key     string `json:"k"`           // Unique key of the item
	Index   int    `json:"i,omitempty"` // Position (for insert/move)
	Content string `json:"c,omitempty"` // HTML content (for insert/update)
}

// DiffPayload is the optimized diff format sent to clients.
// Supports text slots (s), HTML slots (h), list operations (l), and full render (f).
type DiffPayload struct {
	Version   uint64              `json:"v"`           // Version for ordering
	Slots     map[string]string   `json:"s,omitempty"` // Text-only slots (fast path)
	HTMLSlots map[string]string   `json:"h,omitempty"` // HTML slots (innerHTML)
	ListOps   map[string][]ListOp `json:"l,omitempty"` // List operations
	Full      string              `json:"f,omitempty"` // Full render (fallback)
}

// IsEmpty returns true if the payload has no changes.
func (d *DiffPayload) IsEmpty() bool {
	return len(d.Slots) == 0 &&
		len(d.HTMLSlots) == 0 &&
		len(d.ListOps) == 0 &&
		d.Full == ""
}

// Size returns the total size of the payload in bytes.
func (d *DiffPayload) Size() int {
	size := 0
	for _, content := range d.Slots {
		size += len(content)
	}
	for _, content := range d.HTMLSlots {
		size += len(content)
	}
	for _, ops := range d.ListOps {
		for _, op := range ops {
			size += len(op.Content)
		}
	}
	size += len(d.Full)
	return size
}

// SendOptimizedDiff sends an optimized diff payload to the client.
func (s *Socket) SendOptimizedDiff(payload *DiffPayload) error {
	if payload == nil || payload.IsEmpty() {
		return nil
	}

	return s.Push("diff", map[string]any{
		"v": payload.Version,
		"s": payload.Slots,
		"h": payload.HTMLSlots,
		"l": payload.ListOps,
		"f": payload.Full,
	})
}

// SendDiff sends a diff update to the client (legacy compatibility).
// Deprecated: Use SendOptimizedDiff for new code.
func (s *Socket) SendDiff(payload *DiffPayload) error {
	return s.SendOptimizedDiff(payload)
}

// Subscribe adds a topic subscription.
func (s *Socket) Subscribe(topic string, handler func(any)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscriptions[topic] = true
	// Note: actual subscription logic would be handled by PubSub
}

// Unsubscribe removes a topic subscription.
func (s *Socket) Unsubscribe(topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscriptions, topic)
}

// Subscriptions returns all active subscriptions.
func (s *Socket) Subscriptions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topics := make([]string, 0, len(s.subscriptions))
	for topic := range s.subscriptions {
		topics = append(topics, topic)
	}
	return topics
}

// GetMetadata retrieves metadata by key.
func (s *Socket) GetMetadata(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metadata[key]
}

// SetMetadata stores metadata.
func (s *Socket) SetMetadata(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[key] = value
}

// GetCookie retrieves a cookie value (stored in metadata).
func (s *Socket) GetCookie(name string) string {
	cookies, ok := s.GetMetadata("cookies").(map[string]string)
	if !ok {
		return ""
	}
	return cookies[name]
}

// Assign sets a value in assigns (convenience method).
func (s *Socket) Assign(key string, value any) {
	s.assigns.Set(key, value)
}

// Close closes the socket connection.
func (s *Socket) Close() error {
	s.mu.Lock()
	s.connected = false
	transport := s.transport
	s.mu.Unlock()

	if transport != nil {
		return transport.Close()
	}
	return nil
}

// Disconnect is an alias for Close.
func (s *Socket) Disconnect() error {
	return s.Close()
}

// IncrementErrorCount increments the error counter.
func (s *Socket) IncrementErrorCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorCount++
	return s.errorCount
}

// ResetErrorCount resets the error counter.
func (s *Socket) ResetErrorCount() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorCount = 0
}

// ErrorCount returns the current error count.
func (s *Socket) ErrorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errorCount
}

// SocketManager manages all active sockets.
type SocketManager struct {
	sockets     map[string]*Socket
	activeAsync sync.WaitGroup
	shutdownCh  chan struct{}
	isShutdown  bool
	mu          sync.RWMutex
}

// NewSocketManager creates a new socket manager.
func NewSocketManager() *SocketManager {
	return &SocketManager{
		sockets:    make(map[string]*Socket),
		shutdownCh: make(chan struct{}),
	}
}

// Add registers a socket.
func (sm *SocketManager) Add(socket *Socket) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sockets[socket.ID()] = socket
}

// Remove unregisters a socket.
func (sm *SocketManager) Remove(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sockets, id)
}

// Get retrieves a socket by ID.
func (sm *SocketManager) Get(id string) (*Socket, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	s, ok := sm.sockets[id]
	return s, ok
}

// Count returns the number of active sockets.
func (sm *SocketManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sockets)
}

// All returns all sockets.
func (sm *SocketManager) All() []*Socket {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Socket, 0, len(sm.sockets))
	for _, s := range sm.sockets {
		result = append(result, s)
	}
	return result
}

// Broadcast sends a message to all sockets.
// Uses a worker pool to limit concurrent goroutines and prevent leaks.
func (sm *SocketManager) Broadcast(msg Message) {
	sm.mu.RLock()
	sockets := make([]*Socket, 0, len(sm.sockets))
	for _, s := range sm.sockets {
		sockets = append(sockets, s)
	}
	sm.mu.RUnlock()

	if len(sockets) == 0 {
		return
	}

	// Worker pool with limit to prevent goroutine explosion
	const maxWorkers = 100
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, s := range sockets {
		wg.Add(1)
		sem <- struct{}{} // Acquire slot
		go func(socket *Socket) {
			defer func() {
				<-sem // Release slot
				wg.Done()
			}()
			socket.Send(msg)
		}(s)
	}

	wg.Wait()
}

// BroadcastAsync sends a message to all sockets without waiting.
// SECURITY FIX: Supports graceful shutdown to prevent lost messages.
func (sm *SocketManager) BroadcastAsync(msg Message) {
	// Check if shutdown is in progress
	sm.mu.RLock()
	if sm.isShutdown {
		sm.mu.RUnlock()
		return
	}

	sockets := make([]*Socket, 0, len(sm.sockets))
	for _, s := range sm.sockets {
		sockets = append(sockets, s)
	}
	sm.mu.RUnlock()

	if len(sockets) == 0 {
		return
	}

	// Track async operation for graceful shutdown
	sm.activeAsync.Add(1)

	go func() {
		defer sm.activeAsync.Done()

		// Worker pool with limit to prevent goroutine explosion
		const maxWorkers = 100
		sem := make(chan struct{}, maxWorkers)
		var wg sync.WaitGroup

		for _, s := range sockets {
			// Check if shutdown started
			select {
			case <-sm.shutdownCh:
				// Shutdown in progress, stop spawning new workers
				wg.Wait()
				return
			default:
			}

			wg.Add(1)
			sem <- struct{}{} // Acquire slot
			go func(socket *Socket) {
				defer func() {
					<-sem // Release slot
					wg.Done()
				}()
				socket.Send(msg)
			}(s)
		}

		wg.Wait()
	}()
}

// Shutdown gracefully shuts down the socket manager.
// Waits for all async operations to complete or until context is cancelled.
func (sm *SocketManager) Shutdown(ctx context.Context) error {
	sm.mu.Lock()
	if sm.isShutdown {
		sm.mu.Unlock()
		return nil
	}
	sm.isShutdown = true
	close(sm.shutdownCh)
	sm.mu.Unlock()

	// Wait for async operations to complete
	done := make(chan struct{})
	go func() {
		sm.activeAsync.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsShutdown returns true if the manager is shutting down.
func (sm *SocketManager) IsShutdown() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isShutdown
}

// CleanupInactive removes sockets inactive for longer than the duration.
func (sm *SocketManager) CleanupInactive(ctx context.Context, maxInactive time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, s := range sm.sockets {
		if now.Sub(s.LastActivity()) > maxInactive {
			s.Close()
			delete(sm.sockets, id)
			removed++
		}
	}

	return removed
}

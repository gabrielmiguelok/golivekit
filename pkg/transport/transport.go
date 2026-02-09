// Package transport provides the communication layer for GoliveKit.
// It supports multiple transport mechanisms: WebSocket (primary),
// Server-Sent Events (fallback), and long-polling (legacy).
package transport

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Common transport errors.
var (
	ErrNotConnected     = errors.New("transport not connected")
	ErrConnectionClosed = errors.New("connection closed")
	ErrSendTimeout      = errors.New("send timeout")
	ErrInvalidMessage   = errors.New("invalid message format")
	ErrTransportFull    = errors.New("transport buffer full")
)

// Transport is the interface for all transport mechanisms.
type Transport interface {
	// Connect establishes the connection.
	Connect(ctx context.Context) error

	// Send sends a message to the client.
	Send(msg Message) error

	// Receive returns a channel for incoming messages.
	Receive() <-chan Message

	// Close terminates the connection.
	Close() error

	// IsConnected returns true if connected.
	IsConnected() bool

	// Type returns the transport type.
	Type() TransportType
}

// TransportType identifies the transport mechanism.
type TransportType string

const (
	TransportWebSocket   TransportType = "websocket"
	TransportSSE         TransportType = "sse"
	TransportLongPolling TransportType = "longpoll"
)

// Message represents a message sent over a transport.
type Message struct {
	// Ref is an optional message reference for request/response correlation
	Ref string `json:"ref,omitempty"`

	// Topic is the channel/room the message is for
	Topic string `json:"topic"`

	// Event is the event type
	Event string `json:"event"`

	// Payload contains the message data
	Payload map[string]any `json:"payload,omitempty"`

	// Timestamp is when the message was created
	Timestamp time.Time `json:"ts,omitempty"`
}

// NewMessage creates a new message.
func NewMessage(topic, event string, payload map[string]any) Message {
	return Message{
		Topic:     topic,
		Event:     event,
		Payload:   payload,
		Timestamp: time.Now(),
	}
}

// WithRef adds a reference to the message.
func (m Message) WithRef(ref string) Message {
	m.Ref = ref
	return m
}

// Marshal serializes the message to JSON.
func (m Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal deserializes a message from JSON.
func Unmarshal(data []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return m, err
}

// TransportConfig holds common transport configuration.
type TransportConfig struct {
	// ReadTimeout is the maximum time to wait for a read
	ReadTimeout time.Duration

	// WriteTimeout is the maximum time to wait for a write
	WriteTimeout time.Duration

	// PingInterval is how often to send heartbeats
	PingInterval time.Duration

	// PongTimeout is how long to wait for a pong response
	PongTimeout time.Duration

	// MaxMessageSize is the maximum message size in bytes
	MaxMessageSize int64

	// SendBufferSize is the size of the send channel buffer
	SendBufferSize int

	// ReceiveBufferSize is the size of the receive channel buffer
	ReceiveBufferSize int
}

// DefaultTransportConfig returns sensible defaults.
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      10 * time.Second,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
		MaxMessageSize:    512 * 1024, // 512KB
		SendBufferSize:    256,
		ReceiveBufferSize: 256,
	}
}

// BaseTransport provides common functionality for transports.
type BaseTransport struct {
	config    *TransportConfig
	connected bool
	sendCh    chan Message
	recvCh    chan Message
	closeCh   chan struct{}
	closeOnce sync.Once
	mu        sync.RWMutex
}

// NewBaseTransport creates a new base transport.
func NewBaseTransport(config *TransportConfig) *BaseTransport {
	if config == nil {
		config = DefaultTransportConfig()
	}
	return &BaseTransport{
		config:  config,
		sendCh:  make(chan Message, config.SendBufferSize),
		recvCh:  make(chan Message, config.ReceiveBufferSize),
		closeCh: make(chan struct{}),
	}
}

// Config returns the transport configuration.
func (t *BaseTransport) Config() *TransportConfig {
	return t.config
}

// IsConnected returns the connection status.
func (t *BaseTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

// SetConnected updates the connection status.
func (t *BaseTransport) SetConnected(connected bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connected = connected
}

// Receive returns the receive channel.
func (t *BaseTransport) Receive() <-chan Message {
	return t.recvCh
}

// SendChannel returns the send channel.
func (t *BaseTransport) SendChannel() chan<- Message {
	return t.sendCh
}

// CloseChan returns the close channel.
func (t *BaseTransport) CloseChan() <-chan struct{} {
	return t.closeCh
}

// Close closes the base transport channels.
func (t *BaseTransport) Close() error {
	t.closeOnce.Do(func() {
		t.SetConnected(false)
		close(t.closeCh)
	})
	return nil
}

// PushMessage pushes a message to the receive channel.
func (t *BaseTransport) PushMessage(msg Message) error {
	select {
	case t.recvCh <- msg:
		return nil
	case <-t.closeCh:
		return ErrConnectionClosed
	default:
		return ErrTransportFull
	}
}

// TransportHandler handles transport events.
type TransportHandler interface {
	OnConnect(transport Transport)
	OnDisconnect(transport Transport, err error)
	OnMessage(transport Transport, msg Message)
	OnError(transport Transport, err error)
}

// TransportManager manages multiple transports.
type TransportManager struct {
	transports map[string]Transport
	handler    TransportHandler
	mu         sync.RWMutex
}

// NewTransportManager creates a new transport manager.
func NewTransportManager(handler TransportHandler) *TransportManager {
	return &TransportManager{
		transports: make(map[string]Transport),
		handler:    handler,
	}
}

// Add registers a transport.
func (tm *TransportManager) Add(id string, transport Transport) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.transports[id] = transport
}

// Remove unregisters a transport.
func (tm *TransportManager) Remove(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.transports, id)
}

// Get retrieves a transport by ID.
func (tm *TransportManager) Get(id string) (Transport, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.transports[id]
	return t, ok
}

// Count returns the number of transports.
func (tm *TransportManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.transports)
}

// Broadcast sends a message to all transports.
func (tm *TransportManager) Broadcast(msg Message) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, t := range tm.transports {
		go t.Send(msg)
	}
}

// CloseAll closes all transports.
func (tm *TransportManager) CloseAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for id, t := range tm.transports {
		t.Close()
		delete(tm.transports, id)
	}
}

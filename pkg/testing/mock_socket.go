package testing

import (
	"sync"
	"time"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/google/uuid"
)

// MockSocket implements core.Transport for testing.
type MockSocket struct {
	ID           string
	Connected    bool
	Sent         []core.Message
	Received     []core.Message
	Closed       bool
	metadata     map[string]any
	errorToSend  error

	mu sync.Mutex
}

// NewMockSocket creates a new mock socket.
func NewMockSocket() *MockSocket {
	return &MockSocket{
		ID:        "test-socket-" + uuid.New().String()[:8],
		Connected: true,
		Sent:      make([]core.Message, 0),
		Received:  make([]core.Message, 0),
		metadata:  make(map[string]any),
	}
}

// Send records a sent message.
func (ms *MockSocket) Send(msg core.Message) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.errorToSend != nil {
		return ms.errorToSend
	}

	if ms.Closed {
		return core.ErrSocketClosed
	}

	ms.Sent = append(ms.Sent, msg)
	return nil
}

// Close marks the socket as closed.
func (ms *MockSocket) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.Closed = true
	ms.Connected = false
	return nil
}

// IsConnected returns the connection status.
func (ms *MockSocket) IsConnected() bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.Connected && !ms.Closed
}

// LastSent returns the last sent message.
func (ms *MockSocket) LastSent() core.Message {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if len(ms.Sent) == 0 {
		return core.Message{}
	}
	return ms.Sent[len(ms.Sent)-1]
}

// SentCount returns the number of sent messages.
func (ms *MockSocket) SentCount() int {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return len(ms.Sent)
}

// SentMessages returns all sent messages.
func (ms *MockSocket) SentMessages() []core.Message {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	result := make([]core.Message, len(ms.Sent))
	copy(result, ms.Sent)
	return result
}

// SimulateReceive simulates receiving a message.
func (ms *MockSocket) SimulateReceive(msg core.Message) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.Received = append(ms.Received, msg)
}

// GetMetadata retrieves metadata.
func (ms *MockSocket) GetMetadata(key string) any {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.metadata[key]
}

// SetMetadata stores metadata.
func (ms *MockSocket) SetMetadata(key string, value any) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.metadata[key] = value
}

// SetError sets an error to return on next Send.
func (ms *MockSocket) SetError(err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.errorToSend = err
}

// ClearError clears any set error.
func (ms *MockSocket) ClearError() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.errorToSend = nil
}

// Reset resets the mock to initial state.
func (ms *MockSocket) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.Sent = make([]core.Message, 0)
	ms.Received = make([]core.Message, 0)
	ms.Closed = false
	ms.Connected = true
	ms.errorToSend = nil
}

// AssertSent asserts that a message with given event was sent.
func (ms *MockSocket) AssertSent(event string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, msg := range ms.Sent {
		if msg.Event == event {
			return true
		}
	}
	return false
}

// AssertSentWithPayload asserts a message with event and payload was sent.
func (ms *MockSocket) AssertSentWithPayload(event string, payload map[string]any) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, msg := range ms.Sent {
		if msg.Event == event {
			// Simple payload comparison
			for k, v := range payload {
				if msg.Payload[k] != v {
					return false
				}
			}
			return true
		}
	}
	return false
}

// MockPubSub implements a mock PubSub for testing.
type MockPubSub struct {
	subscriptions map[string][]func(any)
	published     []pubsubMessage
	mu            sync.Mutex
}

type pubsubMessage struct {
	Topic   string
	Payload any
}

// NewMockPubSub creates a new mock PubSub.
func NewMockPubSub() *MockPubSub {
	return &MockPubSub{
		subscriptions: make(map[string][]func(any)),
		published:     make([]pubsubMessage, 0),
	}
}

// Subscribe registers a subscriber.
func (mps *MockPubSub) Subscribe(topic string, handler func(any)) {
	mps.mu.Lock()
	defer mps.mu.Unlock()
	mps.subscriptions[topic] = append(mps.subscriptions[topic], handler)
}

// Publish publishes a message.
func (mps *MockPubSub) Publish(topic string, payload any) {
	mps.mu.Lock()
	mps.published = append(mps.published, pubsubMessage{Topic: topic, Payload: payload})
	handlers := mps.subscriptions[topic]
	mps.mu.Unlock()

	// Call handlers outside lock
	for _, handler := range handlers {
		handler(payload)
	}
}

// PublishedCount returns the number of published messages.
func (mps *MockPubSub) PublishedCount() int {
	mps.mu.Lock()
	defer mps.mu.Unlock()
	return len(mps.published)
}

// PublishedTo returns messages published to a topic.
func (mps *MockPubSub) PublishedTo(topic string) []any {
	mps.mu.Lock()
	defer mps.mu.Unlock()

	var result []any
	for _, msg := range mps.published {
		if msg.Topic == topic {
			result = append(result, msg.Payload)
		}
	}
	return result
}

// MockTimer provides a controllable timer for testing.
type MockTimer struct {
	duration time.Duration
	callback func()
	stopped  bool
	fired    bool
	mu       sync.Mutex
}

// NewMockTimer creates a new mock timer.
func NewMockTimer(d time.Duration, callback func()) *MockTimer {
	return &MockTimer{
		duration: d,
		callback: callback,
	}
}

// Fire triggers the timer callback.
func (mt *MockTimer) Fire() {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if mt.stopped || mt.fired {
		return
	}

	mt.fired = true
	if mt.callback != nil {
		mt.callback()
	}
}

// Stop stops the timer.
func (mt *MockTimer) Stop() bool {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	wasActive := !mt.stopped && !mt.fired
	mt.stopped = true
	return wasActive
}

// Reset resets the timer.
func (mt *MockTimer) Reset(d time.Duration) bool {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	wasActive := !mt.stopped && !mt.fired
	mt.duration = d
	mt.stopped = false
	mt.fired = false
	return wasActive
}

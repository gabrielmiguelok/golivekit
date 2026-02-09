// Package pubsub provides pub/sub messaging for GoliveKit.
package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
)

// channelWrapper wraps a channel with sync.Once for safe closing.
type channelWrapper struct {
	ch        chan []byte
	closeOnce sync.Once
}

func newChannelWrapper(size int) *channelWrapper {
	return &channelWrapper{
		ch: make(chan []byte, size),
	}
}

func (cw *channelWrapper) close() {
	cw.closeOnce.Do(func() {
		close(cw.ch)
	})
}

// Common pubsub errors.
var (
	ErrPubSubClosed      = errors.New("pubsub is closed")
	ErrTopicNotFound     = errors.New("topic not found")
	ErrInvalidSubscriber = errors.New("invalid subscriber")
)

// PubSub is the interface for pub/sub implementations.
type PubSub interface {
	// Subscribe adds a handler for a topic.
	Subscribe(topic string, handler func(msg []byte)) (Subscription, error)

	// Publish sends a message to all subscribers of a topic.
	Publish(topic string, msg []byte) error

	// Close shuts down the pubsub system.
	Close() error
}

// Subscription represents an active subscription.
type Subscription interface {
	// Unsubscribe removes this subscription.
	Unsubscribe() error

	// Topic returns the subscribed topic.
	Topic() string
}

// Message represents a pub/sub message.
type Message struct {
	Topic   string
	Payload []byte
}

// MemoryPubSub is an in-memory pub/sub implementation.
// Suitable for single-node deployments and testing.
type MemoryPubSub struct {
	topics  map[string]map[string]*channelWrapper
	subs    map[string]*memorySubscription
	nextID  int
	closed  bool
	mu      sync.RWMutex
}

// NewMemoryPubSub creates a new in-memory pub/sub.
func NewMemoryPubSub() *MemoryPubSub {
	return &MemoryPubSub{
		topics: make(map[string]map[string]*channelWrapper),
		subs:   make(map[string]*memorySubscription),
	}
}

// Subscribe adds a handler for a topic.
// SECURITY FIX: Uses context, atomic flag, and sync.Once to prevent race conditions.
func (ps *MemoryPubSub) Subscribe(topic string, handler func(msg []byte)) (Subscription, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return nil, ErrPubSubClosed
	}

	// Create topic if needed
	if ps.topics[topic] == nil {
		ps.topics[topic] = make(map[string]*channelWrapper)
	}

	// Generate subscription ID
	ps.nextID++
	subID := topic + "-" + string(rune(ps.nextID))

	// Create channel wrapper for this subscription (uses sync.Once for safe close)
	chWrapper := newChannelWrapper(256)
	ps.topics[topic][subID] = chWrapper

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Create subscription
	sub := &memorySubscription{
		id:        subID,
		topic:     topic,
		ps:        ps,
		chWrapper: chWrapper,
		ctx:       ctx,
		cancel:    cancel,
	}
	ps.subs[subID] = sub

	// Start handler goroutine with panic protection and context awareness
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Log panic but don't crash - subscription is already closing
			}
		}()

		for {
			select {
			case msg, ok := <-chWrapper.ch:
				if !ok || sub.closed.Load() {
					return // Channel closed or subscription cancelled
				}
				handler(msg)
			case <-ctx.Done():
				return // Context cancelled
			}
		}
	}()

	return sub, nil
}

// Publish sends a message to all subscribers of a topic.
// SECURITY FIX: Checks subscription closed state before sending to prevent panic.
func (ps *MemoryPubSub) Publish(topic string, msg []byte) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return ErrPubSubClosed
	}

	subscribers := ps.topics[topic]
	if subscribers == nil {
		return nil // No subscribers
	}

	// Make a copy of message to avoid issues
	msgCopy := make([]byte, len(msg))
	copy(msgCopy, msg)

	// Send to all subscribers
	// SECURITY: Check if subscription is closed before sending
	for subID, chWrapper := range subscribers {
		// Get subscription to check closed state
		sub := ps.subs[subID]
		if sub != nil && sub.closed.Load() {
			continue // Skip closed subscriptions
		}

		select {
		case chWrapper.ch <- msgCopy:
			// Sent successfully
		default:
			// Channel full, drop message (backpressure)
		}
	}

	return nil
}

// Close shuts down the pubsub system.
func (ps *MemoryPubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return nil
	}

	ps.closed = true

	// Close all subscription channels using sync.Once (safe for concurrent close)
	for _, subscribers := range ps.topics {
		for _, chWrapper := range subscribers {
			chWrapper.close()
		}
	}

	// Clear maps
	ps.topics = make(map[string]map[string]*channelWrapper)
	ps.subs = make(map[string]*memorySubscription)

	return nil
}

// TopicCount returns the number of topics.
func (ps *MemoryPubSub) TopicCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.topics)
}

// SubscriberCount returns the number of subscribers for a topic.
func (ps *MemoryPubSub) SubscriberCount(topic string) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.topics[topic])
}

type memorySubscription struct {
	id        string
	topic     string
	ps        *MemoryPubSub
	chWrapper *channelWrapper      // SECURITY FIX: Use wrapper with sync.Once
	closed    atomic.Bool          // SECURITY FIX: Use atomic to prevent race condition
	ctx       context.Context
	cancel    context.CancelFunc
}

// Unsubscribe removes this subscription safely.
// Uses atomic bool and sync.Once to prevent race condition with Publish() and Close().
func (s *memorySubscription) Unsubscribe() error {
	// Atomically check and set closed flag to prevent double-close
	if !s.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	// Cancel context first to stop handler goroutine
	if s.cancel != nil {
		s.cancel()
	}

	s.ps.mu.Lock()
	defer s.ps.mu.Unlock()

	// Remove from topic
	if subscribers := s.ps.topics[s.topic]; subscribers != nil {
		delete(subscribers, s.id)
		if len(subscribers) == 0 {
			delete(s.ps.topics, s.topic)
		}
	}

	// Remove from subs map
	delete(s.ps.subs, s.id)

	// Close channel using sync.Once (safe for concurrent close with ps.Close())
	s.chWrapper.close()

	return nil
}

func (s *memorySubscription) Topic() string {
	return s.topic
}

// IsClosed returns true if the subscription is closed.
func (s *memorySubscription) IsClosed() bool {
	return s.closed.Load()
}

// Broadcaster provides a high-level API for broadcasting to sockets.
type Broadcaster struct {
	pubsub PubSub
}

// NewBroadcaster creates a new broadcaster.
func NewBroadcaster(ps PubSub) *Broadcaster {
	return &Broadcaster{pubsub: ps}
}

// Broadcast sends a message to a topic.
func (b *Broadcaster) Broadcast(topic string, event string, payload map[string]any) error {
	// Simple encoding
	msg := Message{
		Topic:   topic,
		Payload: encodeMessage(event, payload),
	}

	data, err := encodeEnvelope(msg)
	if err != nil {
		return err
	}

	return b.pubsub.Publish(topic, data)
}

// BroadcastFrom sends a message to all subscribers except the sender.
func (b *Broadcaster) BroadcastFrom(topic string, senderID string, event string, payload map[string]any) error {
	// Add sender ID to payload for filtering
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["_sender"] = senderID

	return b.Broadcast(topic, event, payload)
}

// Subscribe subscribes to a topic with a message handler.
func (b *Broadcaster) Subscribe(topic string, handler func(event string, payload map[string]any)) (Subscription, error) {
	return b.pubsub.Subscribe(topic, func(msg []byte) {
		event, payload := decodeMessage(msg)
		handler(event, payload)
	})
}

// Helper functions for encoding/decoding

func encodeMessage(event string, payload map[string]any) []byte {
	// Simple JSON encoding
	data := map[string]any{
		"event":   event,
		"payload": payload,
	}
	result, _ := json.Marshal(data)
	return result
}

func encodeEnvelope(msg Message) ([]byte, error) {
	return json.Marshal(msg)
}

func decodeMessage(data []byte) (string, map[string]any) {
	var msg struct {
		Event   string         `json:"event"`
		Payload map[string]any `json:"payload"`
	}
	json.Unmarshal(data, &msg)
	return msg.Event, msg.Payload
}

// Channel provides a high-level abstraction for pub/sub topics.
type Channel struct {
	name    string
	pubsub  PubSub
	subs    []Subscription
	mu      sync.Mutex
}

// NewChannel creates a new channel.
func NewChannel(name string, ps PubSub) *Channel {
	return &Channel{
		name:   name,
		pubsub: ps,
		subs:   make([]Subscription, 0),
	}
}

// Name returns the channel name.
func (c *Channel) Name() string {
	return c.name
}

// Push sends a message to the channel.
func (c *Channel) Push(ctx context.Context, event string, payload map[string]any) error {
	data := encodeMessage(event, payload)
	return c.pubsub.Publish(c.name, data)
}

// Subscribe adds a handler to the channel.
func (c *Channel) Subscribe(handler func(event string, payload map[string]any)) error {
	sub, err := c.pubsub.Subscribe(c.name, func(msg []byte) {
		event, payload := decodeMessage(msg)
		handler(event, payload)
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.subs = append(c.subs, sub)
	c.mu.Unlock()

	return nil
}

// Close closes the channel and unsubscribes all handlers.
func (c *Channel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sub := range c.subs {
		sub.Unsubscribe()
	}
	c.subs = nil

	return nil
}

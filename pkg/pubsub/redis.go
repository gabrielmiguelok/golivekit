// Package pubsub provides pub/sub messaging for GoliveKit.
package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Redis-specific errors.
var (
	ErrRedisNotConnected = errors.New("redis not connected")
	ErrRedisSubscription = errors.New("redis subscription failed")
)

// RedisConfig configures the Redis PubSub connection.
type RedisConfig struct {
	// Addr is the Redis server address (default: "localhost:6379")
	Addr string

	// Password is the Redis password (empty for no auth)
	Password string

	// DB is the Redis database number (default: 0)
	DB int

	// PoolSize is the connection pool size (default: 10)
	PoolSize int

	// ReadTimeout for operations (default: 3s)
	ReadTimeout time.Duration

	// WriteTimeout for operations (default: 3s)
	WriteTimeout time.Duration

	// DialTimeout for initial connection (default: 5s)
	DialTimeout time.Duration

	// MaxRetries before giving up (default: 3)
	MaxRetries int
}

// DefaultRedisConfig returns sensible defaults.
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Addr:         "localhost:6379",
		DB:           0,
		PoolSize:     10,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		DialTimeout:  5 * time.Second,
		MaxRetries:   3,
	}
}

// RedisPubSub implements PubSub using Redis.
// Note: This is a stub implementation. To use Redis, add the redis client dependency:
// go get github.com/redis/go-redis/v9
type RedisPubSub struct {
	config *RedisConfig

	// Subscriptions tracking
	subs   map[string][]*redisSubscription
	nextID int64

	// State
	ctx    context.Context
	cancel context.CancelFunc
	closed bool

	mu sync.RWMutex
}

// redisSubscription represents a Redis subscription.
type redisSubscription struct {
	id      int64
	topic   string
	handler func([]byte)
	ps      *RedisPubSub
	closed  bool
	mu      sync.Mutex
}

// NewRedisPubSub creates a new Redis PubSub.
// Note: This creates a mock implementation. For real Redis support,
// uncomment the go-redis integration below.
func NewRedisPubSub(config *RedisConfig) (*RedisPubSub, error) {
	if config == nil {
		config = DefaultRedisConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	ps := &RedisPubSub{
		config: config,
		subs:   make(map[string][]*redisSubscription),
		ctx:    ctx,
		cancel: cancel,
	}

	// In a real implementation, you would connect to Redis here:
	// ps.client = redis.NewClient(&redis.Options{
	//     Addr:         config.Addr,
	//     Password:     config.Password,
	//     DB:           config.DB,
	//     PoolSize:     config.PoolSize,
	//     ReadTimeout:  config.ReadTimeout,
	//     WriteTimeout: config.WriteTimeout,
	//     DialTimeout:  config.DialTimeout,
	//     MaxRetries:   config.MaxRetries,
	// })
	//
	// if err := ps.client.Ping(ctx).Err(); err != nil {
	//     cancel()
	//     return nil, fmt.Errorf("redis connection failed: %w", err)
	// }
	//
	// ps.pubsub = ps.client.Subscribe(ctx)

	return ps, nil
}

// Subscribe adds a handler for a topic.
func (ps *RedisPubSub) Subscribe(topic string, handler func(msg []byte)) (Subscription, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return nil, ErrPubSubClosed
	}

	ps.nextID++
	sub := &redisSubscription{
		id:      ps.nextID,
		topic:   topic,
		handler: handler,
		ps:      ps,
	}

	ps.subs[topic] = append(ps.subs[topic], sub)

	// In a real implementation, you would subscribe to Redis here:
	// if err := ps.pubsub.Subscribe(ps.ctx, topic); err != nil {
	//     return nil, fmt.Errorf("redis subscribe failed: %w", err)
	// }

	return sub, nil
}

// Publish sends a message to all subscribers of a topic.
func (ps *RedisPubSub) Publish(topic string, msg []byte) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if ps.closed {
		return ErrPubSubClosed
	}

	// In a real implementation, you would publish to Redis:
	// return ps.client.Publish(ps.ctx, topic, msg).Err()

	// For now, deliver locally (useful for single-node or testing)
	subs := ps.subs[topic]
	for _, sub := range subs {
		if !sub.closed {
			// Deliver asynchronously to prevent blocking
			go sub.handler(msg)
		}
	}

	return nil
}

// Close shuts down the pubsub system.
func (ps *RedisPubSub) Close() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.closed {
		return nil
	}

	ps.closed = true
	ps.cancel()

	// In a real implementation:
	// ps.pubsub.Close()
	// ps.client.Close()

	// Clear subscriptions
	for _, subs := range ps.subs {
		for _, sub := range subs {
			sub.closed = true
		}
	}
	ps.subs = make(map[string][]*redisSubscription)

	return nil
}

// Unsubscribe removes this subscription.
func (s *redisSubscription) Unsubscribe() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	s.ps.mu.Lock()
	defer s.ps.mu.Unlock()

	// Remove from subscriptions list
	subs := s.ps.subs[s.topic]
	for i, sub := range subs {
		if sub.id == s.id {
			s.ps.subs[s.topic] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	// If no more subscribers for this topic, unsubscribe from Redis
	// if len(s.ps.subs[s.topic]) == 0 {
	//     s.ps.pubsub.Unsubscribe(s.ps.ctx, s.topic)
	// }

	return nil
}

// Topic returns the subscribed topic.
func (s *redisSubscription) Topic() string {
	return s.topic
}

// RedisBroadcaster provides a high-level API for Redis broadcasting.
type RedisBroadcaster struct {
	pubsub *RedisPubSub
	prefix string
}

// NewRedisBroadcaster creates a new Redis broadcaster.
func NewRedisBroadcaster(ps *RedisPubSub, prefix string) *RedisBroadcaster {
	return &RedisBroadcaster{
		pubsub: ps,
		prefix: prefix,
	}
}

// Broadcast sends a message to a topic.
func (b *RedisBroadcaster) Broadcast(topic string, event string, payload map[string]any) error {
	fullTopic := b.prefix + topic

	msg := struct {
		Event   string         `json:"event"`
		Payload map[string]any `json:"payload"`
	}{
		Event:   event,
		Payload: payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return b.pubsub.Publish(fullTopic, data)
}

// Subscribe subscribes to a topic.
func (b *RedisBroadcaster) Subscribe(topic string, handler func(event string, payload map[string]any)) (Subscription, error) {
	fullTopic := b.prefix + topic

	return b.pubsub.Subscribe(fullTopic, func(data []byte) {
		var msg struct {
			Event   string         `json:"event"`
			Payload map[string]any `json:"payload"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			return
		}
		handler(msg.Event, msg.Payload)
	})
}

// RedisPresence provides presence tracking using Redis.
type RedisPresence struct {
	pubsub   *RedisPubSub
	prefix   string
	presence sync.Map // topic -> map[string]PresenceInfo
}

// PresenceInfo contains information about a present user.
type PresenceInfo struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata,omitempty"`
	JoinedAt time.Time      `json:"joined_at"`
}

// NewRedisPresence creates a new Redis presence tracker.
func NewRedisPresence(ps *RedisPubSub, prefix string) *RedisPresence {
	return &RedisPresence{
		pubsub: ps,
		prefix: prefix,
	}
}

// Track adds a user to a topic's presence.
func (p *RedisPresence) Track(topic, id string, metadata map[string]any) error {
	info := PresenceInfo{
		ID:       id,
		Metadata: metadata,
		JoinedAt: time.Now(),
	}

	// Store locally
	presenceMap, _ := p.presence.LoadOrStore(topic, &sync.Map{})
	presenceMap.(*sync.Map).Store(id, info)

	// Broadcast presence update
	return p.pubsub.Publish(p.prefix+"presence:"+topic, mustJSON(map[string]any{
		"type":    "join",
		"id":      id,
		"payload": info,
	}))
}

// Untrack removes a user from a topic's presence.
func (p *RedisPresence) Untrack(topic, id string) error {
	// Remove locally
	if presenceMap, ok := p.presence.Load(topic); ok {
		presenceMap.(*sync.Map).Delete(id)
	}

	// Broadcast presence update
	return p.pubsub.Publish(p.prefix+"presence:"+topic, mustJSON(map[string]any{
		"type": "leave",
		"id":   id,
	}))
}

// List returns all present users in a topic.
func (p *RedisPresence) List(topic string) []PresenceInfo {
	presenceMap, ok := p.presence.Load(topic)
	if !ok {
		return nil
	}

	var result []PresenceInfo
	presenceMap.(*sync.Map).Range(func(key, value any) bool {
		result = append(result, value.(PresenceInfo))
		return true
	})

	return result
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

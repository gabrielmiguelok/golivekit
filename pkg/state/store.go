// Package state provides state management for GoliveKit components.
// It supports multiple backends: in-memory (default), Redis, and custom stores.
package state

import (
	"context"
	"errors"
	"time"
)

// Common store errors.
var (
	ErrKeyNotFound    = errors.New("key not found")
	ErrStoreClosed    = errors.New("store is closed")
	ErrInvalidData    = errors.New("invalid data format")
	ErrStoreTimeout   = errors.New("store operation timeout")
)

// Store is the interface for state storage backends.
type Store interface {
	// Get retrieves a value by key.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value with optional TTL.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// Keys returns all keys matching a pattern.
	Keys(ctx context.Context, pattern string) ([]string, error)

	// Close closes the store.
	Close() error
}

// TypedStore provides type-safe access to the store.
type TypedStore[T any] struct {
	store      Store
	serializer Serializer[T]
}

// NewTypedStore creates a new typed store wrapper.
func NewTypedStore[T any](store Store, serializer Serializer[T]) *TypedStore[T] {
	return &TypedStore[T]{
		store:      store,
		serializer: serializer,
	}
}

// Get retrieves and deserializes a value.
func (ts *TypedStore[T]) Get(ctx context.Context, key string) (T, error) {
	var zero T

	data, err := ts.store.Get(ctx, key)
	if err != nil {
		return zero, err
	}

	return ts.serializer.Deserialize(data)
}

// Set serializes and stores a value.
func (ts *TypedStore[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	data, err := ts.serializer.Serialize(value)
	if err != nil {
		return err
	}

	return ts.store.Set(ctx, key, data, ttl)
}

// Delete removes a key.
func (ts *TypedStore[T]) Delete(ctx context.Context, key string) error {
	return ts.store.Delete(ctx, key)
}

// Serializer handles serialization/deserialization.
type Serializer[T any] interface {
	Serialize(value T) ([]byte, error)
	Deserialize(data []byte) (T, error)
}

// ComponentState holds the persistent state of a component.
type ComponentState struct {
	// SocketID is the socket this state belongs to
	SocketID string `json:"socket_id" msgpack:"socket_id"`

	// ComponentName is the component type
	ComponentName string `json:"component_name" msgpack:"component_name"`

	// Assigns contains the component's assigns
	Assigns map[string]any `json:"assigns" msgpack:"assigns"`

	// Version is the state version for optimistic locking
	Version uint64 `json:"version" msgpack:"version"`

	// CreatedAt is when the state was created
	CreatedAt time.Time `json:"created_at" msgpack:"created_at"`

	// UpdatedAt is when the state was last updated
	UpdatedAt time.Time `json:"updated_at" msgpack:"updated_at"`

	// ExpiresAt is when the state expires (for recovery)
	ExpiresAt time.Time `json:"expires_at" msgpack:"expires_at"`
}

// NewComponentState creates a new component state.
func NewComponentState(socketID, componentName string) *ComponentState {
	now := time.Now()
	return &ComponentState{
		SocketID:      socketID,
		ComponentName: componentName,
		Assigns:       make(map[string]any),
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
		ExpiresAt:     now.Add(24 * time.Hour), // Default 24h expiry
	}
}

// Update increments the version and updates the timestamp.
func (cs *ComponentState) Update() {
	cs.Version++
	cs.UpdatedAt = time.Now()
}

// IsExpired returns true if the state has expired.
func (cs *ComponentState) IsExpired() bool {
	return time.Now().After(cs.ExpiresAt)
}

// SetExpiry sets when the state expires.
func (cs *ComponentState) SetExpiry(d time.Duration) {
	cs.ExpiresAt = time.Now().Add(d)
}

// StateManager provides high-level state management operations.
type StateManager struct {
	store        Store
	serializer   *MsgPackSerializer
	keyPrefix    string
	defaultTTL   time.Duration
}

// NewStateManager creates a new state manager.
func NewStateManager(store Store, opts ...StateManagerOption) *StateManager {
	sm := &StateManager{
		store:      store,
		serializer: NewMsgPackSerializer(),
		keyPrefix:  "golivekit:state:",
		defaultTTL: 24 * time.Hour,
	}

	for _, opt := range opts {
		opt(sm)
	}

	return sm
}

// StateManagerOption configures the state manager.
type StateManagerOption func(*StateManager)

// WithKeyPrefix sets the key prefix.
func WithKeyPrefix(prefix string) StateManagerOption {
	return func(sm *StateManager) {
		sm.keyPrefix = prefix
	}
}

// WithDefaultTTL sets the default TTL.
func WithDefaultTTL(ttl time.Duration) StateManagerOption {
	return func(sm *StateManager) {
		sm.defaultTTL = ttl
	}
}

// Save persists component state.
func (sm *StateManager) Save(ctx context.Context, state *ComponentState) error {
	state.Update()

	data, err := sm.serializer.Marshal(state)
	if err != nil {
		return err
	}

	key := sm.keyPrefix + state.SocketID
	ttl := time.Until(state.ExpiresAt)

	return sm.store.Set(ctx, key, data, ttl)
}

// Load retrieves component state.
func (sm *StateManager) Load(ctx context.Context, socketID string) (*ComponentState, error) {
	key := sm.keyPrefix + socketID

	data, err := sm.store.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var state ComponentState
	if err := sm.serializer.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	if state.IsExpired() {
		sm.Delete(ctx, socketID)
		return nil, ErrKeyNotFound
	}

	return &state, nil
}

// Delete removes component state.
func (sm *StateManager) Delete(ctx context.Context, socketID string) error {
	key := sm.keyPrefix + socketID
	return sm.store.Delete(ctx, key)
}

// Exists checks if state exists for a socket.
func (sm *StateManager) Exists(ctx context.Context, socketID string) (bool, error) {
	key := sm.keyPrefix + socketID
	return sm.store.Exists(ctx, key)
}

// ListSockets returns all socket IDs with saved state.
func (sm *StateManager) ListSockets(ctx context.Context) ([]string, error) {
	pattern := sm.keyPrefix + "*"
	keys, err := sm.store.Keys(ctx, pattern)
	if err != nil {
		return nil, err
	}

	prefixLen := len(sm.keyPrefix)
	socketIDs := make([]string, len(keys))
	for i, key := range keys {
		socketIDs[i] = key[prefixLen:]
	}

	return socketIDs, nil
}

// Cleanup removes expired states.
func (sm *StateManager) Cleanup(ctx context.Context) (int, error) {
	socketIDs, err := sm.ListSockets(ctx)
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, socketID := range socketIDs {
		state, err := sm.Load(ctx, socketID)
		if errors.Is(err, ErrKeyNotFound) {
			continue
		}
		if err != nil {
			continue
		}

		if state.IsExpired() {
			sm.Delete(ctx, socketID)
			removed++
		}
	}

	return removed, nil
}

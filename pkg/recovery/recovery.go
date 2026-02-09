// Package recovery provides state recovery for GoliveKit.
package recovery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Common errors.
var (
	ErrTokenExpired  = errors.New("recovery token expired")
	ErrTokenInvalid  = errors.New("recovery token invalid")
	ErrStateNotFound = errors.New("state not found")
)

// StateStore is the interface for state storage.
type StateStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// RecoveryManager manages state recovery.
type RecoveryManager struct {
	store   StateStore
	secret  []byte
	ttl     time.Duration
	prefix  string
	mu      sync.RWMutex
}

// Config configures the recovery manager.
type Config struct {
	Store  StateStore
	Secret []byte
	TTL    time.Duration
	Prefix string
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	return &Config{
		Store:  NewMemoryStore(),
		Secret: []byte("change-me-in-production"),
		TTL:    24 * time.Hour,
		Prefix: "recovery:",
	}
}

// NewRecoveryManager creates a new recovery manager.
func NewRecoveryManager(config *Config) *RecoveryManager {
	if config == nil {
		config = DefaultConfig()
	}
	return &RecoveryManager{
		store:  config.Store,
		secret: config.Secret,
		ttl:    config.TTL,
		prefix: config.Prefix,
	}
}

// ComponentState represents the state of a component.
type ComponentState struct {
	ComponentName string         `json:"component_name"`
	Assigns       map[string]any `json:"assigns"`
	Version       uint64         `json:"version"`
	CreatedAt     int64          `json:"created_at"`
}

// RecoveryToken represents a recovery token.
type RecoveryToken struct {
	SocketID      string `json:"socket_id"`
	ComponentName string `json:"component_name"`
	StateVersion  uint64 `json:"state_version"`
	CreatedAt     int64  `json:"created_at"`
	ExpiresAt     int64  `json:"expires_at"`
	Signature     string `json:"signature"`
}

// Save saves the state for a socket.
func (rm *RecoveryManager) Save(ctx context.Context, socketID string, state *ComponentState) error {
	key := rm.prefix + socketID

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	return rm.store.Set(ctx, key, data, rm.ttl)
}

// Restore restores state from a recovery token.
func (rm *RecoveryManager) Restore(ctx context.Context, tokenStr string) (*ComponentState, error) {
	// Decode token
	token, err := rm.decodeToken(tokenStr)
	if err != nil {
		return nil, err
	}

	// Validate token
	if err := rm.validateToken(token); err != nil {
		return nil, err
	}

	// Get state from store
	key := rm.prefix + token.SocketID
	data, err := rm.store.Get(ctx, key)
	if err != nil {
		return nil, ErrStateNotFound
	}

	var state ComponentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Verify state version matches
	if state.Version != token.StateVersion {
		return nil, ErrTokenInvalid
	}

	return &state, nil
}

// GenerateToken generates a recovery token for a socket.
func (rm *RecoveryManager) GenerateToken(socketID string, state *ComponentState) (string, error) {
	now := time.Now()

	token := &RecoveryToken{
		SocketID:      socketID,
		ComponentName: state.ComponentName,
		StateVersion:  state.Version,
		CreatedAt:     now.Unix(),
		ExpiresAt:     now.Add(rm.ttl).Unix(),
	}

	// Sign the token
	token.Signature = rm.signToken(token)

	// Encode to JSON then base64
	data, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	return base64.URLEncoding.EncodeToString(data), nil
}

// Delete removes the state for a socket.
func (rm *RecoveryManager) Delete(ctx context.Context, socketID string) error {
	key := rm.prefix + socketID
	return rm.store.Delete(ctx, key)
}

func (rm *RecoveryManager) decodeToken(tokenStr string) (*RecoveryToken, error) {
	data, err := base64.URLEncoding.DecodeString(tokenStr)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	var token RecoveryToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, ErrTokenInvalid
	}

	return &token, nil
}

func (rm *RecoveryManager) validateToken(token *RecoveryToken) error {
	// Check expiration
	if time.Now().Unix() > token.ExpiresAt {
		return ErrTokenExpired
	}

	// Verify signature
	expectedSig := rm.signToken(token)
	if !hmac.Equal([]byte(token.Signature), []byte(expectedSig)) {
		return ErrTokenInvalid
	}

	return nil
}

func (rm *RecoveryManager) signToken(token *RecoveryToken) string {
	// Create signature data (without the signature field)
	data := fmt.Sprintf("%s:%s:%d:%d:%d",
		token.SocketID,
		token.ComponentName,
		token.StateVersion,
		token.CreatedAt,
		token.ExpiresAt,
	)

	mac := hmac.New(sha256.New, rm.secret)
	mac.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// MemoryStore is an in-memory state store.
type MemoryStore struct {
	items map[string]*storeItem
	mu    sync.RWMutex
}

type storeItem struct {
	value     []byte
	expiresAt time.Time
}

// NewMemoryStore creates a new memory store.
func NewMemoryStore() *MemoryStore {
	ms := &MemoryStore{
		items: make(map[string]*storeItem),
	}
	go ms.cleanup()
	return ms
}

// Get retrieves a value from the store.
func (ms *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	ms.mu.RLock()
	item, exists := ms.items[key]
	ms.mu.RUnlock()

	if !exists {
		return nil, ErrStateNotFound
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		ms.Delete(ctx, key)
		return nil, ErrStateNotFound
	}

	return item.value, nil
}

// Set stores a value in the store.
func (ms *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	ms.items[key] = &storeItem{
		value:     value,
		expiresAt: expiresAt,
	}
	return nil
}

// Delete removes a value from the store.
func (ms *MemoryStore) Delete(ctx context.Context, key string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.items, key)
	return nil
}

func (ms *MemoryStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ms.mu.Lock()
		now := time.Now()
		for key, item := range ms.items {
			if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
				delete(ms.items, key)
			}
		}
		ms.mu.Unlock()
	}
}

// RecoveryMiddleware provides recovery middleware for components.
type RecoveryMiddleware struct {
	manager *RecoveryManager
}

// NewRecoveryMiddleware creates a new recovery middleware.
func NewRecoveryMiddleware(manager *RecoveryManager) *RecoveryMiddleware {
	return &RecoveryMiddleware{manager: manager}
}

// OnDisconnect handles socket disconnection.
func (rm *RecoveryMiddleware) OnDisconnect(ctx context.Context, socketID string, state *ComponentState) error {
	return rm.manager.Save(ctx, socketID, state)
}

// OnReconnect handles socket reconnection.
func (rm *RecoveryMiddleware) OnReconnect(ctx context.Context, token string) (*ComponentState, error) {
	return rm.manager.Restore(ctx, token)
}

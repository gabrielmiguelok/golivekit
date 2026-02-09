# recovery

The `recovery` package provides state recovery for GoliveKit applications, enabling seamless reconnection after network interruptions.

## Installation

```go
import "github.com/gabrielmiguelok/golivekit/pkg/recovery"
```

## Overview

When a WebSocket connection drops, users can lose their session state. The recovery package:
- Generates secure recovery tokens
- Stores component state temporarily
- Restores state on reconnection
- Uses HMAC signing for security

## How It Works

1. **Connection established** → Server generates recovery token
2. **State changes** → State snapshots stored with token
3. **Connection drops** → Client retains recovery token
4. **Reconnection** → Client sends token, server restores state
5. **Token expires** → Old state cleaned up automatically

## Basic Usage

### Creating a Recovery Manager

```go
manager := recovery.NewManager(recovery.Options{
    Secret:     []byte("your-32-byte-secret-key-here!!!"),
    TTL:        5 * time.Minute,
    MaxEntries: 10000,
})
```

### Generating Recovery Tokens

```go
// When component mounts, create recovery token
func (c *MyComponent) Mount(ctx context.Context, params core.Params, session core.Session) error {
    // Check for recovery token in params
    if token := params.Get("_recovery"); token != "" {
        if state, err := c.recoveryManager.Recover(token); err == nil {
            c.restoreState(state)
            return nil
        }
    }

    // Normal mount
    c.initializeState()

    // Generate recovery token
    token, err := c.recoveryManager.CreateToken(c.Socket().ID())
    if err != nil {
        return err
    }
    c.Assigns().Set("_recovery_token", token)

    return nil
}
```

### Saving State

```go
// After state changes, save for recovery
func (c *MyComponent) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
    // Handle event
    c.updateState(event, payload)

    // Save state for recovery
    token := c.Assigns().Get("_recovery_token").(string)
    c.recoveryManager.Save(token, c.getRecoverableState())

    return nil
}

func (c *MyComponent) getRecoverableState() map[string]any {
    return map[string]any{
        "count":    c.Count,
        "items":    c.Items,
        "settings": c.Settings,
    }
}
```

### Recovering State

```go
func (c *MyComponent) restoreState(state map[string]any) {
    if count, ok := state["count"].(int); ok {
        c.Count = count
    }
    if items, ok := state["items"].([]any); ok {
        c.Items = items
    }
    if settings, ok := state["settings"].(map[string]any); ok {
        c.Settings = settings
    }
}
```

## Token Security

### HMAC Signing

Tokens are signed with HMAC-SHA256:

```go
// Token structure (base64 encoded):
// [8 bytes: socket ID hash][8 bytes: timestamp][32 bytes: HMAC signature]
```

### Validation

```go
// Tokens are validated on recovery
state, err := manager.Recover(token)
if err != nil {
    switch err {
    case recovery.ErrTokenExpired:
        // Token past TTL
    case recovery.ErrInvalidSignature:
        // Tampered token
    case recovery.ErrNotFound:
        // State already cleaned up
    }
}
```

## Storage Backends

### Memory Storage (Default)

```go
manager := recovery.NewManager(recovery.Options{
    Secret: secret,
    TTL:    5 * time.Minute,
    Storage: recovery.NewMemoryStorage(recovery.MemoryOptions{
        MaxEntries:    10000,
        CleanupPeriod: time.Minute,
    }),
})
```

### Redis Storage

```go
manager := recovery.NewManager(recovery.Options{
    Secret: secret,
    TTL:    5 * time.Minute,
    Storage: recovery.NewRedisStorage(recovery.RedisOptions{
        Client: redisClient,
        Prefix: "golivekit:recovery:",
    }),
})
```

### Custom Storage

```go
type MyStorage struct{}

func (s *MyStorage) Save(token string, state map[string]any, ttl time.Duration) error {
    // Save to your storage
    return nil
}

func (s *MyStorage) Load(token string) (map[string]any, error) {
    // Load from your storage
    return state, nil
}

func (s *MyStorage) Delete(token string) error {
    // Remove from storage
    return nil
}

manager := recovery.NewManager(recovery.Options{
    Secret:  secret,
    Storage: &MyStorage{},
})
```

## JavaScript Client Integration

The client automatically handles recovery tokens:

```javascript
// On disconnect
localStorage.setItem('lv_recovery_' + viewId, recoveryToken)

// On reconnect
const token = localStorage.getItem('lv_recovery_' + viewId)
if (token) {
    socket.send({event: 'phx_join', payload: {_recovery: token}})
}
```

## Middleware Integration

```go
// Middleware to inject recovery manager
func RecoveryMiddleware(manager *recovery.Manager) router.Middleware {
    return func(next router.Handler) router.Handler {
        return router.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := recovery.WithManager(r.Context(), manager)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Access in component
func (c *MyComponent) Mount(ctx context.Context, params core.Params, session core.Session) error {
    manager := recovery.FromContext(ctx)
    // ...
}
```

## Configuration Options

```go
manager := recovery.NewManager(recovery.Options{
    // Secret key for HMAC signing (required, min 32 bytes)
    Secret: []byte("your-32-byte-secret-key-here!!!"),

    // How long to keep recovery state
    TTL: 5 * time.Minute,

    // Storage backend
    Storage: recovery.NewMemoryStorage(recovery.MemoryOptions{
        MaxEntries: 10000,
    }),

    // Called when state is recovered
    OnRecover: func(socketID string, state map[string]any) {
        log.Printf("Recovered state for socket %s", socketID)
    },

    // Called when recovery fails
    OnRecoverError: func(socketID string, err error) {
        log.Printf("Recovery failed for socket %s: %v", socketID, err)
    },

    // State serialization (default: JSON)
    Serializer: recovery.JSONSerializer{},
})
```

## State Serialization

### JSON (Default)

```go
manager := recovery.NewManager(recovery.Options{
    Serializer: recovery.JSONSerializer{},
})
```

### MessagePack

For better performance with large state:

```go
manager := recovery.NewManager(recovery.Options{
    Serializer: recovery.MsgPackSerializer{},
})
```

### Custom Serializer

```go
type MySerializer struct{}

func (s MySerializer) Marshal(state map[string]any) ([]byte, error) {
    // Serialize state
}

func (s MySerializer) Unmarshal(data []byte) (map[string]any, error) {
    // Deserialize state
}
```

## Best Practices

1. **Keep recovery state minimal** - Only store essential data
2. **Use short TTL** - 5 minutes is usually sufficient
3. **Set max entries** - Prevent memory exhaustion
4. **Use Redis for clustering** - Memory storage doesn't share across nodes
5. **Rotate secrets** - Plan for secret rotation
6. **Monitor recovery rates** - Track success/failure metrics
7. **Handle recovery errors gracefully** - Fall back to fresh state

## Metrics

```go
// Built-in metrics
golivekit_recovery_tokens_created_total
golivekit_recovery_tokens_recovered_total
golivekit_recovery_tokens_expired_total
golivekit_recovery_tokens_failed_total
golivekit_recovery_state_size_bytes
```

## Cleanup

```go
// Manual cleanup
manager.CleanupExpired()

// Automatic cleanup (enabled by default)
manager := recovery.NewManager(recovery.Options{
    CleanupInterval: time.Minute,
})

// Graceful shutdown
defer manager.Close()
```

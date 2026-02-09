package core

import (
	"encoding/binary"
	"encoding/json"
	"hash/fnv"
	"sort"
	"sync"
)

// Assigns is a thread-safe store for component state.
// It tracks changes to optimize diff generation.
type Assigns struct {
	data    map[string]any
	tracker *ChangeTracker
	mu      sync.RWMutex
}

// NewAssigns creates a new assigns store.
func NewAssigns() *Assigns {
	return &Assigns{
		data:    make(map[string]any),
		tracker: NewChangeTracker(),
	}
}

// Get retrieves a value from the store.
func (a *Assigns) Get(key string) any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.data[key]
}

// GetString retrieves a string value.
func (a *Assigns) GetString(key string) string {
	if v, ok := a.Get(key).(string); ok {
		return v
	}
	return ""
}

// GetInt retrieves an int value.
func (a *Assigns) GetInt(key string) int {
	if v, ok := a.Get(key).(int); ok {
		return v
	}
	return 0
}

// GetBool retrieves a bool value.
func (a *Assigns) GetBool(key string) bool {
	if v, ok := a.Get(key).(bool); ok {
		return v
	}
	return false
}

// Set stores a value and tracks the change.
func (a *Assigns) Set(key string, value any) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.data[key] = value
	a.tracker.Track(key, value)
}

// SetAll sets multiple values at once.
func (a *Assigns) SetAll(values map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for key, value := range values {
		a.data[key] = value
		a.tracker.Track(key, value)
	}
}

// Delete removes a value from the store.
func (a *Assigns) Delete(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.data, key)
	a.tracker.Track(key, nil)
}

// Data returns a copy of all data.
func (a *Assigns) Data() map[string]any {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]any, len(a.data))
	for k, v := range a.data {
		result[k] = v
	}
	return result
}

// Clone creates a deep copy of the assigns.
// This ensures that mutable values (maps, slices) are properly copied
// to prevent shared state mutations.
func (a *Assigns) Clone() *Assigns {
	a.mu.RLock()
	defer a.mu.RUnlock()

	clone := NewAssigns()
	for k, v := range a.data {
		clone.data[k] = deepCopyValue(v)
	}
	return clone
}

// deepCopyValue creates a deep copy of a value.
// It handles maps, slices, and primitive types.
func deepCopyValue(v any) any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		copyMap := make(map[string]any, len(val))
		for k, v := range val {
			copyMap[k] = deepCopyValue(v)
		}
		return copyMap

	case map[string]string:
		copyMap := make(map[string]string, len(val))
		for k, v := range val {
			copyMap[k] = v
		}
		return copyMap

	case map[string]int:
		copyMap := make(map[string]int, len(val))
		for k, v := range val {
			copyMap[k] = v
		}
		return copyMap

	case []any:
		copySlice := make([]any, len(val))
		for i, v := range val {
			copySlice[i] = deepCopyValue(v)
		}
		return copySlice

	case []string:
		copySlice := make([]string, len(val))
		copy(copySlice, val)
		return copySlice

	case []int:
		copySlice := make([]int, len(val))
		copy(copySlice, val)
		return copySlice

	case []byte:
		copySlice := make([]byte, len(val))
		copy(copySlice, val)
		return copySlice

	default:
		// Primitives (string, int, float64, bool) are immutable in Go
		return v
	}
}

// Tracker returns the change tracker.
func (a *Assigns) Tracker() *ChangeTracker {
	return a.tracker
}

// MarkChanged manually marks a field as changed.
// Useful when modifying nested data structures.
func (a *Assigns) MarkChanged(key string) {
	a.mu.RLock()
	value := a.data[key]
	a.mu.RUnlock()

	a.tracker.Track(key, value)
}

// ChangeTracker tracks changes in assigns between renders.
type ChangeTracker struct {
	// Previous state of each field
	previous map[string]fieldState
	// Fields modified since last render
	changed map[string]bool
	// Version for optimistic concurrency
	version uint64
	mu      sync.RWMutex
}

type fieldState struct {
	hash    uint64
	version uint64
}

// NewChangeTracker creates a new change tracker.
func NewChangeTracker() *ChangeTracker {
	return &ChangeTracker{
		previous: make(map[string]fieldState),
		changed:  make(map[string]bool),
	}
}

// Track registers a change in a field.
func (ct *ChangeTracker) Track(field string, value any) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	newHash := hashValue(value)

	if prev, ok := ct.previous[field]; ok {
		if prev.hash != newHash {
			ct.changed[field] = true
		}
	} else {
		ct.changed[field] = true // New field
	}

	ct.previous[field] = fieldState{
		hash:    newHash,
		version: ct.version,
	}
}

// GetChanged returns fields that changed and clears tracking.
func (ct *ChangeTracker) GetChanged() []string {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	changed := make([]string, 0, len(ct.changed))
	for field := range ct.changed {
		changed = append(changed, field)
	}

	ct.changed = make(map[string]bool)
	ct.version++

	return changed
}

// HasChanges returns true if there are pending changes.
func (ct *ChangeTracker) HasChanges() bool {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.changed) > 0
}

// Version returns the current version number.
func (ct *ChangeTracker) Version() uint64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.version
}

// Reset clears all tracking state.
func (ct *ChangeTracker) Reset() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.previous = make(map[string]fieldState)
	ct.changed = make(map[string]bool)
	ct.version = 0
}

// hashValue calculates a fast hash of any value.
func hashValue(v any) uint64 {
	h := fnv.New64a()

	switch val := v.(type) {
	case nil:
		h.Write([]byte{0})
	case string:
		h.Write([]byte(val))
	case int:
		binary.Write(h, binary.LittleEndian, int64(val))
	case int64:
		binary.Write(h, binary.LittleEndian, val)
	case int32:
		binary.Write(h, binary.LittleEndian, val)
	case float64:
		binary.Write(h, binary.LittleEndian, val)
	case float32:
		binary.Write(h, binary.LittleEndian, val)
	case bool:
		if val {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
	case []any:
		for _, item := range val {
			binary.Write(h, binary.LittleEndian, hashValue(item))
		}
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			h.Write([]byte(k))
			binary.Write(h, binary.LittleEndian, hashValue(val[k]))
		}
	default:
		// Fallback: JSON encoding
		data, _ := json.Marshal(val)
		h.Write(data)
	}

	return h.Sum64()
}

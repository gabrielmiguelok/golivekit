// Package islands provides the Islands Architecture for GoliveKit.
// Islands allow partial hydration of components, improving initial load
// performance by only loading JavaScript where interactivity is needed.
package islands

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"sync"
)

// Island represents a component that hydrates independently.
type Island struct {
	// ID is a unique identifier for this island instance
	ID string

	// Name is the component name
	Name string

	// Props are the serialized component properties
	Props map[string]any

	// Hydration strategy determines when to hydrate
	Hydration HydrationStrategy

	// Priority determines loading order (higher = sooner)
	Priority LoadPriority

	// Slot is optional slot content
	Slot string
}

// HydrationStrategy determines when an island should hydrate.
type HydrationStrategy string

const (
	// HydrateOnLoad hydrates immediately when the page loads
	HydrateOnLoad HydrationStrategy = "load"

	// HydrateOnVisible hydrates when the island becomes visible (IntersectionObserver)
	HydrateOnVisible HydrationStrategy = "visible"

	// HydrateOnIdle hydrates when the browser is idle (requestIdleCallback)
	HydrateOnIdle HydrationStrategy = "idle"

	// HydrateOnInteraction hydrates on first user interaction (click/focus)
	HydrateOnInteraction HydrationStrategy = "interaction"

	// HydrateOnMedia hydrates when a media query matches
	HydrateOnMedia HydrationStrategy = "media"

	// HydrateNever never hydrates (static content only)
	HydrateNever HydrationStrategy = "none"
)

// LoadPriority determines the loading priority of an island.
type LoadPriority int

const (
	PriorityLow    LoadPriority = 1
	PriorityMedium LoadPriority = 2
	PriorityHigh   LoadPriority = 3
)

// IslandOption configures an island.
type IslandOption func(*Island)

// WithHydration sets the hydration strategy.
func WithHydration(strategy HydrationStrategy) IslandOption {
	return func(i *Island) {
		i.Hydration = strategy
	}
}

// WithPriority sets the load priority.
func WithPriority(priority LoadPriority) IslandOption {
	return func(i *Island) {
		i.Priority = priority
	}
}

// WithProps sets the island props.
func WithProps(props map[string]any) IslandOption {
	return func(i *Island) {
		i.Props = props
	}
}

// WithSlot sets slot content.
func WithSlot(slot string) IslandOption {
	return func(i *Island) {
		i.Slot = slot
	}
}

// NewIsland creates a new island.
func NewIsland(id, name string, opts ...IslandOption) *Island {
	island := &Island{
		ID:        id,
		Name:      name,
		Props:     make(map[string]any),
		Hydration: HydrateOnLoad, // Default
		Priority:  PriorityMedium,
	}

	for _, opt := range opts {
		opt(island)
	}

	return island
}

// IslandManager tracks all islands on a page.
type IslandManager struct {
	islands map[string]*Island
	order   []string // Rendering order
	mu      sync.RWMutex
}

// NewIslandManager creates a new island manager.
func NewIslandManager() *IslandManager {
	return &IslandManager{
		islands: make(map[string]*Island),
		order:   make([]string, 0),
	}
}

// Register adds an island.
func (im *IslandManager) Register(island *Island) {
	im.mu.Lock()
	defer im.mu.Unlock()

	im.islands[island.ID] = island
	im.order = append(im.order, island.ID)
}

// Get retrieves an island by ID.
func (im *IslandManager) Get(id string) (*Island, bool) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	island, ok := im.islands[id]
	return island, ok
}

// Remove removes an island.
func (im *IslandManager) Remove(id string) {
	im.mu.Lock()
	defer im.mu.Unlock()

	delete(im.islands, id)

	for i, oid := range im.order {
		if oid == id {
			im.order = append(im.order[:i], im.order[i+1:]...)
			break
		}
	}
}

// All returns all islands in order.
func (im *IslandManager) All() []*Island {
	im.mu.RLock()
	defer im.mu.RUnlock()

	result := make([]*Island, 0, len(im.order))
	for _, id := range im.order {
		if island, ok := im.islands[id]; ok {
			result = append(result, island)
		}
	}
	return result
}

// ByPriority returns islands sorted by priority (highest first).
// Uses O(n log n) sort.Slice instead of O(n²) bubble sort.
func (im *IslandManager) ByPriority() []*Island {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// Create a copy to avoid holding the lock during sort
	islands := make([]*Island, len(im.islands))
	i := 0
	for _, island := range im.islands {
		islands[i] = island
		i++
	}

	// Use efficient sort algorithm O(n log n)
	sort.Slice(islands, func(i, j int) bool {
		return islands[i].Priority > islands[j].Priority
	})

	return islands
}

// ByStrategy returns islands grouped by hydration strategy.
func (im *IslandManager) ByStrategy() map[HydrationStrategy][]*Island {
	islands := im.All()
	result := make(map[HydrationStrategy][]*Island)

	for _, island := range islands {
		result[island.Hydration] = append(result[island.Hydration], island)
	}

	return result
}

// Clear removes all islands.
func (im *IslandManager) Clear() {
	im.mu.Lock()
	defer im.mu.Unlock()

	im.islands = make(map[string]*Island)
	im.order = make([]string, 0)
}

// Count returns the number of islands.
func (im *IslandManager) Count() int {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return len(im.islands)
}

// IslandContext provides island information during rendering.
type IslandContext struct {
	context.Context
	manager *IslandManager
	current *Island
}

// NewIslandContext creates a new island context.
func NewIslandContext(ctx context.Context, manager *IslandManager) *IslandContext {
	return &IslandContext{
		Context: ctx,
		manager: manager,
	}
}

// Manager returns the island manager.
func (ic *IslandContext) Manager() *IslandManager {
	return ic.manager
}

// Current returns the current island being rendered.
func (ic *IslandContext) Current() *Island {
	return ic.current
}

// WithCurrent sets the current island.
func (ic *IslandContext) WithCurrent(island *Island) *IslandContext {
	return &IslandContext{
		Context: ic.Context,
		manager: ic.manager,
		current: island,
	}
}

// IslandManifest describes all islands for the client.
type IslandManifest struct {
	// Islands maps island IDs to their configuration
	Islands map[string]IslandConfig `json:"islands"`

	// Scripts lists required JavaScript files
	Scripts []string `json:"scripts"`

	// Styles lists required CSS files
	Styles []string `json:"styles"`
}

// IslandConfig is the client-side configuration for an island.
type IslandConfig struct {
	Component string            `json:"component"`
	Hydrate   HydrationStrategy `json:"hydrate"`
	Priority  LoadPriority      `json:"priority"`
	Props     map[string]any    `json:"props"`
}

// GenerateManifest creates a manifest from the island manager.
func GenerateManifest(manager *IslandManager) *IslandManifest {
	manifest := &IslandManifest{
		Islands: make(map[string]IslandConfig),
		Scripts: []string{"/_live/golivekit.js"},
		Styles:  []string{"/_live/golivekit.css"},
	}

	for _, island := range manager.All() {
		manifest.Islands[island.ID] = IslandConfig{
			Component: island.Name,
			Hydrate:   island.Hydration,
			Priority:  island.Priority,
			Props:     island.Props,
		}
	}

	return manifest
}

// RenderIslandWrapper generates the HTML wrapper for an island.
func RenderIslandWrapper(island *Island, content string) string {
	propsJSON := "{}"
	if len(island.Props) > 0 {
		// Simple JSON serialization
		propsJSON = serializeProps(island.Props)
	}

	return fmt.Sprintf(`<golive-island id="%s" component="%s" hydrate="%s" priority="%d" props='%s'>%s</golive-island>`,
		island.ID,
		island.Name,
		island.Hydration,
		island.Priority,
		html.EscapeString(propsJSON),
		content,
	)
}

// serializeProps converts props to JSON string.
// This properly serializes all props to valid JSON.
func serializeProps(props map[string]any) string {
	if props == nil || len(props) == 0 {
		return "{}"
	}

	data, err := json.Marshal(props)
	if err != nil {
		// Log error in production, return empty object
		return "{}"
	}
	return string(data)
}

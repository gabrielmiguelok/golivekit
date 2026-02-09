package islands

import (
	"fmt"
	"sync"
)

// Registry manages island component registrations.
type Registry struct {
	components map[string]*IslandDefinition
	mu         sync.RWMutex
}

// IslandDefinition describes an island component.
type IslandDefinition struct {
	// Name is the component name
	Name string

	// Description describes what the component does
	Description string

	// DefaultHydration is the default hydration strategy
	DefaultHydration HydrationStrategy

	// DefaultPriority is the default load priority
	DefaultPriority LoadPriority

	// RequiredProps lists required prop names
	RequiredProps []string

	// OptionalProps lists optional prop names with defaults
	OptionalProps map[string]any

	// Scripts lists additional scripts needed
	Scripts []string

	// Styles lists additional styles needed
	Styles []string

	// PreloadHint suggests browser preload behavior
	PreloadHint PreloadHint
}

// PreloadHint suggests how to preload island resources.
type PreloadHint string

const (
	PreloadNone     PreloadHint = ""
	PreloadPrefetch PreloadHint = "prefetch"
	PreloadPreload  PreloadHint = "preload"
)

// NewRegistry creates a new island registry.
func NewRegistry() *Registry {
	return &Registry{
		components: make(map[string]*IslandDefinition),
	}
}

// Register adds an island definition.
func (r *Registry) Register(def *IslandDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("island name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.components[def.Name]; exists {
		return fmt.Errorf("island %s already registered", def.Name)
	}

	// Apply defaults
	if def.DefaultHydration == "" {
		def.DefaultHydration = HydrateOnLoad
	}
	if def.DefaultPriority == 0 {
		def.DefaultPriority = PriorityMedium
	}

	r.components[def.Name] = def
	return nil
}

// Get retrieves an island definition.
func (r *Registry) Get(name string) (*IslandDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, ok := r.components[name]
	return def, ok
}

// Unregister removes an island definition.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.components, name)
}

// List returns all registered island names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.components))
	for name := range r.components {
		names = append(names, name)
	}
	return names
}

// All returns all island definitions.
func (r *Registry) All() []*IslandDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]*IslandDefinition, 0, len(r.components))
	for _, def := range r.components {
		defs = append(defs, def)
	}
	return defs
}

// Clear removes all registrations.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.components = make(map[string]*IslandDefinition)
}

// CreateIsland creates an island instance from a definition.
func (r *Registry) CreateIsland(id, name string, props map[string]any) (*Island, error) {
	def, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("island %s not registered", name)
	}

	// Validate required props
	for _, reqProp := range def.RequiredProps {
		if _, ok := props[reqProp]; !ok {
			return nil, fmt.Errorf("missing required prop: %s", reqProp)
		}
	}

	// Apply optional prop defaults
	mergedProps := make(map[string]any)
	for k, v := range def.OptionalProps {
		mergedProps[k] = v
	}
	for k, v := range props {
		mergedProps[k] = v
	}

	return &Island{
		ID:        id,
		Name:      name,
		Props:     mergedProps,
		Hydration: def.DefaultHydration,
		Priority:  def.DefaultPriority,
	}, nil
}

// ValidateProps checks if props are valid for an island.
func (r *Registry) ValidateProps(name string, props map[string]any) error {
	def, ok := r.Get(name)
	if !ok {
		return fmt.Errorf("island %s not registered", name)
	}

	for _, reqProp := range def.RequiredProps {
		if _, ok := props[reqProp]; !ok {
			return fmt.Errorf("missing required prop: %s", reqProp)
		}
	}

	return nil
}

// GetResources returns all scripts and styles needed for given islands.
func (r *Registry) GetResources(islandNames []string) (scripts, styles []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	scriptSet := make(map[string]bool)
	styleSet := make(map[string]bool)

	for _, name := range islandNames {
		if def, ok := r.components[name]; ok {
			for _, script := range def.Scripts {
				if !scriptSet[script] {
					scripts = append(scripts, script)
					scriptSet[script] = true
				}
			}
			for _, style := range def.Styles {
				if !styleSet[style] {
					styles = append(styles, style)
					styleSet[style] = true
				}
			}
		}
	}

	return
}

// GlobalRegistry is the default registry.
var GlobalRegistry = NewRegistry()

// Register adds an island to the global registry.
func Register(def *IslandDefinition) error {
	return GlobalRegistry.Register(def)
}

// GetDefinition retrieves from the global registry.
func GetDefinition(name string) (*IslandDefinition, bool) {
	return GlobalRegistry.Get(name)
}

// MustRegister registers an island or panics.
func MustRegister(def *IslandDefinition) {
	if err := GlobalRegistry.Register(def); err != nil {
		panic(err)
	}
}

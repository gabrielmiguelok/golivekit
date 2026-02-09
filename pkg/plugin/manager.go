package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PluginManager manages the lifecycle of plugins.
type PluginManager struct {
	plugins  map[string]Plugin
	order    []string // Initialization order
	app      *App
	config   *PluginManagerConfig
	mu       sync.RWMutex
	started  bool
}

// PluginManagerConfig configures the plugin manager.
type PluginManagerConfig struct {
	// StopOnError stops initialization if a plugin fails
	StopOnError bool
	// ShutdownTimeout is the maximum time to wait for shutdown
	ShutdownTimeout time.Duration
	// InitTimeout is the maximum time to wait for a plugin to initialize
	InitTimeout time.Duration
}

// DefaultPluginManagerConfig returns default configuration.
func DefaultPluginManagerConfig() *PluginManagerConfig {
	return &PluginManagerConfig{
		StopOnError:     true,
		ShutdownTimeout: 30 * time.Second,
		InitTimeout:     10 * time.Second,
	}
}

// NewPluginManager creates a new plugin manager.
func NewPluginManager(app *App, config *PluginManagerConfig) *PluginManager {
	if config == nil {
		config = DefaultPluginManagerConfig()
	}
	return &PluginManager{
		plugins: make(map[string]Plugin),
		order:   make([]string, 0),
		app:     app,
		config:  config,
	}
}

// Register adds a plugin to the manager.
func (pm *PluginManager) Register(plugin Plugin) error {
	info := plugin.Info()

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if already registered
	if _, exists := pm.plugins[info.Name]; exists {
		return &PluginError{
			Plugin:  info.Name,
			Message: "plugin already registered",
		}
	}

	// Verify dependencies
	for _, dep := range info.Requires {
		if _, ok := pm.plugins[dep]; !ok {
			return &PluginError{
				Plugin:  info.Name,
				Message: fmt.Sprintf("missing required plugin: %s", dep),
			}
		}
	}

	// Verify conflicts
	for _, conflict := range info.Conflicts {
		if _, ok := pm.plugins[conflict]; ok {
			return &PluginError{
				Plugin:  info.Name,
				Message: fmt.Sprintf("conflicts with plugin: %s", conflict),
			}
		}
	}

	pm.plugins[info.Name] = plugin
	pm.order = append(pm.order, info.Name)

	return nil
}

// Unregister removes a plugin from the manager.
func (pm *PluginManager) Unregister(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.plugins[name]; !exists {
		return &PluginError{
			Plugin:  name,
			Message: "plugin not found",
		}
	}

	// Check if other plugins depend on this one
	for pname, p := range pm.plugins {
		for _, dep := range p.Info().Requires {
			if dep == name {
				return &PluginError{
					Plugin:  name,
					Message: fmt.Sprintf("cannot unregister: %s depends on it", pname),
				}
			}
		}
	}

	delete(pm.plugins, name)

	// Remove from order
	for i, n := range pm.order {
		if n == name {
			pm.order = append(pm.order[:i], pm.order[i+1:]...)
			break
		}
	}

	return nil
}

// Start initializes all registered plugins with timeout protection.
func (pm *PluginManager) Start() error {
	return pm.StartWithContext(context.Background())
}

// StartWithContext initializes all registered plugins with context support.
func (pm *PluginManager) StartWithContext(ctx context.Context) error {
	pm.mu.Lock()
	if pm.started {
		pm.mu.Unlock()
		return nil
	}
	pm.mu.Unlock()

	pm.mu.RLock()
	order := make([]string, len(pm.order))
	copy(order, pm.order)
	pm.mu.RUnlock()

	var errs []error

	for _, name := range order {
		pm.mu.RLock()
		plugin := pm.plugins[name]
		pm.mu.RUnlock()

		// Initialize with timeout
		if err := pm.initPluginWithTimeout(ctx, name, plugin); err != nil {
			pluginErr := &PluginError{
				Plugin:  name,
				Message: "initialization failed",
				Err:     err,
			}

			if pm.config.StopOnError {
				return pluginErr
			}
			errs = append(errs, pluginErr)
		}
	}

	pm.mu.Lock()
	pm.started = true
	pm.mu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("plugin initialization errors: %v", errs)
	}

	return nil
}

// initPluginWithTimeout initializes a single plugin with timeout protection.
func (pm *PluginManager) initPluginWithTimeout(ctx context.Context, name string, plugin Plugin) error {
	initCtx, cancel := context.WithTimeout(ctx, pm.config.InitTimeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- plugin.Init(pm.app)
	}()

	select {
	case err := <-done:
		return err
	case <-initCtx.Done():
		return fmt.Errorf("plugin %s initialization timeout after %v", name, pm.config.InitTimeout)
	}
}

// Shutdown stops all plugins in reverse order.
func (pm *PluginManager) Shutdown(ctx context.Context) error {
	pm.mu.Lock()
	if !pm.started {
		pm.mu.Unlock()
		return nil
	}
	pm.mu.Unlock()

	pm.mu.RLock()
	order := make([]string, len(pm.order))
	copy(order, pm.order)
	pm.mu.RUnlock()

	var errs []error

	// Shutdown in reverse order
	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]

		pm.mu.RLock()
		plugin := pm.plugins[name]
		pm.mu.RUnlock()

		if err := plugin.Shutdown(ctx); err != nil {
			errs = append(errs, &PluginError{
				Plugin:  name,
				Message: "shutdown failed",
				Err:     err,
			})
		}
	}

	pm.mu.Lock()
	pm.started = false
	pm.mu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("plugin shutdown errors: %v", errs)
	}

	return nil
}

// Get retrieves a plugin by name.
func (pm *PluginManager) Get(name string) (Plugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.plugins[name]
	return p, ok
}

// GetByCapability retrieves all plugins with a specific capability.
func (pm *PluginManager) GetByCapability(cap Capability) []Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []Plugin
	for _, p := range pm.plugins {
		for _, c := range p.Info().Capabilities {
			if c == cap {
				result = append(result, p)
				break
			}
		}
	}
	return result
}

// List returns all registered plugin names.
func (pm *PluginManager) List() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]string, len(pm.order))
	copy(result, pm.order)
	return result
}

// Count returns the number of registered plugins.
func (pm *PluginManager) Count() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.plugins)
}

// IsStarted returns true if the manager has been started.
func (pm *PluginManager) IsStarted() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.started
}

// App returns the application context.
func (pm *PluginManager) App() *App {
	return pm.app
}

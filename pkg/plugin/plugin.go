package plugin

import (
	"context"
	"fmt"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// Plugin is the interface that all plugins must implement.
type Plugin interface {
	// Info returns metadata about the plugin.
	Info() PluginInfo

	// Init initializes the plugin with the application.
	Init(app *App) error

	// Shutdown cleans up plugin resources.
	Shutdown(ctx context.Context) error
}

// PluginInfo contains metadata about a plugin.
type PluginInfo struct {
	Name        string
	Version     string
	Description string
	Author      string
	License     string
	Homepage    string

	// Dependencies
	Requires  []string // Required plugins
	Conflicts []string // Incompatible plugins

	// Capabilities
	Capabilities []Capability
}

// Capability indicates what a plugin provides.
type Capability string

const (
	CapabilityAuth      Capability = "auth"
	CapabilityStorage   Capability = "storage"
	CapabilityTransport Capability = "transport"
	CapabilityAnalytics Capability = "analytics"
	CapabilityUI        Capability = "ui"
	CapabilityCache     Capability = "cache"
	CapabilityPubSub    Capability = "pubsub"
	CapabilityLogging   Capability = "logging"
)

// App represents the application context available to plugins.
type App struct {
	hooks      *HookRegistry
	components *core.ComponentRegistry
	router     *Router
	config     *Config
}

// NewApp creates a new application context.
func NewApp() *App {
	return &App{
		hooks:      NewHookRegistry(),
		components: core.NewComponentRegistry(),
		router:     NewRouter(),
		config:     NewConfig(),
	}
}

// Hooks returns the hook registry.
func (a *App) Hooks() *HookRegistry {
	return a.hooks
}

// Components returns the component registry.
func (a *App) Components() *core.ComponentRegistry {
	return a.components
}

// Router returns the router.
func (a *App) Router() *Router {
	return a.router
}

// Config returns the configuration.
func (a *App) Config() *Config {
	return a.config
}

// RegisterComponent registers a component factory.
func (a *App) RegisterComponent(name string, factory func() core.Component) {
	a.components.Register(name, factory)
}

// Router is a simple HTTP router for plugins.
type Router struct {
	routes []Route
	groups []RouteGroup
}

// Route represents an HTTP route.
type Route struct {
	Method  string
	Path    string
	Handler interface{}
}

// RouteGroup represents a group of routes with a common prefix.
type RouteGroup struct {
	Prefix string
	Routes []Route
}

// NewRouter creates a new router.
func NewRouter() *Router {
	return &Router{
		routes: make([]Route, 0),
		groups: make([]RouteGroup, 0),
	}
}

// Get registers a GET route.
func (r *Router) Get(path string, handler interface{}) {
	r.routes = append(r.routes, Route{Method: "GET", Path: path, Handler: handler})
}

// Post registers a POST route.
func (r *Router) Post(path string, handler interface{}) {
	r.routes = append(r.routes, Route{Method: "POST", Path: path, Handler: handler})
}

// Put registers a PUT route.
func (r *Router) Put(path string, handler interface{}) {
	r.routes = append(r.routes, Route{Method: "PUT", Path: path, Handler: handler})
}

// Delete registers a DELETE route.
func (r *Router) Delete(path string, handler interface{}) {
	r.routes = append(r.routes, Route{Method: "DELETE", Path: path, Handler: handler})
}

// Group creates a route group with a prefix.
func (r *Router) Group(prefix string, fn func(rg *RouteGroup)) {
	group := &RouteGroup{Prefix: prefix, Routes: make([]Route, 0)}
	fn(group)
	r.groups = append(r.groups, *group)
}

// Get adds a GET route to the group.
func (rg *RouteGroup) Get(path string, handler interface{}) {
	rg.Routes = append(rg.Routes, Route{Method: "GET", Path: path, Handler: handler})
}

// Post adds a POST route to the group.
func (rg *RouteGroup) Post(path string, handler interface{}) {
	rg.Routes = append(rg.Routes, Route{Method: "POST", Path: path, Handler: handler})
}

// Routes returns all registered routes.
func (r *Router) Routes() []Route {
	all := make([]Route, len(r.routes))
	copy(all, r.routes)

	for _, g := range r.groups {
		for _, route := range g.Routes {
			all = append(all, Route{
				Method:  route.Method,
				Path:    g.Prefix + route.Path,
				Handler: route.Handler,
			})
		}
	}

	return all
}

// Config holds plugin configuration.
type Config struct {
	values map[string]any
}

// NewConfig creates a new configuration.
func NewConfig() *Config {
	return &Config{
		values: make(map[string]any),
	}
}

// Get retrieves a configuration value.
func (c *Config) Get(key string) any {
	return c.values[key]
}

// GetString retrieves a string configuration value.
func (c *Config) GetString(key string) string {
	if v, ok := c.values[key].(string); ok {
		return v
	}
	return ""
}

// GetInt retrieves an int configuration value.
func (c *Config) GetInt(key string) int {
	if v, ok := c.values[key].(int); ok {
		return v
	}
	return 0
}

// GetBool retrieves a bool configuration value.
func (c *Config) GetBool(key string) bool {
	if v, ok := c.values[key].(bool); ok {
		return v
	}
	return false
}

// Set stores a configuration value.
func (c *Config) Set(key string, value any) {
	c.values[key] = value
}

// SetDefaults sets default values if not already set.
func (c *Config) SetDefaults(defaults map[string]any) {
	for key, value := range defaults {
		if _, exists := c.values[key]; !exists {
			c.values[key] = value
		}
	}
}

// BasePlugin provides default implementations for Plugin methods.
type BasePlugin struct {
	info PluginInfo
}

// NewBasePlugin creates a new base plugin with the given info.
func NewBasePlugin(info PluginInfo) *BasePlugin {
	return &BasePlugin{info: info}
}

// Info returns the plugin info.
func (bp *BasePlugin) Info() PluginInfo {
	return bp.info
}

// Init does nothing by default.
func (bp *BasePlugin) Init(app *App) error {
	return nil
}

// Shutdown does nothing by default.
func (bp *BasePlugin) Shutdown(ctx context.Context) error {
	return nil
}

// PluginError represents an error from a plugin.
type PluginError struct {
	Plugin  string
	Message string
	Err     error
}

func (e *PluginError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("plugin %s: %s: %v", e.Plugin, e.Message, e.Err)
	}
	return fmt.Sprintf("plugin %s: %s", e.Plugin, e.Message)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}

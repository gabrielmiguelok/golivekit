// Package core provides the fundamental abstractions for GoliveKit components.
package core

import (
	"context"
	"io"
)

// Component is the interface that all LiveView components must implement.
// Components are stateful server-side entities that handle user interactions
// and render HTML updates efficiently through WebSocket connections.
type Component interface {
	// Name returns the unique identifier for this component type.
	Name() string

	// Mount is called when the component is first connected.
	// It receives the connection parameters and session data.
	Mount(ctx context.Context, params Params, session Session) error

	// Render returns the current HTML representation of the component.
	// This is called after Mount and after each event that modifies state.
	Render(ctx context.Context) Renderer

	// HandleEvent processes user interactions (clicks, form submissions, etc.).
	// The event string identifies the action, and payload contains event data.
	HandleEvent(ctx context.Context, event string, payload map[string]any) error

	// HandleInfo processes internal messages sent to the component.
	// These are typically used for pub/sub, timers, or background task results.
	HandleInfo(ctx context.Context, msg any) error

	// Terminate is called when the component is being destroyed.
	// Use this for cleanup operations.
	Terminate(ctx context.Context, reason TerminateReason) error
}

// Renderer is the interface for rendering HTML content.
// It's compatible with templ components.
type Renderer interface {
	Render(ctx context.Context, w io.Writer) error
}

// RendererFunc is an adapter to allow ordinary functions to be used as Renderers.
type RendererFunc func(ctx context.Context, w io.Writer) error

func (f RendererFunc) Render(ctx context.Context, w io.Writer) error {
	return f(ctx, w)
}

// Params contains URL parameters and query strings from the connection.
type Params map[string]string

// Get returns a parameter value or empty string if not found.
func (p Params) Get(key string) string {
	return p[key]
}

// GetDefault returns a parameter value or the default if not found.
func (p Params) GetDefault(key, defaultValue string) string {
	if v, ok := p[key]; ok {
		return v
	}
	return defaultValue
}

// Session contains user session data passed from the HTTP handler.
type Session map[string]any

// Get returns a session value.
func (s Session) Get(key string) any {
	return s[key]
}

// GetString returns a session value as string.
func (s Session) GetString(key string) string {
	if v, ok := s[key].(string); ok {
		return v
	}
	return ""
}

// TerminateReason indicates why a component is being terminated.
type TerminateReason int

const (
	// TerminateNormal indicates clean disconnection.
	TerminateNormal TerminateReason = iota
	// TerminateShutdown indicates server shutdown.
	TerminateShutdown
	// TerminateError indicates termination due to an error.
	TerminateError
	// TerminateTimeout indicates termination due to inactivity.
	TerminateTimeout
)

func (r TerminateReason) String() string {
	switch r {
	case TerminateNormal:
		return "normal"
	case TerminateShutdown:
		return "shutdown"
	case TerminateError:
		return "error"
	case TerminateTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// BaseComponent provides default implementations for Component methods.
// Embed this in your components to avoid implementing unused methods.
type BaseComponent struct {
	socket  *Socket
	assigns *Assigns
}

// SetSocket sets the socket for the component (called by the framework).
func (bc *BaseComponent) SetSocket(s *Socket) {
	bc.socket = s
}

// Socket returns the component's socket connection.
func (bc *BaseComponent) Socket() *Socket {
	return bc.socket
}

// Assigns returns the component's assigns store.
func (bc *BaseComponent) Assigns() *Assigns {
	if bc.assigns == nil {
		bc.assigns = NewAssigns()
	}
	return bc.assigns
}

// Name returns an empty string (override in your component).
func (bc *BaseComponent) Name() string {
	return ""
}

// Mount does nothing by default.
func (bc *BaseComponent) Mount(ctx context.Context, params Params, session Session) error {
	return nil
}

// HandleEvent does nothing by default.
func (bc *BaseComponent) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	return nil
}

// HandleInfo does nothing by default.
func (bc *BaseComponent) HandleInfo(ctx context.Context, msg any) error {
	return nil
}

// Terminate does nothing by default.
func (bc *BaseComponent) Terminate(ctx context.Context, reason TerminateReason) error {
	return nil
}

// ComponentRegistry manages registered components.
type ComponentRegistry struct {
	components map[string]func() Component
}

// NewComponentRegistry creates a new component registry.
func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		components: make(map[string]func() Component),
	}
}

// Register adds a component factory to the registry.
func (r *ComponentRegistry) Register(name string, factory func() Component) {
	r.components[name] = factory
}

// Get retrieves a component factory by name.
func (r *ComponentRegistry) Get(name string) (func() Component, bool) {
	f, ok := r.components[name]
	return f, ok
}

// Create instantiates a new component by name.
func (r *ComponentRegistry) Create(name string) (Component, bool) {
	f, ok := r.components[name]
	if !ok {
		return nil, false
	}
	return f(), true
}

// TemplateProvider allows components to declare their template source
// for AST compilation and dependency analysis.
// Implementing this interface enables the diff engine to analyze
// which slots depend on which assigns fields.
type TemplateProvider interface {
	// TemplateSource returns the raw template source for compilation.
	// This is used by the diff engine to build an AST and track dependencies.
	TemplateSource() string
}

// ListItem represents an item in a keyed list for efficient diffing.
// Used by ListProvider to enable insert/delete/move/update operations
// instead of full list re-renders.
type ListItem struct {
	Key     string // Unique identifier for the item
	Content string // Rendered HTML content
}

// ListProvider allows components to declare keyed lists for efficient diffing.
// Components implementing this interface get granular list updates
// (insert, delete, move, update) instead of full list re-renders.
//
// Example implementation:
//
//	func (t *TodoList) GetLists() map[string][]ListItem {
//	    items := make([]ListItem, len(t.Todos))
//	    for i, todo := range t.Todos {
//	        items[i] = ListItem{
//	            Key:     todo.ID,
//	            Content: fmt.Sprintf(`<li data-key="%s">%s</li>`, todo.ID, todo.Text),
//	        }
//	    }
//	    return map[string][]ListItem{"todos": items}
//	}
type ListProvider interface {
	// GetLists returns the current state of all keyed lists.
	// key = listID (matches data-list="listID" in HTML)
	// value = slice of ListItems with unique keys
	GetLists() map[string][]ListItem
}

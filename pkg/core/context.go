package core

import (
	"context"
)

// Context keys for storing values in context.
type contextKey string

const (
	socketKey    contextKey = "golivekit:socket"
	componentKey contextKey = "golivekit:component"
	assignsKey   contextKey = "golivekit:assigns"
	sessionKey   contextKey = "golivekit:session"
	paramsKey    contextKey = "golivekit:params"
	flashKey     contextKey = "golivekit:flash"
)

// WithSocket adds a socket to the context.
func WithSocket(ctx context.Context, socket *Socket) context.Context {
	return context.WithValue(ctx, socketKey, socket)
}

// SocketFromContext retrieves the socket from context.
func SocketFromContext(ctx context.Context) *Socket {
	s, _ := ctx.Value(socketKey).(*Socket)
	return s
}

// WithComponent adds a component to the context.
func WithComponent(ctx context.Context, comp Component) context.Context {
	return context.WithValue(ctx, componentKey, comp)
}

// ComponentFromContext retrieves the component from context.
func ComponentFromContext(ctx context.Context) Component {
	c, _ := ctx.Value(componentKey).(Component)
	return c
}

// WithAssigns adds assigns to the context.
func WithAssigns(ctx context.Context, assigns *Assigns) context.Context {
	return context.WithValue(ctx, assignsKey, assigns)
}

// AssignsFromContext retrieves assigns from context.
func AssignsFromContext(ctx context.Context) *Assigns {
	a, _ := ctx.Value(assignsKey).(*Assigns)
	return a
}

// WithSession adds session data to the context.
func WithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// SessionFromContext retrieves session from context.
func SessionFromContext(ctx context.Context) Session {
	s, _ := ctx.Value(sessionKey).(Session)
	return s
}

// WithParams adds params to the context.
func WithParams(ctx context.Context, params Params) context.Context {
	return context.WithValue(ctx, paramsKey, params)
}

// ParamsFromContext retrieves params from context.
func ParamsFromContext(ctx context.Context) Params {
	p, _ := ctx.Value(paramsKey).(Params)
	return p
}

// Flash represents flash messages for the current request.
type Flash struct {
	Info    []string
	Error   []string
	Warning []string
	Success []string
}

// WithFlash adds flash messages to the context.
func WithFlash(ctx context.Context, flash *Flash) context.Context {
	return context.WithValue(ctx, flashKey, flash)
}

// FlashFromContext retrieves flash messages from context.
func FlashFromContext(ctx context.Context) *Flash {
	f, _ := ctx.Value(flashKey).(*Flash)
	if f == nil {
		return &Flash{}
	}
	return f
}

// PutFlash adds a flash message of the specified type.
func PutFlash(ctx context.Context, flashType, message string) context.Context {
	flash := FlashFromContext(ctx)

	switch flashType {
	case "info":
		flash.Info = append(flash.Info, message)
	case "error":
		flash.Error = append(flash.Error, message)
	case "warning":
		flash.Warning = append(flash.Warning, message)
	case "success":
		flash.Success = append(flash.Success, message)
	}

	return WithFlash(ctx, flash)
}

// BuildContext creates a fully populated context for rendering.
func BuildContext(ctx context.Context, socket *Socket, comp Component, session Session, params Params) context.Context {
	ctx = WithSocket(ctx, socket)
	ctx = WithComponent(ctx, comp)
	ctx = WithAssigns(ctx, socket.Assigns())
	ctx = WithSession(ctx, session)
	ctx = WithParams(ctx, params)
	ctx = WithFlash(ctx, &Flash{})
	return ctx
}

// Event represents a user interaction event.
type Event struct {
	Type    string         `json:"type"`
	Target  string         `json:"target"`
	Payload map[string]any `json:"payload,omitempty"`
}

// RenderData contains data passed to render hooks.
type RenderData struct {
	HTML      string
	Slots     map[string]string
	Duration  int64 // nanoseconds
	BytesSent int
}

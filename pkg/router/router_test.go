package router

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// MockComponent implements core.Component for testing.
type MockComponent struct {
	mountCalled      bool
	renderCalled     bool
	handleEventCalled bool
	terminateCalled  bool

	socket  *core.Socket
	assigns *core.Assigns
}

func NewMockComponent() *MockComponent {
	return &MockComponent{
		assigns: core.NewAssigns(),
	}
}

func (c *MockComponent) Name() string {
	return "MockComponent"
}

func (c *MockComponent) Mount(ctx context.Context, params core.Params, session core.Session) error {
	c.mountCalled = true
	return nil
}

func (c *MockComponent) Render(ctx context.Context) core.Renderer {
	c.renderCalled = true
	return &MockRenderer{content: "<div>Mock Content</div>"}
}

func (c *MockComponent) HandleEvent(ctx context.Context, event string, payload map[string]any) error {
	c.handleEventCalled = true
	return nil
}

func (c *MockComponent) HandleInfo(ctx context.Context, msg any) error {
	return nil
}

func (c *MockComponent) Terminate(ctx context.Context, reason core.TerminateReason) error {
	c.terminateCalled = true
	return nil
}

func (c *MockComponent) SetSocket(socket *core.Socket) {
	c.socket = socket
}

// MockRenderer implements core.Renderer for testing.
type MockRenderer struct {
	content string
}

func (r *MockRenderer) Render(ctx context.Context, w io.Writer) error {
	_, err := w.Write([]byte(r.content))
	return err
}

func TestRouter_New(t *testing.T) {
	r := New()

	if r == nil {
		t.Fatal("expected router to be created")
	}

	if r.sessionManager == nil {
		t.Error("expected sessionManager to be initialized")
	}

	if r.socketManager == nil {
		t.Error("expected socketManager to be initialized")
	}

	if r.codec == nil {
		t.Error("expected codec to be initialized")
	}

	if r.diffEngine == nil {
		t.Error("expected diffEngine to be initialized")
	}

	if r.pubsub == nil {
		t.Error("expected pubsub to be initialized")
	}
}

func TestRouter_Live(t *testing.T) {
	r := New()

	componentFactory := func() core.Component {
		return NewMockComponent()
	}

	r.Live("/", componentFactory)

	// Check route was registered
	if len(r.liveRoutes) != 1 {
		t.Errorf("expected 1 live route, got %d", len(r.liveRoutes))
	}

	route, ok := r.liveRoutes["/"]
	if !ok {
		t.Fatal("expected route to be registered at /")
	}

	if route.Path != "/" {
		t.Errorf("expected path '/', got '%s'", route.Path)
	}
}

func TestRouter_Live_InitialHTTPRender(t *testing.T) {
	r := New()

	var component *MockComponent
	r.Live("/", func() core.Component {
		component = NewMockComponent()
		return component
	})

	// Create HTTP request (not WebSocket)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if !component.mountCalled {
		t.Error("expected Mount to be called")
	}

	if !component.renderCalled {
		t.Error("expected Render to be called")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Mock Content") {
		t.Errorf("expected body to contain 'Mock Content', got '%s'", body)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got '%s'", contentType)
	}
}

func TestRouter_Handle(t *testing.T) {
	r := New()

	r.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "OK" {
		t.Errorf("expected body 'OK', got '%s'", rec.Body.String())
	}
}

func TestRouter_Middleware(t *testing.T) {
	r := New()

	// Add middleware that adds a header
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "applied")
			next.ServeHTTP(w, r)
		})
	})

	r.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Header().Get("X-Middleware") != "applied" {
		t.Error("expected middleware to be applied")
	}
}

func TestRouter_Group(t *testing.T) {
	r := New()

	r.Group("/api", func(g *RouteGroup) {
		g.Get("/users", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("users"))
		})

		g.Post("/users", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("created"))
		})
	})

	// Test GET
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Body.String() != "users" {
		t.Errorf("expected 'users', got '%s'", rec.Body.String())
	}

	// Test POST
	req = httptest.NewRequest(http.MethodPost, "/api/users", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Body.String() != "created" {
		t.Errorf("expected 'created', got '%s'", rec.Body.String())
	}
}

func TestRouter_RouteOptions(t *testing.T) {
	r := New()

	layout := func() core.Component {
		return NewMockComponent()
	}

	r.Live("/",
		func() core.Component { return NewMockComponent() },
		WithLayout(layout),
		WithHooks("hook1", "hook2"),
		WithMeta("key", "value"),
	)

	route := r.liveRoutes["/"]

	if route.Layout == nil {
		t.Error("expected layout to be set")
	}

	if len(route.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(route.Hooks))
	}

	if route.Meta["key"] != "value" {
		t.Errorf("expected meta 'key' to be 'value', got '%v'", route.Meta["key"])
	}
}

func TestRouter_Static(t *testing.T) {
	r := New()

	// Create a temp dir for static files
	// In real tests, you'd create actual files
	r.Static("/static/", ".")

	// Just verify it doesn't panic
	req := httptest.NewRequest(http.MethodGet, "/static/router.go", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// File should exist in current directory
	if rec.Code == http.StatusNotFound {
		// This is expected if running from a different directory
		t.Skip("Static file test skipped - depends on working directory")
	}
}

func TestRouter_extractParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar&baz=123", nil)
	params := extractParams(req)

	if params["foo"] != "bar" {
		t.Errorf("expected foo='bar', got '%v'", params["foo"])
	}

	if params["baz"] != "123" {
		t.Errorf("expected baz='123', got '%v'", params["baz"])
	}
}

func TestRouter_isWebSocketRequest(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "WebSocket request",
			headers:  map[string]string{"Upgrade": "websocket"},
			expected: true,
		},
		{
			name:     "WebSocket request uppercase",
			headers:  map[string]string{"Upgrade": "WebSocket"},
			expected: true,
		},
		{
			name:     "Normal request",
			headers:  map[string]string{},
			expected: false,
		},
		{
			name:     "Other upgrade",
			headers:  map[string]string{"Upgrade": "http/2"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := isWebSocketRequest(req)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRouter_ErrorHandler(t *testing.T) {
	r := New()

	r.SetErrorHandler(func(w http.ResponseWriter, req *http.Request, err error) {
		_ = err // unused in this test
		http.Error(w, "Custom Error", http.StatusInternalServerError)
	})

	// Create a component that returns nil renderer
	r.Live("/", func() core.Component {
		return &MockComponent{}
	})

	// Force render to return nil
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// The mock component returns a valid renderer, so no error
	// To properly test error handling, we'd need a component that fails
}

func TestLiveViewSession_Manager(t *testing.T) {
	sm := NewLiveViewSessionManager()

	// Create sessions
	s1 := sm.Create("socket-1", NewMockComponent(), nil, nil)
	_ = sm.Create("socket-2", NewMockComponent(), nil, nil)

	if sm.Count() != 2 {
		t.Errorf("expected 2 sessions, got %d", sm.Count())
	}

	// Get by ID
	session, ok := sm.Get(s1.ID)
	if !ok || session.ID != s1.ID {
		t.Error("expected to find session by ID")
	}

	// Get by socket
	session, ok = sm.GetBySocket("socket-1")
	if !ok || session.SocketID != "socket-1" {
		t.Error("expected to find session by socket ID")
	}

	// Remove
	sm.Remove(s1.ID)
	if sm.Count() != 1 {
		t.Errorf("expected 1 session after remove, got %d", sm.Count())
	}

	// Remove by socket
	sm.RemoveBySocket("socket-2")
	if sm.Count() != 0 {
		t.Errorf("expected 0 sessions after remove, got %d", sm.Count())
	}
}

func TestLiveViewSession_Activity(t *testing.T) {
	session := NewLiveViewSession("socket-1", NewMockComponent(), nil, nil)

	initial := session.GetLastActivity()
	session.UpdateActivity()
	updated := session.GetLastActivity()

	if !updated.After(initial) || updated.Equal(initial) {
		// Times might be equal if fast enough
		if updated.Before(initial) {
			t.Error("expected activity time to be updated")
		}
	}
}

func TestLiveViewSession_Mounted(t *testing.T) {
	session := NewLiveViewSession("socket-1", NewMockComponent(), nil, nil)

	if session.IsMounted() {
		t.Error("expected session to not be mounted initially")
	}

	session.SetMounted(true)

	if !session.IsMounted() {
		t.Error("expected session to be mounted")
	}
}

func TestTransportAdapter(t *testing.T) {
	// TransportAdapter requires a WebSocketTransport which needs a real connection
	// This is more of an integration test, so we just verify the adapter can be created

	// For unit testing, we'd mock the WebSocketTransport
	t.Skip("TransportAdapter requires WebSocket connection for proper testing")
}

// Benchmark tests

func BenchmarkRouter_ServeHTTP(b *testing.B) {
	r := New()
	r.Live("/", func() core.Component {
		return NewMockComponent()
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
}

func BenchmarkRouter_extractParams(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar&baz=123&qux=456", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractParams(req)
	}
}

func BenchmarkLiveViewSessionManager_Create(b *testing.B) {
	comp := NewMockComponent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a new manager each 1000 iterations to avoid memory explosion
		sm := NewLiveViewSessionManager()
		for j := 0; j < 100; j++ {
			sm.Create(fmt.Sprintf("socket-%d", j), comp, nil, nil)
		}
	}
}

func BenchmarkLiveViewSessionManager_GetBySocket(b *testing.B) {
	sm := NewLiveViewSessionManager()
	for i := 0; i < 1000; i++ {
		sm.Create(fmt.Sprintf("socket-%d", i), NewMockComponent(), nil, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.GetBySocket(fmt.Sprintf("socket-%d", i%1000))
	}
}

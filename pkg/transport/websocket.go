package transport

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Debug flag
var DebugWebSocket = false

func init() {
	// Enable debug for testing
	// DebugWebSocket = true
}

// WebSocket security errors
var (
	ErrOriginNotAllowed = errors.New("origin not allowed")
	ErrOriginInvalid    = errors.New("invalid origin header")
)

// WebSocketConfig configures WebSocket security settings.
type WebSocketConfig struct {
	// AllowedOrigins is a list of allowed origins for WebSocket connections.
	// If empty and InsecureDevMode is false, only same-origin connections are allowed.
	AllowedOrigins []string

	// InsecureDevMode disables origin validation (ONLY for development).
	// WARNING: Never enable this in production!
	InsecureDevMode bool
}

// DefaultWebSocketConfig returns secure default configuration.
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		AllowedOrigins:  nil,
		InsecureDevMode: false,
	}
}

// WebSocketTransport implements Transport using WebSocket.
type WebSocketTransport struct {
	*BaseTransport
	conn       *websocket.Conn
	url        string
	headers    http.Header
	wsConfig   *WebSocketConfig
	mu         sync.Mutex
}

// NewWebSocketTransport creates a new WebSocket transport.
func NewWebSocketTransport(config *TransportConfig) *WebSocketTransport {
	return &WebSocketTransport{
		BaseTransport: NewBaseTransport(config),
		headers:       make(http.Header),
		wsConfig:      DefaultWebSocketConfig(),
	}
}

// NewWebSocketTransportWithConfig creates a WebSocket transport with security config.
func NewWebSocketTransportWithConfig(config *TransportConfig, wsConfig *WebSocketConfig) *WebSocketTransport {
	if wsConfig == nil {
		wsConfig = DefaultWebSocketConfig()
	}
	return &WebSocketTransport{
		BaseTransport: NewBaseTransport(config),
		headers:       make(http.Header),
		wsConfig:      wsConfig,
	}
}

// SetWebSocketConfig updates the WebSocket security configuration.
func (t *WebSocketTransport) SetWebSocketConfig(config *WebSocketConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.wsConfig = config
}

// isOriginAllowed checks if the origin is allowed for WebSocket connections.
func (t *WebSocketTransport) isOriginAllowed(origin string, requestHost string) bool {
	// If InsecureDevMode is enabled, allow all origins (development only!)
	if t.wsConfig != nil && t.wsConfig.InsecureDevMode {
		return true
	}

	// Empty origin = same-origin request (allowed)
	if origin == "" {
		return true
	}

	// Parse origin URL
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// Same-origin check: origin host matches request host
	if originURL.Host == requestHost {
		return true
	}

	// Check against allowed origins list
	if t.wsConfig != nil {
		for _, allowed := range t.wsConfig.AllowedOrigins {
			if allowed == "*" {
				return true // Explicit wildcard
			}
			if allowed == origin {
				return true
			}
			// Also check without protocol
			if allowedURL, err := url.Parse(allowed); err == nil {
				if allowedURL.Host == originURL.Host {
					return true
				}
			}
		}
	}

	return false
}

// Type returns the transport type.
func (t *WebSocketTransport) Type() TransportType {
	return TransportWebSocket
}

// SetURL sets the WebSocket URL for client-side connections.
func (t *WebSocketTransport) SetURL(url string) {
	t.url = url
}

// SetHeader sets a header for the connection.
func (t *WebSocketTransport) SetHeader(key, value string) {
	t.headers.Set(key, value)
}

// Connect establishes a WebSocket connection (client-side).
func (t *WebSocketTransport) Connect(ctx context.Context) error {
	if t.url == "" {
		return fmt.Errorf("websocket URL not set")
	}

	opts := &websocket.DialOptions{
		HTTPHeader: t.headers,
	}

	conn, _, err := websocket.Dial(ctx, t.url, opts)
	if err != nil {
		return fmt.Errorf("dial websocket: %w", err)
	}

	t.mu.Lock()
	t.conn = conn
	t.SetConnected(true)
	t.mu.Unlock()

	// Set read limit
	conn.SetReadLimit(t.config.MaxMessageSize)

	// Start read/write loops
	go t.readLoop()
	go t.writeLoop()
	go t.pingLoop()

	return nil
}

// Upgrade upgrades an HTTP connection to WebSocket (server-side).
// Validates origin header to prevent WebSocket hijacking attacks.
func (t *WebSocketTransport) Upgrade(w http.ResponseWriter, r *http.Request) error {
	// Validate origin to prevent CSRF over WebSocket
	origin := r.Header.Get("Origin")
	if !t.isOriginAllowed(origin, r.Host) {
		http.Error(w, "Forbidden: Origin not allowed", http.StatusForbidden)
		return ErrOriginNotAllowed
	}

	// Only skip origin verification if explicitly in dev mode
	insecureSkip := t.wsConfig != nil && t.wsConfig.InsecureDevMode

	opts := &websocket.AcceptOptions{
		InsecureSkipVerify: insecureSkip,
	}

	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		return fmt.Errorf("accept websocket: %w", err)
	}

	t.mu.Lock()
	t.conn = conn
	t.SetConnected(true)
	t.mu.Unlock()

	// Set read limit
	conn.SetReadLimit(t.config.MaxMessageSize)

	// Start read/write loops
	go t.readLoop()
	go t.writeLoop()
	go t.pingLoop()

	return nil
}

// Send sends a message over the WebSocket.
func (t *WebSocketTransport) Send(msg Message) error {
	if !t.IsConnected() {
		return ErrNotConnected
	}

	select {
	case t.sendCh <- msg:
		return nil
	case <-t.closeCh:
		return ErrConnectionClosed
	case <-time.After(t.config.WriteTimeout):
		return ErrSendTimeout
	}
}

// Close closes the WebSocket connection.
func (t *WebSocketTransport) Close() error {
	t.BaseTransport.Close()

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn != nil {
		err := t.conn.Close(websocket.StatusNormalClosure, "closing")
		t.conn = nil
		return err
	}
	return nil
}

// readLoop reads messages from the WebSocket.
func (t *WebSocketTransport) readLoop() {
	defer t.Close()

	for {
		select {
		case <-t.closeCh:
			return
		default:
		}

		t.mu.Lock()
		conn := t.conn
		t.mu.Unlock()

		if conn == nil {
			return
		}

		// Set read deadline
		ctx, cancel := context.WithTimeout(context.Background(), t.config.ReadTimeout)

		_, data, err := conn.Read(ctx)
		cancel()

		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			// Log error or notify handler
			return
		}

		msg, err := Unmarshal(data)
		if err != nil {
			if DebugWebSocket {
				log.Printf("[WS DEBUG] Unmarshal error: %v, data: %s\n", err, string(data))
			}
			continue // Skip invalid messages
		}

		if DebugWebSocket {
			log.Printf("[WS DEBUG] Received: event=%s, topic=%s, ref=%s\n", msg.Event, msg.Topic, msg.Ref)
		}

		// Handle special messages
		if msg.Event == "ping" {
			t.sendPong()
			continue
		}

		// Push to receive channel
		select {
		case t.recvCh <- msg:
			if DebugWebSocket {
				log.Printf("[WS DEBUG] Message pushed to recvCh: event=%s\n", msg.Event)
			}
		case <-t.closeCh:
			if DebugWebSocket {
				log.Printf("[WS DEBUG] closeCh received, returning from readLoop\n")
			}
			return
		default:
			if DebugWebSocket {
				log.Printf("[WS DEBUG] Channel full, dropping message: event=%s\n", msg.Event)
			}
		}
	}
}

// writeLoop writes messages to the WebSocket.
func (t *WebSocketTransport) writeLoop() {
	for {
		select {
		case msg := <-t.sendCh:
			t.mu.Lock()
			conn := t.conn
			t.mu.Unlock()

			if conn == nil {
				return
			}

			data, err := msg.Marshal()
			if err != nil {
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), t.config.WriteTimeout)
			err = conn.Write(ctx, websocket.MessageText, data)
			cancel()

			if err != nil {
				return
			}

		case <-t.closeCh:
			return
		}
	}
}

// pingLoop sends periodic pings to keep the connection alive.
func (t *WebSocketTransport) pingLoop() {
	ticker := time.NewTicker(t.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.sendPing()
		case <-t.closeCh:
			return
		}
	}
}

// sendPing sends a ping message.
func (t *WebSocketTransport) sendPing() {
	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), t.config.WriteTimeout)
	defer cancel()

	conn.Ping(ctx)
}

// sendPong sends a pong response.
func (t *WebSocketTransport) sendPong() {
	msg := NewMessage("phoenix", "phx_reply", map[string]any{
		"status": "ok",
	})

	select {
	case t.sendCh <- msg:
	default:
	}
}

// Conn returns the underlying WebSocket connection.
func (t *WebSocketTransport) Conn() *websocket.Conn {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn
}

// WebSocketHandler handles WebSocket upgrade requests.
type WebSocketHandler struct {
	config   *TransportConfig
	onAccept func(t *WebSocketTransport)
}

// NewWebSocketHandler creates a new WebSocket handler.
func NewWebSocketHandler(config *TransportConfig, onAccept func(t *WebSocketTransport)) *WebSocketHandler {
	if config == nil {
		config = DefaultTransportConfig()
	}
	return &WebSocketHandler{
		config:   config,
		onAccept: onAccept,
	}
}

// ServeHTTP handles HTTP requests and upgrades to WebSocket.
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := NewWebSocketTransport(h.config)

	if err := t.Upgrade(w, r); err != nil {
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}

	if h.onAccept != nil {
		h.onAccept(t)
	}
}

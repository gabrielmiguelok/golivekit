package transport

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SSEConfig configures SSE security settings.
type SSEConfig struct {
	// AllowedOrigins is a list of allowed origins for CORS.
	// If empty, no CORS headers are set (same-origin only).
	AllowedOrigins []string

	// AllowCredentials allows credentials in CORS requests.
	AllowCredentials bool
}

// DefaultSSEConfig returns secure default configuration (no CORS).
func DefaultSSEConfig() *SSEConfig {
	return &SSEConfig{
		AllowedOrigins:   nil,
		AllowCredentials: false,
	}
}

// SSETransport implements Transport using Server-Sent Events.
// This is a fallback for environments where WebSocket is not available.
type SSETransport struct {
	*BaseTransport
	writer    http.ResponseWriter
	flusher   http.Flusher
	postURL   string
	clientID  string
	client    *http.Client
	eventID   int64
	sseConfig *SSEConfig
	mu        sync.Mutex
}

// NewSSETransport creates a new SSE transport.
func NewSSETransport(config *TransportConfig) *SSETransport {
	return &SSETransport{
		BaseTransport: NewBaseTransport(config),
		client: &http.Client{
			Timeout: config.WriteTimeout,
		},
		sseConfig: DefaultSSEConfig(),
	}
}

// NewSSETransportWithConfig creates an SSE transport with security config.
func NewSSETransportWithConfig(config *TransportConfig, sseConfig *SSEConfig) *SSETransport {
	if sseConfig == nil {
		sseConfig = DefaultSSEConfig()
	}
	return &SSETransport{
		BaseTransport: NewBaseTransport(config),
		client: &http.Client{
			Timeout: config.WriteTimeout,
		},
		sseConfig: sseConfig,
	}
}

// SetSSEConfig updates the SSE security configuration.
func (t *SSETransport) SetSSEConfig(config *SSEConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sseConfig = config
}

// isOriginAllowed checks if the origin is in the allowed list.
func (t *SSETransport) isOriginAllowed(origin string) bool {
	if t.sseConfig == nil || len(t.sseConfig.AllowedOrigins) == 0 {
		return false
	}
	for _, allowed := range t.sseConfig.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// Type returns the transport type.
func (t *SSETransport) Type() TransportType {
	return TransportSSE
}

// SetPostURL sets the URL for sending messages back to the server.
func (t *SSETransport) SetPostURL(url string) {
	t.postURL = url
}

// SetClientID sets the client identifier.
func (t *SSETransport) SetClientID(id string) {
	t.clientID = id
}

// Connect is not used for SSE (server pushes to client).
func (t *SSETransport) Connect(ctx context.Context) error {
	return fmt.Errorf("SSE transport does not support Connect; use ServeHTTP instead")
}

// ServeHTTP handles SSE requests.
func (t *SSETransport) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// SECURITY FIX: Only set CORS headers if origin is explicitly allowed
	// Never use wildcard "*" by default
	origin := r.Header.Get("Origin")
	if origin != "" && t.isOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if t.sseConfig != nil && t.sseConfig.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
	}
	// If origin is not allowed, don't set any CORS headers (blocks cross-origin)

	t.mu.Lock()
	t.writer = w
	t.flusher = flusher
	t.SetConnected(true)
	t.mu.Unlock()

	// Send initial connection event
	t.sendEvent("connected", map[string]any{
		"client_id": t.clientID,
	})

	// Start write loop
	go t.writeLoop()

	// Keep connection open until closed
	<-r.Context().Done()

	t.Close()
	return nil
}

// Send queues a message to be sent.
func (t *SSETransport) Send(msg Message) error {
	if !t.IsConnected() {
		return ErrNotConnected
	}

	select {
	case t.sendCh <- msg:
		return nil
	case <-t.closeCh:
		return ErrConnectionClosed
	default:
		return ErrTransportFull
	}
}

// Close closes the SSE connection.
func (t *SSETransport) Close() error {
	return t.BaseTransport.Close()
}

// writeLoop sends messages as SSE events.
func (t *SSETransport) writeLoop() {
	ticker := time.NewTicker(t.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg := <-t.sendCh:
			t.sendEvent(msg.Event, msg.Payload)

		case <-ticker.C:
			// Send heartbeat
			t.sendHeartbeat()

		case <-t.closeCh:
			return
		}
	}
}

// sendEvent sends an SSE event.
func (t *SSETransport) sendEvent(event string, data any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.writer == nil {
		return ErrNotConnected
	}

	t.eventID++

	// Format SSE event
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("id: %d\n", t.eventID))
	sb.WriteString(fmt.Sprintf("event: %s\n", event))

	// Serialize data as JSON
	if data != nil {
		dataJSON, err := NewMessage("", event, data.(map[string]any)).Marshal()
		if err == nil {
			sb.WriteString(fmt.Sprintf("data: %s\n", string(dataJSON)))
		}
	}

	sb.WriteString("\n")

	_, err := fmt.Fprint(t.writer, sb.String())
	if err != nil {
		return err
	}

	t.flusher.Flush()
	return nil
}

// sendHeartbeat sends a heartbeat event.
func (t *SSETransport) sendHeartbeat() {
	t.sendEvent("heartbeat", map[string]any{
		"time": time.Now().Unix(),
	})
}

// ReceiveFromPost processes incoming messages from POST requests.
// SSE is unidirectional, so clients send messages via POST.
func (t *SSETransport) ReceiveFromPost(r *http.Request) error {
	scanner := bufio.NewScanner(r.Body)
	defer r.Body.Close()

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		msg, err := Unmarshal([]byte(line))
		if err != nil {
			continue
		}

		select {
		case t.recvCh <- msg:
		case <-t.closeCh:
			return ErrConnectionClosed
		default:
			// Channel full
		}
	}

	return scanner.Err()
}

// SSEHandler handles SSE connections.
type SSEHandler struct {
	config    *TransportConfig
	transports map[string]*SSETransport
	onAccept  func(t *SSETransport)
	mu        sync.RWMutex
}

// NewSSEHandler creates a new SSE handler.
func NewSSEHandler(config *TransportConfig, onAccept func(t *SSETransport)) *SSEHandler {
	if config == nil {
		config = DefaultTransportConfig()
	}
	return &SSEHandler{
		config:     config,
		transports: make(map[string]*SSETransport),
		onAccept:   onAccept,
	}
}

// ServeHTTP handles SSE requests.
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		http.Error(w, "client_id required", http.StatusBadRequest)
		return
	}

	t := NewSSETransport(h.config)
	t.SetClientID(clientID)

	h.mu.Lock()
	h.transports[clientID] = t
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.transports, clientID)
		h.mu.Unlock()
	}()

	if h.onAccept != nil {
		h.onAccept(t)
	}

	t.ServeHTTP(w, r)
}

// HandlePost handles POST requests for sending messages.
func (h *SSEHandler) HandlePost(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		http.Error(w, "client_id required", http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	t, ok := h.transports[clientID]
	h.mu.RUnlock()

	if !ok {
		http.Error(w, "transport not found", http.StatusNotFound)
		return
	}

	if err := t.ReceiveFromPost(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Get retrieves a transport by client ID.
func (h *SSEHandler) Get(clientID string) (*SSETransport, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	t, ok := h.transports[clientID]
	return t, ok
}

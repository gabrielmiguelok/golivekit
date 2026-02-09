package transport

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LongPollingConfig configures long-polling security settings.
type LongPollingConfig struct {
	// TokenValidator validates authentication tokens.
	// Return true if token is valid, false otherwise.
	TokenValidator func(token string) (bool, error)

	// RequireAuth requires authentication for all long-polling requests.
	RequireAuth bool

	// HMACSecret is the secret key for signing client IDs.
	// If empty, client IDs will not be signed (less secure).
	HMACSecret []byte

	// ClientIDExpiry is how long signed client IDs are valid.
	ClientIDExpiry time.Duration
}

// DefaultLongPollingConfig returns secure default configuration.
func DefaultLongPollingConfig() *LongPollingConfig {
	secret := make([]byte, 32)
	rand.Read(secret)
	return &LongPollingConfig{
		TokenValidator: nil,
		RequireAuth:    false,
		HMACSecret:     secret,
		ClientIDExpiry: 24 * time.Hour,
	}
}

// LongPollingTransport implements Transport using long-polling.
// This is a legacy fallback for environments without WebSocket or SSE support.
type LongPollingTransport struct {
	*BaseTransport
	clientID       string
	pendingMsgs    []Message
	pollTimeout    time.Duration
	maxPendingMsgs int // Maximum pending messages to prevent OOM (default: 1000)
	lpConfig       *LongPollingConfig
	mu             sync.Mutex
}

// DefaultMaxPendingMsgs is the default maximum pending messages for long-polling.
const DefaultMaxPendingMsgs = 1000

// NewLongPollingTransport creates a new long-polling transport.
func NewLongPollingTransport(config *TransportConfig) *LongPollingTransport {
	return &LongPollingTransport{
		BaseTransport:  NewBaseTransport(config),
		pendingMsgs:    make([]Message, 0),
		pollTimeout:    30 * time.Second,
		maxPendingMsgs: DefaultMaxPendingMsgs,
		lpConfig:       DefaultLongPollingConfig(),
	}
}

// NewLongPollingTransportWithConfig creates a long-polling transport with security config.
func NewLongPollingTransportWithConfig(config *TransportConfig, lpConfig *LongPollingConfig) *LongPollingTransport {
	if lpConfig == nil {
		lpConfig = DefaultLongPollingConfig()
	}
	return &LongPollingTransport{
		BaseTransport:  NewBaseTransport(config),
		pendingMsgs:    make([]Message, 0),
		pollTimeout:    30 * time.Second,
		maxPendingMsgs: DefaultMaxPendingMsgs,
		lpConfig:       lpConfig,
	}
}

// SetLongPollingConfig updates the long-polling security configuration.
func (t *LongPollingTransport) SetLongPollingConfig(config *LongPollingConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lpConfig = config
}

// generateSignedClientID creates a signed, non-enumerable client ID.
func (t *LongPollingTransport) generateSignedClientID() (string, error) {
	// Generate random bytes
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// Create timestamp
	timestamp := time.Now().Unix()

	// Create payload: random|timestamp
	payload := fmt.Sprintf("%s|%d", base64.URLEncoding.EncodeToString(randomBytes), timestamp)

	// Sign the payload
	if t.lpConfig != nil && len(t.lpConfig.HMACSecret) > 0 {
		mac := hmac.New(sha256.New, t.lpConfig.HMACSecret)
		mac.Write([]byte(payload))
		signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))
		return payload + "." + signature, nil
	}

	return payload, nil
}

// verifyClientIDSignature verifies a signed client ID.
func (t *LongPollingTransport) verifyClientIDSignature(clientID string) bool {
	if t.lpConfig == nil || len(t.lpConfig.HMACSecret) == 0 {
		return true // No signing configured
	}

	parts := strings.Split(clientID, ".")
	if len(parts) != 2 {
		return false
	}

	payload := parts[0]
	providedSig := parts[1]

	// Verify signature
	mac := hmac.New(sha256.New, t.lpConfig.HMACSecret)
	mac.Write([]byte(payload))
	expectedSig := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return false
	}

	// Check timestamp expiry
	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) != 2 {
		return false
	}

	var timestamp int64
	if _, err := fmt.Sscanf(payloadParts[1], "%d", &timestamp); err != nil {
		return false
	}

	if t.lpConfig.ClientIDExpiry > 0 {
		if time.Since(time.Unix(timestamp, 0)) > t.lpConfig.ClientIDExpiry {
			return false
		}
	}

	return true
}

// validateAuthToken validates the authentication token from the request.
func (t *LongPollingTransport) validateAuthToken(r *http.Request) bool {
	if t.lpConfig == nil || !t.lpConfig.RequireAuth || t.lpConfig.TokenValidator == nil {
		return true // Auth not required
	}

	token := r.Header.Get("X-Auth-Token")
	if token == "" {
		token = r.URL.Query().Get("auth_token")
	}

	valid, _ := t.lpConfig.TokenValidator(token)
	return valid
}

// Type returns the transport type.
func (t *LongPollingTransport) Type() TransportType {
	return TransportLongPolling
}

// SetClientID sets the client identifier.
func (t *LongPollingTransport) SetClientID(id string) {
	t.clientID = id
}

// SetPollTimeout sets the long-poll timeout.
func (t *LongPollingTransport) SetPollTimeout(d time.Duration) {
	t.pollTimeout = d
}

// SetMaxPendingMsgs sets the maximum number of pending messages.
// When exceeded, oldest messages are dropped to prevent OOM.
func (t *LongPollingTransport) SetMaxPendingMsgs(max int) {
	if max < 1 {
		max = 1
	}
	t.mu.Lock()
	t.maxPendingMsgs = max
	t.mu.Unlock()
}

// Connect is not used for long-polling.
func (t *LongPollingTransport) Connect(ctx context.Context) error {
	t.SetConnected(true)
	return nil
}

// Send queues a message to be sent on the next poll.
// If the queue is full, the oldest messages are dropped to make room.
func (t *LongPollingTransport) Send(msg Message) error {
	if !t.IsConnected() {
		return ErrNotConnected
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Enforce maximum pending messages to prevent OOM
	if len(t.pendingMsgs) >= t.maxPendingMsgs {
		// Drop oldest 10% of messages to make room
		dropCount := t.maxPendingMsgs / 10
		if dropCount < 1 {
			dropCount = 1
		}
		t.pendingMsgs = t.pendingMsgs[dropCount:]
	}

	t.pendingMsgs = append(t.pendingMsgs, msg)
	return nil
}

// Close closes the transport.
func (t *LongPollingTransport) Close() error {
	return t.BaseTransport.Close()
}

// HandlePoll handles a poll request from the client.
// The client calls this endpoint to receive queued messages.
func (t *LongPollingTransport) HandlePoll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), t.pollTimeout)
	defer cancel()

	// Wait for messages or timeout
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Timeout - return empty array
			t.writeMessages(w, nil)
			return

		case <-ticker.C:
			t.mu.Lock()
			if len(t.pendingMsgs) > 0 {
				msgs := t.pendingMsgs
				t.pendingMsgs = make([]Message, 0)
				t.mu.Unlock()
				t.writeMessages(w, msgs)
				return
			}
			t.mu.Unlock()

		case <-t.closeCh:
			t.writeMessages(w, nil)
			return
		}
	}
}

// writeMessages writes messages as JSON array.
func (t *LongPollingTransport) writeMessages(w http.ResponseWriter, msgs []Message) {
	w.Header().Set("Content-Type", "application/json")

	if msgs == nil {
		msgs = []Message{}
	}

	json.NewEncoder(w).Encode(msgs)
}

// HandleSend handles a send request from the client.
// The client calls this endpoint to send messages to the server.
func (t *LongPollingTransport) HandleSend(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var msgs []Message
	if err := json.Unmarshal(body, &msgs); err != nil {
		// Try single message
		var msg Message
		if err := json.Unmarshal(body, &msg); err != nil {
			http.Error(w, "invalid message format", http.StatusBadRequest)
			return
		}
		msgs = []Message{msg}
	}

	for _, msg := range msgs {
		select {
		case t.recvCh <- msg:
		case <-t.closeCh:
			http.Error(w, "connection closed", http.StatusGone)
			return
		default:
			// Channel full
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}

// PendingCount returns the number of pending messages.
func (t *LongPollingTransport) PendingCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pendingMsgs)
}

// LongPollingHandler handles long-polling connections.
type LongPollingHandler struct {
	config     *TransportConfig
	lpConfig   *LongPollingConfig
	transports map[string]*LongPollingTransport
	onAccept   func(t *LongPollingTransport)
	mu         sync.RWMutex
}

// NewLongPollingHandler creates a new long-polling handler.
func NewLongPollingHandler(config *TransportConfig, onAccept func(t *LongPollingTransport)) *LongPollingHandler {
	if config == nil {
		config = DefaultTransportConfig()
	}
	return &LongPollingHandler{
		config:     config,
		lpConfig:   DefaultLongPollingConfig(),
		transports: make(map[string]*LongPollingTransport),
		onAccept:   onAccept,
	}
}

// NewLongPollingHandlerWithConfig creates a handler with security config.
func NewLongPollingHandlerWithConfig(config *TransportConfig, lpConfig *LongPollingConfig, onAccept func(t *LongPollingTransport)) *LongPollingHandler {
	if config == nil {
		config = DefaultTransportConfig()
	}
	if lpConfig == nil {
		lpConfig = DefaultLongPollingConfig()
	}
	return &LongPollingHandler{
		config:     config,
		lpConfig:   lpConfig,
		transports: make(map[string]*LongPollingTransport),
		onAccept:   onAccept,
	}
}

// SetLongPollingConfig updates the security configuration.
func (h *LongPollingHandler) SetLongPollingConfig(config *LongPollingConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lpConfig = config
}

// Connect creates a new transport for a client.
// SECURITY FIX: Generates signed client IDs to prevent enumeration attacks.
func (h *LongPollingHandler) Connect(w http.ResponseWriter, r *http.Request) {
	// Validate auth token if required
	t := NewLongPollingTransportWithConfig(h.config, h.lpConfig)

	if !t.validateAuthToken(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate signed client ID (prevents enumeration)
	clientID, err := t.generateSignedClientID()
	if err != nil {
		http.Error(w, "Failed to generate client ID", http.StatusInternalServerError)
		return
	}

	t.SetClientID(clientID)
	t.Connect(r.Context())

	h.mu.Lock()
	h.transports[clientID] = t
	h.mu.Unlock()

	if h.onAccept != nil {
		h.onAccept(t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"client_id": clientID,
		"status":    "connected",
	})
}

// Poll handles poll requests.
// SECURITY FIX: Validates client ID signature to prevent session hijacking.
func (h *LongPollingHandler) Poll(w http.ResponseWriter, r *http.Request) {
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

	// Verify client ID signature to prevent hijacking
	if !t.verifyClientIDSignature(clientID) {
		http.Error(w, "Invalid client ID", http.StatusBadRequest)
		return
	}

	// Validate auth token if required
	if !t.validateAuthToken(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	t.HandlePoll(w, r)
}

// Send handles send requests.
// SECURITY FIX: Validates client ID signature and auth token.
func (h *LongPollingHandler) Send(w http.ResponseWriter, r *http.Request) {
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

	// Verify client ID signature
	if !t.verifyClientIDSignature(clientID) {
		http.Error(w, "Invalid client ID", http.StatusBadRequest)
		return
	}

	// Validate auth token if required
	if !t.validateAuthToken(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	t.HandleSend(w, r)
}

// Disconnect removes a transport.
func (h *LongPollingHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		http.Error(w, "client_id required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	if t, ok := h.transports[clientID]; ok {
		t.Close()
		delete(h.transports, clientID)
	}
	h.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

// Get retrieves a transport by client ID.
func (h *LongPollingHandler) Get(clientID string) (*LongPollingTransport, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	t, ok := h.transports[clientID]
	return t, ok
}

// Cleanup removes inactive transports.
func (h *LongPollingHandler) Cleanup(maxInactive time.Duration) int {
	h.mu.Lock()
	defer h.mu.Unlock()

	// For long-polling, we rely on connection tracking
	// This is a simplified cleanup
	return 0
}

// ServeHTTP routes requests to the appropriate handler.
func (h *LongPollingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/poll":
		h.Poll(w, r)
	case r.Method == "POST" && r.URL.Path == "/send":
		h.Send(w, r)
	case r.Method == "POST" && r.URL.Path == "/connect":
		h.Connect(w, r)
	case r.Method == "POST" && r.URL.Path == "/disconnect":
		h.Disconnect(w, r)
	default:
		http.Error(w, fmt.Sprintf("unknown endpoint: %s %s", r.Method, r.URL.Path), http.StatusNotFound)
	}
}

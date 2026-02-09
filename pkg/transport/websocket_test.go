package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebSocket_OriginValidation(t *testing.T) {
	config := DefaultTransportConfig()

	tests := []struct {
		name           string
		wsConfig       *WebSocketConfig
		origin         string
		host           string
		expectAllowed  bool
	}{
		{
			name: "same-origin allowed",
			wsConfig: &WebSocketConfig{
				AllowedOrigins:  nil,
				InsecureDevMode: false,
			},
			origin:        "https://example.com",
			host:          "example.com",
			expectAllowed: true,
		},
		{
			name: "no origin allowed",
			wsConfig: &WebSocketConfig{
				AllowedOrigins:  nil,
				InsecureDevMode: false,
			},
			origin:        "",
			host:          "example.com",
			expectAllowed: true,
		},
		{
			name: "explicit origin allowed",
			wsConfig: &WebSocketConfig{
				AllowedOrigins:  []string{"https://allowed.com"},
				InsecureDevMode: false,
			},
			origin:        "https://allowed.com",
			host:          "example.com",
			expectAllowed: true,
		},
		{
			name: "origin not in list blocked",
			wsConfig: &WebSocketConfig{
				AllowedOrigins:  []string{"https://allowed.com"},
				InsecureDevMode: false,
			},
			origin:        "https://attacker.com",
			host:          "example.com",
			expectAllowed: false,
		},
		{
			name: "wildcard allows all",
			wsConfig: &WebSocketConfig{
				AllowedOrigins:  []string{"*"},
				InsecureDevMode: false,
			},
			origin:        "https://any-site.com",
			host:          "example.com",
			expectAllowed: true,
		},
		{
			name: "insecure dev mode allows all",
			wsConfig: &WebSocketConfig{
				AllowedOrigins:  nil,
				InsecureDevMode: true,
			},
			origin:        "https://attacker.com",
			host:          "example.com",
			expectAllowed: true,
		},
		{
			name: "cross-origin blocked by default",
			wsConfig: &WebSocketConfig{
				AllowedOrigins:  nil,
				InsecureDevMode: false,
			},
			origin:        "https://other-site.com",
			host:          "example.com",
			expectAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewWebSocketTransportWithConfig(config, tt.wsConfig)

			allowed := transport.isOriginAllowed(tt.origin, tt.host)

			if allowed != tt.expectAllowed {
				t.Errorf("isOriginAllowed(%q, %q) = %v, want %v",
					tt.origin, tt.host, allowed, tt.expectAllowed)
			}
		})
	}
}

func TestWebSocket_RejectsInvalidOrigin(t *testing.T) {
	config := DefaultTransportConfig()
	wsConfig := &WebSocketConfig{
		AllowedOrigins:  []string{"https://allowed.com"},
		InsecureDevMode: false,
	}
	transport := NewWebSocketTransportWithConfig(config, wsConfig)

	// Create a mock request with invalid origin
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://attacker.com")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Host = "example.com"

	w := httptest.NewRecorder()

	err := transport.Upgrade(w, req)

	if err != ErrOriginNotAllowed {
		t.Errorf("Expected ErrOriginNotAllowed, got %v", err)
	}

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestWebSocket_AcceptsValidOrigin(t *testing.T) {
	config := DefaultTransportConfig()
	wsConfig := &WebSocketConfig{
		AllowedOrigins:  []string{"https://allowed.com"},
		InsecureDevMode: false,
	}
	transport := NewWebSocketTransportWithConfig(config, wsConfig)

	// Create a mock request with valid origin
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Origin", "https://allowed.com")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Host = "example.com"

	// Note: Full WebSocket upgrade test requires an actual WebSocket server
	// This test verifies origin validation logic
	allowed := transport.isOriginAllowed("https://allowed.com", "example.com")
	if !allowed {
		t.Error("Expected origin to be allowed")
	}
}

func TestDefaultWebSocketConfig(t *testing.T) {
	config := DefaultWebSocketConfig()

	if config.InsecureDevMode != false {
		t.Error("InsecureDevMode should be false by default")
	}

	if config.AllowedOrigins != nil {
		t.Error("AllowedOrigins should be nil by default (same-origin only)")
	}
}

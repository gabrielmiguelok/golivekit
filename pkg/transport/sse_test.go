package transport

import (
	"testing"
)

func TestSSE_CORSNotWildcard(t *testing.T) {
	// Verify default config doesn't allow all origins
	config := DefaultSSEConfig()

	if len(config.AllowedOrigins) != 0 {
		t.Error("Default SSE config should have empty AllowedOrigins (no CORS)")
	}

	if config.AllowCredentials {
		t.Error("Default SSE config should not allow credentials")
	}
}

func TestSSE_CORSValidatesOrigin(t *testing.T) {
	config := DefaultTransportConfig()

	tests := []struct {
		name          string
		sseConfig     *SSEConfig
		origin        string
		expectAllowed bool
	}{
		{
			name: "no config rejects all",
			sseConfig: &SSEConfig{
				AllowedOrigins: nil,
			},
			origin:        "https://example.com",
			expectAllowed: false,
		},
		{
			name: "empty list rejects all",
			sseConfig: &SSEConfig{
				AllowedOrigins: []string{},
			},
			origin:        "https://example.com",
			expectAllowed: false,
		},
		{
			name: "explicit origin allowed",
			sseConfig: &SSEConfig{
				AllowedOrigins: []string{"https://allowed.com"},
			},
			origin:        "https://allowed.com",
			expectAllowed: true,
		},
		{
			name: "origin not in list rejected",
			sseConfig: &SSEConfig{
				AllowedOrigins: []string{"https://allowed.com"},
			},
			origin:        "https://other.com",
			expectAllowed: false,
		},
		{
			name: "wildcard allows all",
			sseConfig: &SSEConfig{
				AllowedOrigins: []string{"*"},
			},
			origin:        "https://any-site.com",
			expectAllowed: true,
		},
		{
			name: "multiple origins",
			sseConfig: &SSEConfig{
				AllowedOrigins: []string{"https://a.com", "https://b.com"},
			},
			origin:        "https://b.com",
			expectAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewSSETransportWithConfig(config, tt.sseConfig)

			allowed := transport.isOriginAllowed(tt.origin)

			if allowed != tt.expectAllowed {
				t.Errorf("isOriginAllowed(%q) = %v, want %v",
					tt.origin, allowed, tt.expectAllowed)
			}
		})
	}
}

func TestSSE_DefaultConfigSecure(t *testing.T) {
	// Create transport with default config
	config := DefaultTransportConfig()
	transport := NewSSETransport(config)

	// Should reject cross-origin by default
	if transport.isOriginAllowed("https://evil.com") {
		t.Error("Default SSE config should reject cross-origin requests")
	}
}

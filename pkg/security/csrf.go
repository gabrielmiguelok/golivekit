// Package security provides security utilities for GoliveKit.
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Common security errors.
var (
	ErrInvalidToken     = errors.New("invalid CSRF token")
	ErrMissingToken     = errors.New("missing CSRF token")
	ErrTokenExpired     = errors.New("CSRF token expired")
	ErrInvalidSignature = errors.New("invalid token signature")
)

// CSRFProtection provides CSRF protection for forms.
type CSRFProtection struct {
	secret    []byte
	tokenLen  int
	maxAge    time.Duration
	sameSite  http.SameSite
	secure    bool
	domain    string
	cookieName string
	headerName string
	formField  string
	mu        sync.RWMutex
}

// CSRFConfig configures CSRF protection.
type CSRFConfig struct {
	// Secret key for signing tokens (required)
	Secret []byte

	// TokenLen is the length of random bytes in the token (default 32)
	TokenLen int

	// MaxAge is how long tokens are valid (default 24h)
	MaxAge time.Duration

	// SameSite cookie attribute (default Lax)
	SameSite http.SameSite

	// Secure cookie attribute (default true in production)
	Secure bool

	// Domain for the cookie (default empty = current domain)
	Domain string

	// CookieName for storing the token (default "_csrf")
	CookieName string

	// HeaderName for the token header (default "X-CSRF-Token")
	HeaderName string

	// FormField name for the token (default "_csrf")
	FormField string
}

// DefaultCSRFConfig returns default CSRF configuration.
func DefaultCSRFConfig() CSRFConfig {
	secret := make([]byte, 32)
	rand.Read(secret)

	return CSRFConfig{
		Secret:     secret,
		TokenLen:   32,
		MaxAge:     24 * time.Hour,
		SameSite:   http.SameSiteLaxMode,
		Secure:     true,
		CookieName: "_csrf",
		HeaderName: "X-CSRF-Token",
		FormField:  "_csrf",
	}
}

// NewCSRFProtection creates a new CSRF protection instance.
func NewCSRFProtection(config CSRFConfig) *CSRFProtection {
	if len(config.Secret) == 0 {
		config.Secret = make([]byte, 32)
		rand.Read(config.Secret)
	}
	if config.TokenLen == 0 {
		config.TokenLen = 32
	}
	if config.MaxAge == 0 {
		config.MaxAge = 24 * time.Hour
	}
	if config.CookieName == "" {
		config.CookieName = "_csrf"
	}
	if config.HeaderName == "" {
		config.HeaderName = "X-CSRF-Token"
	}
	if config.FormField == "" {
		config.FormField = "_csrf"
	}

	return &CSRFProtection{
		secret:     config.Secret,
		tokenLen:   config.TokenLen,
		maxAge:     config.MaxAge,
		sameSite:   config.SameSite,
		secure:     config.Secure,
		domain:     config.Domain,
		cookieName: config.CookieName,
		headerName: config.HeaderName,
		formField:  config.FormField,
	}
}

// GenerateToken creates a new CSRF token for the given session ID.
func (c *CSRFProtection) GenerateToken(sessionID string) (string, error) {
	// Generate random bytes
	randomBytes := make([]byte, c.tokenLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Create timestamp
	timestamp := time.Now().Unix()

	// Create token payload: random|timestamp|sessionID
	payload := fmt.Sprintf("%s|%d|%s",
		base64.StdEncoding.EncodeToString(randomBytes),
		timestamp,
		sessionID,
	)

	// Sign the payload
	signature := c.sign([]byte(payload))

	// Combine: payload.signature
	token := payload + "." + base64.StdEncoding.EncodeToString(signature)

	return token, nil
}

// ValidateToken validates a CSRF token against a session ID.
func (c *CSRFProtection) ValidateToken(token, sessionID string) error {
	if token == "" {
		return ErrMissingToken
	}

	// Split token into payload and signature
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return ErrInvalidToken
	}

	payload := parts[0]
	signatureB64 := parts[1]

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return ErrInvalidToken
	}

	// Verify signature
	expectedSig := c.sign([]byte(payload))
	if subtle.ConstantTimeCompare(signature, expectedSig) != 1 {
		return ErrInvalidSignature
	}

	// Parse payload
	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) != 3 {
		return ErrInvalidToken
	}

	// Check timestamp
	var timestamp int64
	_, err = fmt.Sscanf(payloadParts[1], "%d", &timestamp)
	if err != nil {
		return ErrInvalidToken
	}

	if time.Since(time.Unix(timestamp, 0)) > c.maxAge {
		return ErrTokenExpired
	}

	// Check session ID
	if payloadParts[2] != sessionID {
		return ErrInvalidToken
	}

	return nil
}

// sign creates an HMAC signature.
func (c *CSRFProtection) sign(data []byte) []byte {
	mac := hmac.New(sha256.New, c.secret)
	mac.Write(data)
	return mac.Sum(nil)
}

// Middleware returns HTTP middleware for CSRF protection.
func (c *CSRFProtection) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip safe methods
			if isSafeMethod(r.Method) {
				// For GET requests, ensure token exists in cookie
				c.ensureToken(w, r)
				next.ServeHTTP(w, r)
				return
			}

			// For unsafe methods, validate token
			token := c.getToken(r)
			sessionID := c.getSessionID(r)

			if err := c.ValidateToken(token, sessionID); err != nil {
				http.Error(w, "Forbidden - Invalid CSRF Token", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ensureToken ensures a CSRF token exists in the cookie.
func (c *CSRFProtection) ensureToken(w http.ResponseWriter, r *http.Request) {
	// Check if cookie exists
	if cookie, err := r.Cookie(c.cookieName); err == nil && cookie.Value != "" {
		return
	}

	// Generate new token
	sessionID := c.getSessionID(r)
	token, err := c.GenerateToken(sessionID)
	if err != nil {
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     c.cookieName,
		Value:    token,
		Path:     "/",
		Domain:   c.domain,
		MaxAge:   int(c.maxAge.Seconds()),
		Secure:   c.secure,
		HttpOnly: false, // Must be accessible to JavaScript
		SameSite: c.sameSite,
	})
}

// getToken extracts the CSRF token from the request.
func (c *CSRFProtection) getToken(r *http.Request) string {
	// Check header first
	if token := r.Header.Get(c.headerName); token != "" {
		return token
	}

	// Check form field
	if token := r.FormValue(c.formField); token != "" {
		return token
	}

	return ""
}

// getSessionID extracts the session ID from the request.
func (c *CSRFProtection) getSessionID(r *http.Request) string {
	// Try to get from session cookie
	if cookie, err := r.Cookie("session"); err == nil {
		return cookie.Value
	}

	// Fall back to remote addr + user agent
	return hashString(r.RemoteAddr + r.UserAgent())
}

// isSafeMethod returns true for safe HTTP methods.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// hashString creates a short hash of a string.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return base64.StdEncoding.EncodeToString(h[:8])
}

// GetToken is a helper to get the CSRF token from context for templates.
func GetToken(r *http.Request) string {
	if cookie, err := r.Cookie("_csrf"); err == nil {
		return cookie.Value
	}
	return ""
}

// TemplateFunc returns a template function for getting CSRF tokens.
func (c *CSRFProtection) TemplateFunc() func(*http.Request) string {
	return func(r *http.Request) string {
		if cookie, err := r.Cookie(c.cookieName); err == nil {
			return cookie.Value
		}
		return ""
	}
}

// Hidden returns an HTML hidden input with the CSRF token.
func (c *CSRFProtection) Hidden(r *http.Request) string {
	token := GetToken(r)
	return fmt.Sprintf(`<input type="hidden" name="%s" value="%s">`, c.formField, token)
}

package router

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Common errors.
var (
	ErrNilRenderer = errors.New("component returned nil renderer")
)

// RequestID middleware adds a unique request ID to the context.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = generateRequestID()
			}

			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			w.Header().Set("X-Request-ID", id)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type requestIDKey struct{}

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// generateRequestID generates a unique request ID.
func generateRequestID() string {
	// Simple implementation - in production use UUID
	return time.Now().Format("20060102150405.000000")
}

// Logger middleware logs requests.
func Logger(logger *log.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			logger.Printf(
				"%s %s %d %s %s",
				r.Method,
				r.URL.Path,
				rw.status,
				time.Since(start),
				r.RemoteAddr,
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// Recovery middleware recovers from panics.
func Recovery(onPanic func(any, *http.Request)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if onPanic != nil {
						onPanic(rec, r)
					}

					// Log stack trace
					debug.PrintStack()

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Timeout middleware adds a timeout to requests.
func Timeout(duration time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()

			done := make(chan struct{})
			go func() {
				next.ServeHTTP(w, r.WithContext(ctx))
				close(done)
			}()

			select {
			case <-done:
				// Request completed
			case <-ctx.Done():
				// Timeout
				http.Error(w, "Request Timeout", http.StatusRequestTimeout)
			}
		})
	}
}

// Compress middleware adds gzip compression.
func Compress() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if client accepts gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Wrap response writer
			gz := gzip.NewWriter(w)
			defer gz.Close()

			w.Header().Set("Content-Encoding", "gzip")
			gzw := &gzipResponseWriter{ResponseWriter: w, Writer: gz}

			next.ServeHTTP(gzw, r)
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (gzw *gzipResponseWriter) Write(b []byte) (int, error) {
	return gzw.Writer.Write(b)
}

// CORS middleware adds CORS headers.
func CORS(opts CORSOptions) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if opts.AllowAllOrigins || containsString(opts.AllowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			if len(opts.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(opts.AllowedMethods, ", "))
			}

			if len(opts.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(opts.AllowedHeaders, ", "))
			}

			if opts.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if opts.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", string(rune(opts.MaxAge)))
			}

			// Handle preflight
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSOptions configures CORS middleware.
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	AllowAllOrigins  bool
	MaxAge           int
}

// DefaultCORSOptions returns permissive CORS options for development.
func DefaultCORSOptions() CORSOptions {
	return CORSOptions{
		AllowAllOrigins:  true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           86400,
	}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// SecureHeadersConfig configures security headers.
type SecureHeadersConfig struct {
	// FrameOptions controls X-Frame-Options header.
	// Default: "DENY"
	FrameOptions string

	// ContentTypeNosniff enables X-Content-Type-Options: nosniff.
	// Default: true
	ContentTypeNosniff bool

	// XSSProtection enables X-XSS-Protection header.
	// Default: true
	XSSProtection bool

	// ReferrerPolicy sets the Referrer-Policy header.
	// Default: "strict-origin-when-cross-origin"
	ReferrerPolicy string

	// PermissionsPolicy sets the Permissions-Policy header.
	// Default: restricts geolocation, microphone, camera
	PermissionsPolicy string

	// HSTSEnabled enables Strict-Transport-Security header.
	// Only set when request is over HTTPS.
	// Default: true
	HSTSEnabled bool

	// HSTSMaxAge is the max-age for HSTS in seconds.
	// Default: 31536000 (1 year)
	HSTSMaxAge int

	// HSTSIncludeSubDomains includes subdomains in HSTS.
	// Default: true
	HSTSIncludeSubDomains bool

	// ContentSecurityPolicy sets the CSP header.
	// Default: secure policy with nonce support
	ContentSecurityPolicy string

	// CSPNonceEnabled enables CSP nonce for scripts/styles.
	// Default: true
	CSPNonceEnabled bool
}

// DefaultSecureHeadersConfig returns secure default configuration.
func DefaultSecureHeadersConfig() SecureHeadersConfig {
	return SecureHeadersConfig{
		FrameOptions:         "DENY",
		ContentTypeNosniff:   true,
		XSSProtection:        true,
		ReferrerPolicy:       "strict-origin-when-cross-origin",
		PermissionsPolicy:    "geolocation=(), microphone=(), camera=()",
		HSTSEnabled:          true,
		HSTSMaxAge:           31536000, // 1 year
		HSTSIncludeSubDomains: true,
		ContentSecurityPolicy: "", // Will be generated with nonce
		CSPNonceEnabled:      true,
	}
}

// cspNonceKey is the context key for CSP nonce.
type cspNonceKey struct{}

// GetCSPNonce retrieves the CSP nonce from context.
func GetCSPNonce(ctx context.Context) string {
	if nonce, ok := ctx.Value(cspNonceKey{}).(string); ok {
		return nonce
	}
	return ""
}

// generateNonce generates a random nonce for CSP.
func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// SecureHeaders middleware adds security headers.
// SECURITY: Updated with OWASP-recommended headers.
func SecureHeaders() Middleware {
	return SecureHeadersWithConfig(DefaultSecureHeadersConfig())
}

// SecureHeadersWithConfig creates middleware with custom config.
func SecureHeadersWithConfig(config SecureHeadersConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prevent clickjacking - use DENY instead of SAMEORIGIN for better security
			if config.FrameOptions != "" {
				w.Header().Set("X-Frame-Options", config.FrameOptions)
			}

			// Prevent MIME sniffing
			if config.ContentTypeNosniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			// XSS protection (legacy, but still useful for older browsers)
			if config.XSSProtection {
				w.Header().Set("X-XSS-Protection", "1; mode=block")
			}

			// Referrer policy
			if config.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}

			// Permissions policy (replaces Feature-Policy)
			if config.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", config.PermissionsPolicy)
			}

			// HSTS - only for HTTPS
			if config.HSTSEnabled && (r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https") {
				hstsValue := "max-age=" + strconv.Itoa(config.HSTSMaxAge)
				if config.HSTSIncludeSubDomains {
					hstsValue += "; includeSubDomains"
				}
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}

			// Content Security Policy with nonce
			ctx := r.Context()
			if config.CSPNonceEnabled {
				nonce := generateNonce()
				ctx = context.WithValue(ctx, cspNonceKey{}, nonce)

				csp := config.ContentSecurityPolicy
				if csp == "" {
					// SECURITY: Secure default CSP - no unsafe-inline or unsafe-eval
					csp = "default-src 'self'; " +
						"script-src 'self' 'nonce-" + nonce + "'; " +
						"style-src 'self' 'nonce-" + nonce + "'; " +
						"img-src 'self' data: https:; " +
						"connect-src 'self' wss:; " +
						"font-src 'self'; " +
						"frame-ancestors 'none'; " +
						"base-uri 'self'; " +
						"form-action 'self'"
				}
				w.Header().Set("Content-Security-Policy", csp)
			} else if config.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RateLimit middleware applies simple rate limiting.
// For production, use a proper rate limiter with distributed state.
func RateLimit(requestsPerSecond int) Middleware {
	// Simple token bucket per IP
	buckets := make(map[string]*tokenBucket)
	var mu sync.Mutex

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			mu.Lock()
			bucket, exists := buckets[ip]
			if !exists {
				bucket = newTokenBucket(requestsPerSecond)
				buckets[ip] = bucket
			}
			mu.Unlock()

			if !bucket.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote addr
	parts := strings.Split(r.RemoteAddr, ":")
	return parts[0]
}

type tokenBucket struct {
	tokens   int
	maxRate  int
	lastFill time.Time
	mu       sync.Mutex
}

func newTokenBucket(rate int) *tokenBucket {
	return &tokenBucket{
		tokens:   rate,
		maxRate:  rate,
		lastFill: time.Now(),
	}
}

func (tb *tokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(tb.lastFill)
	refill := int(elapsed.Seconds()) * tb.maxRate
	if refill > 0 {
		tb.tokens = min(tb.tokens+refill, tb.maxRate)
		tb.lastFill = now
	}

	if tb.tokens <= 0 {
		return false
	}

	tb.tokens--
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

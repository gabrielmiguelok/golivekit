package limits

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionLimiter limits concurrent connections per IP address.
type ConnectionLimiter struct {
	maxPerIP    int
	connections sync.Map // map[string]*atomic.Int32

	// Metrics
	totalBlocked atomic.Int64
	totalAllowed atomic.Int64
}

// NewConnectionLimiter creates a new connection limiter.
func NewConnectionLimiter(maxPerIP int) *ConnectionLimiter {
	if maxPerIP <= 0 {
		maxPerIP = 100 // Default max connections per IP
	}
	return &ConnectionLimiter{
		maxPerIP: maxPerIP,
	}
}

// Acquire attempts to acquire a connection slot for an IP.
// Returns true if successful, false if limit exceeded.
func (cl *ConnectionLimiter) Acquire(ip string) bool {
	counter, _ := cl.connections.LoadOrStore(ip, &atomic.Int32{})
	c := counter.(*atomic.Int32)

	for {
		cur := c.Load()
		if int(cur) >= cl.maxPerIP {
			cl.totalBlocked.Add(1)
			return false
		}
		if c.CompareAndSwap(cur, cur+1) {
			cl.totalAllowed.Add(1)
			return true
		}
	}
}

// Release releases a connection slot for an IP.
func (cl *ConnectionLimiter) Release(ip string) {
	if counter, ok := cl.connections.Load(ip); ok {
		c := counter.(*atomic.Int32)
		c.Add(-1)

		// Clean up if count reaches 0
		if c.Load() <= 0 {
			cl.connections.Delete(ip)
		}
	}
}

// Count returns the current connection count for an IP.
func (cl *ConnectionLimiter) Count(ip string) int {
	if counter, ok := cl.connections.Load(ip); ok {
		return int(counter.(*atomic.Int32).Load())
	}
	return 0
}

// TotalBlocked returns the total number of blocked connections.
func (cl *ConnectionLimiter) TotalBlocked() int64 {
	return cl.totalBlocked.Load()
}

// TotalAllowed returns the total number of allowed connections.
func (cl *ConnectionLimiter) TotalAllowed() int64 {
	return cl.totalAllowed.Load()
}

// SetMaxPerIP updates the maximum connections per IP.
func (cl *ConnectionLimiter) SetMaxPerIP(max int) {
	cl.maxPerIP = max
}

// Middleware returns HTTP middleware that limits connections per IP.
func (cl *ConnectionLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetClientIP(r)

			if !cl.Acquire(ip) {
				http.Error(w, "Too Many Connections", http.StatusTooManyRequests)
				return
			}
			defer cl.Release(ip)

			next.ServeHTTP(w, r)
		})
	}
}

// GetClientIP extracts the client IP from an HTTP request.
// Checks X-Forwarded-For and X-Real-IP headers, falling back to RemoteAddr.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For (first IP in the list)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// GlobalConnectionLimiter limits total concurrent connections across all IPs.
type GlobalConnectionLimiter struct {
	maxConnections int32
	current        atomic.Int32
}

// NewGlobalConnectionLimiter creates a global connection limiter.
func NewGlobalConnectionLimiter(max int) *GlobalConnectionLimiter {
	return &GlobalConnectionLimiter{
		maxConnections: int32(max),
	}
}

// Acquire attempts to acquire a connection slot.
func (gl *GlobalConnectionLimiter) Acquire() bool {
	for {
		cur := gl.current.Load()
		if cur >= gl.maxConnections {
			return false
		}
		if gl.current.CompareAndSwap(cur, cur+1) {
			return true
		}
	}
}

// Release releases a connection slot.
func (gl *GlobalConnectionLimiter) Release() {
	gl.current.Add(-1)
}

// Count returns the current connection count.
func (gl *GlobalConnectionLimiter) Count() int {
	return int(gl.current.Load())
}

// Available returns the number of available slots.
func (gl *GlobalConnectionLimiter) Available() int {
	return int(gl.maxConnections - gl.current.Load())
}

// Middleware returns HTTP middleware for global connection limiting.
func (gl *GlobalConnectionLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !gl.Acquire() {
				http.Error(w, "Service Busy", http.StatusServiceUnavailable)
				return
			}
			defer gl.Release()

			next.ServeHTTP(w, r)
		})
	}
}

// CompositeConnectionLimiter combines per-IP and global limits.
type CompositeConnectionLimiter struct {
	perIP  *ConnectionLimiter
	global *GlobalConnectionLimiter
}

// NewCompositeConnectionLimiter creates a composite limiter.
func NewCompositeConnectionLimiter(maxPerIP, maxGlobal int) *CompositeConnectionLimiter {
	return &CompositeConnectionLimiter{
		perIP:  NewConnectionLimiter(maxPerIP),
		global: NewGlobalConnectionLimiter(maxGlobal),
	}
}

// Acquire attempts to acquire both per-IP and global slots.
func (cl *CompositeConnectionLimiter) Acquire(ip string) bool {
	// Try global first
	if !cl.global.Acquire() {
		return false
	}

	// Then try per-IP
	if !cl.perIP.Acquire(ip) {
		cl.global.Release()
		return false
	}

	return true
}

// Release releases both slots.
func (cl *CompositeConnectionLimiter) Release(ip string) {
	cl.perIP.Release(ip)
	cl.global.Release()
}

// Middleware returns HTTP middleware.
func (cl *CompositeConnectionLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetClientIP(r)

			if !cl.Acquire(ip) {
				http.Error(w, "Too Many Connections", http.StatusTooManyRequests)
				return
			}
			defer cl.Release(ip)

			next.ServeHTTP(w, r)
		})
	}
}

// ConnectionTracker tracks connection metrics.
type ConnectionTracker struct {
	perIP           *ConnectionLimiter
	global          *GlobalConnectionLimiter
	connectionTimes sync.Map // map[string]time.Time - when connection was established
}

// NewConnectionTracker creates a new connection tracker.
func NewConnectionTracker(maxPerIP, maxGlobal int) *ConnectionTracker {
	return &ConnectionTracker{
		perIP:  NewConnectionLimiter(maxPerIP),
		global: NewGlobalConnectionLimiter(maxGlobal),
	}
}

// Track starts tracking a connection.
func (ct *ConnectionTracker) Track(id, ip string) bool {
	if !ct.perIP.Acquire(ip) {
		return false
	}
	if !ct.global.Acquire() {
		ct.perIP.Release(ip)
		return false
	}

	ct.connectionTimes.Store(id, time.Now())
	return true
}

// Untrack stops tracking a connection.
func (ct *ConnectionTracker) Untrack(id, ip string) {
	ct.connectionTimes.Delete(id)
	ct.perIP.Release(ip)
	ct.global.Release()
}

// Duration returns how long a connection has been active.
func (ct *ConnectionTracker) Duration(id string) time.Duration {
	if v, ok := ct.connectionTimes.Load(id); ok {
		return time.Since(v.(time.Time))
	}
	return 0
}

// Stats returns connection statistics.
func (ct *ConnectionTracker) Stats() map[string]interface{} {
	return map[string]interface{}{
		"current_connections": ct.global.Count(),
		"available_slots":     ct.global.Available(),
		"total_blocked":       ct.perIP.TotalBlocked(),
		"total_allowed":       ct.perIP.TotalAllowed(),
	}
}

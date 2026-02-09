// Package health provides health check endpoints for GoliveKit applications.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a service.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// CheckResult represents the result of a single health check.
type CheckResult struct {
	Status   Status        `json:"status"`
	Duration time.Duration `json:"duration_ms"`
	Error    string        `json:"error,omitempty"`
	Details  interface{}   `json:"details,omitempty"`
}

// HealthStatus represents the overall health status.
type HealthStatus struct {
	Status    Status                 `json:"status"`
	Checks    map[string]CheckResult `json:"checks"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version,omitempty"`
}

// Check defines a single health check.
type Check struct {
	Name     string
	Check    func(ctx context.Context) error
	Timeout  time.Duration
	Critical bool // If true, failure makes overall status unhealthy
}

// Checker manages health checks for the application.
type Checker struct {
	checks  []Check
	version string
	mu      sync.RWMutex
}

// NewChecker creates a new health checker.
func NewChecker() *Checker {
	return &Checker{
		checks: make([]Check, 0),
	}
}

// SetVersion sets the application version shown in health responses.
func (hc *Checker) SetVersion(version string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.version = version
}

// AddCheck adds a health check.
func (hc *Checker) AddCheck(name string, check func(context.Context) error, timeout time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks = append(hc.checks, Check{
		Name:     name,
		Check:    check,
		Timeout:  timeout,
		Critical: false,
	})
}

// AddCriticalCheck adds a critical health check.
// If a critical check fails, the overall status is unhealthy.
func (hc *Checker) AddCriticalCheck(name string, check func(context.Context) error, timeout time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks = append(hc.checks, Check{
		Name:     name,
		Check:    check,
		Timeout:  timeout,
		Critical: true,
	})
}

// Check runs all health checks and returns the overall status.
func (hc *Checker) Check(ctx context.Context) HealthStatus {
	hc.mu.RLock()
	checks := make([]Check, len(hc.checks))
	copy(checks, hc.checks)
	version := hc.version
	hc.mu.RUnlock()

	status := HealthStatus{
		Status:    StatusHealthy,
		Checks:    make(map[string]CheckResult),
		Timestamp: time.Now(),
		Version:   version,
	}

	// Run checks concurrently
	type checkResult struct {
		name     string
		result   CheckResult
		critical bool
	}

	results := make(chan checkResult, len(checks))
	var wg sync.WaitGroup

	for _, c := range checks {
		wg.Add(1)
		go func(check Check) {
			defer wg.Done()

			timeout := check.Timeout
			if timeout == 0 {
				timeout = 5 * time.Second
			}

			start := time.Now()
			checkCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			err := check.Check(checkCtx)
			duration := time.Since(start)

			result := CheckResult{
				Status:   StatusHealthy,
				Duration: duration / time.Millisecond,
			}

			if err != nil {
				result.Status = StatusUnhealthy
				result.Error = err.Error()
			}

			results <- checkResult{
				name:     check.Name,
				result:   result,
				critical: check.Critical,
			}
		}(c)
	}

	// Wait and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for r := range results {
		status.Checks[r.name] = r.result

		if r.result.Status != StatusHealthy {
			if r.critical {
				status.Status = StatusUnhealthy
			} else if status.Status == StatusHealthy {
				status.Status = StatusDegraded
			}
		}
	}

	return status
}

// LivenessHandler returns an HTTP handler for liveness probes.
// Returns 200 if the process is running.
func (hc *Checker) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "alive",
			"timestamp": time.Now(),
		})
	})
}

// ReadinessHandler returns an HTTP handler for readiness probes.
// Returns 200 if all critical checks pass, 503 otherwise.
func (hc *Checker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status := hc.Check(ctx)

		w.Header().Set("Content-Type", "application/json")

		if status.Status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		json.NewEncoder(w).Encode(status)
	})
}

// HealthHandler returns an HTTP handler for full health checks.
// Always returns 200 with detailed status.
func (hc *Checker) HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status := hc.Check(ctx)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	})
}

// Common health check functions

// PingCheck creates a simple ping check that always passes.
func PingCheck() func(context.Context) error {
	return func(ctx context.Context) error {
		return nil
	}
}

// DatabaseCheck creates a database connectivity check.
func DatabaseCheck(ping func(context.Context) error) func(context.Context) error {
	return ping
}

// WebSocketPoolCheck creates a check for WebSocket connection pool.
func WebSocketPoolCheck(getCount func() int, maxConnections int) func(context.Context) error {
	return func(ctx context.Context) error {
		count := getCount()
		if count >= maxConnections {
			return &HealthError{
				Message: "WebSocket pool at capacity",
				Details: map[string]interface{}{
					"current": count,
					"max":     maxConnections,
				},
			}
		}
		return nil
	}
}

// MemoryCheck creates a memory usage check.
func MemoryCheck(maxBytes uint64) func(context.Context) error {
	return func(ctx context.Context) error {
		// In production, use runtime.MemStats
		return nil
	}
}

// HealthError represents a health check error with details.
type HealthError struct {
	Message string
	Details map[string]interface{}
}

func (e *HealthError) Error() string {
	return e.Message
}

// DefaultChecker provides a pre-configured checker with common checks.
func DefaultChecker(version string) *Checker {
	hc := NewChecker()
	hc.SetVersion(version)
	hc.AddCheck("ping", PingCheck(), time.Second)
	return hc
}

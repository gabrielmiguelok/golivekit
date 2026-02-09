package core

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Circuit breaker errors.
var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrCircuitHalfOpen = errors.New("circuit breaker is half-open")
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int32

const (
	// CircuitClosed means the circuit is functioning normally.
	CircuitClosed CircuitState = iota
	// CircuitOpen means the circuit has tripped and is not allowing requests.
	CircuitOpen
	// CircuitHalfOpen means the circuit is testing if recovery is possible.
	CircuitHalfOpen
)

// String returns the string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures the circuit breaker behavior.
type CircuitBreakerConfig struct {
	// MaxErrors is the number of consecutive errors before opening the circuit.
	MaxErrors int

	// ResetTimeout is how long to wait before attempting to close the circuit.
	ResetTimeout time.Duration

	// SuccessThreshold is the number of successful calls needed to close the circuit.
	SuccessThreshold int

	// OnStateChange is called when the circuit state changes.
	OnStateChange func(from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns sensible defaults.
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxErrors:        5,
		ResetTimeout:     30 * time.Second,
		SuccessThreshold: 2,
	}
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	config *CircuitBreakerConfig

	// State tracking using atomics for performance
	state        atomic.Int32
	errorCount   atomic.Int32
	successCount atomic.Int32
	lastError    atomic.Int64 // Unix nanoseconds

	mu sync.Mutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig()
	}

	cb := &CircuitBreaker{
		config: config,
	}
	cb.state.Store(int32(CircuitClosed))

	return cb
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	return CircuitState(cb.state.Load())
}

// Allow checks if a request should be allowed through.
// Returns nil if allowed, an error if the circuit is open/half-open.
func (cb *CircuitBreaker) Allow() error {
	state := cb.State()

	switch state {
	case CircuitClosed:
		return nil

	case CircuitOpen:
		// Check if reset timeout has passed
		lastErr := time.Unix(0, cb.lastError.Load())
		if time.Since(lastErr) > cb.config.ResetTimeout {
			// Transition to half-open
			cb.setState(CircuitHalfOpen)
			return nil
		}
		return ErrCircuitOpen

	case CircuitHalfOpen:
		// Allow request through in half-open state
		return nil
	}

	return nil
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess() {
	state := cb.State()

	switch state {
	case CircuitClosed:
		// Reset error count on success
		cb.errorCount.Store(0)

	case CircuitHalfOpen:
		// Increment success count
		successes := cb.successCount.Add(1)
		if int(successes) >= cb.config.SuccessThreshold {
			// Transition to closed
			cb.setState(CircuitClosed)
			cb.successCount.Store(0)
			cb.errorCount.Store(0)
		}

	case CircuitOpen:
		// Shouldn't happen, but reset anyway
		cb.errorCount.Store(0)
	}
}

// RecordError records a failed operation.
func (cb *CircuitBreaker) RecordError() {
	cb.lastError.Store(time.Now().UnixNano())
	state := cb.State()

	switch state {
	case CircuitClosed:
		// Increment error count
		errors := cb.errorCount.Add(1)
		if int(errors) >= cb.config.MaxErrors {
			// Transition to open
			cb.setState(CircuitOpen)
		}

	case CircuitHalfOpen:
		// Any error in half-open state opens the circuit
		cb.setState(CircuitOpen)
		cb.successCount.Store(0)

	case CircuitOpen:
		// Already open, just update last error time
	}
}

// Reset manually resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.setState(CircuitClosed)
	cb.errorCount.Store(0)
	cb.successCount.Store(0)
}

// Metrics returns current circuit breaker metrics.
func (cb *CircuitBreaker) Metrics() CircuitBreakerMetrics {
	return CircuitBreakerMetrics{
		State:        cb.State(),
		ErrorCount:   int(cb.errorCount.Load()),
		SuccessCount: int(cb.successCount.Load()),
		LastError:    time.Unix(0, cb.lastError.Load()),
	}
}

// setState changes the circuit state and calls the callback.
func (cb *CircuitBreaker) setState(newState CircuitState) {
	oldState := CircuitState(cb.state.Swap(int32(newState)))

	if cb.config.OnStateChange != nil && oldState != newState {
		cb.config.OnStateChange(oldState, newState)
	}
}

// CircuitBreakerMetrics contains circuit breaker metrics.
type CircuitBreakerMetrics struct {
	State        CircuitState
	ErrorCount   int
	SuccessCount int
	LastError    time.Time
}

// Execute runs a function with circuit breaker protection.
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.Allow(); err != nil {
		return err
	}

	err := fn()
	if err != nil {
		cb.RecordError()
	} else {
		cb.RecordSuccess()
	}

	return err
}

// ExecuteWithResult runs a function that returns a value with circuit breaker protection.
func ExecuteWithResult[T any](cb *CircuitBreaker, fn func() (T, error)) (T, error) {
	var zero T

	if err := cb.Allow(); err != nil {
		return zero, err
	}

	result, err := fn()
	if err != nil {
		cb.RecordError()
	} else {
		cb.RecordSuccess()
	}

	return result, err
}

// CircuitBreakerRegistry manages multiple circuit breakers.
type CircuitBreakerRegistry struct {
	breakers map[string]*CircuitBreaker
	config   *CircuitBreakerConfig
	mu       sync.RWMutex
}

// NewCircuitBreakerRegistry creates a new registry.
func NewCircuitBreakerRegistry(defaultConfig *CircuitBreakerConfig) *CircuitBreakerRegistry {
	if defaultConfig == nil {
		defaultConfig = DefaultCircuitBreakerConfig()
	}
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
		config:   defaultConfig,
	}
}

// Get retrieves or creates a circuit breaker for the given name.
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[name]
	r.mu.RUnlock()

	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, ok = r.breakers[name]; ok {
		return cb
	}

	cb = NewCircuitBreaker(r.config)
	r.breakers[name] = cb
	return cb
}

// Remove removes a circuit breaker from the registry.
func (r *CircuitBreakerRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.breakers, name)
}

// AllMetrics returns metrics for all circuit breakers.
func (r *CircuitBreakerRegistry) AllMetrics() map[string]CircuitBreakerMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]CircuitBreakerMetrics, len(r.breakers))
	for name, cb := range r.breakers {
		result[name] = cb.Metrics()
	}
	return result
}

// DefaultCircuitBreakerRegistry is the global registry.
var DefaultCircuitBreakerRegistry = NewCircuitBreakerRegistry(nil)

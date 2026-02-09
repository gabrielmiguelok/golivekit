// Package retry provides retry logic with exponential backoff.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Common retry errors.
var (
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	ErrContextCanceled    = errors.New("context canceled during retry")
)

// Config configures retry behavior.
type Config struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int

	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration

	// Multiplier is the factor by which the delay increases.
	Multiplier float64

	// Jitter is the randomization factor (0-1) to prevent thundering herd.
	Jitter float64

	// RetryIf determines if an error should be retried.
	// If nil, all errors are retried.
	RetryIf func(error) bool

	// OnRetry is called before each retry attempt.
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxRetries:   5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
}

// Retry executes a function with retry logic.
func Retry(ctx context.Context, config *Config, fn func() error) error {
	if config == nil {
		config = DefaultConfig()
	}

	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context before attempt
		select {
		case <-ctx.Done():
			return ErrContextCanceled
		default:
		}

		// Execute function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error should be retried
		if config.RetryIf != nil && !config.RetryIf(err) {
			return err
		}

		// Don't delay after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := Backoff(attempt, config)

		// Callback before retry
		if config.OnRetry != nil {
			config.OnRetry(attempt+1, err, delay)
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ErrContextCanceled
		case <-time.After(delay):
		}
	}

	return errors.Join(ErrMaxRetriesExceeded, lastErr)
}

// RetryWithResult executes a function that returns a value with retry logic.
func RetryWithResult[T any](ctx context.Context, config *Config, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	if config == nil {
		config = DefaultConfig()
	}

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context before attempt
		select {
		case <-ctx.Done():
			return result, ErrContextCanceled
		default:
		}

		// Execute function
		res, err := fn()
		if err == nil {
			return res, nil
		}

		lastErr = err

		// Check if error should be retried
		if config.RetryIf != nil && !config.RetryIf(err) {
			return result, err
		}

		// Don't delay after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Calculate delay with exponential backoff
		delay := Backoff(attempt, config)

		// Callback before retry
		if config.OnRetry != nil {
			config.OnRetry(attempt+1, err, delay)
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return result, ErrContextCanceled
		case <-time.After(delay):
		}
	}

	return result, errors.Join(ErrMaxRetriesExceeded, lastErr)
}

// Backoff calculates the delay for a given attempt.
func Backoff(attempt int, config *Config) time.Duration {
	if config == nil {
		config = DefaultConfig()
	}

	// Calculate base delay with exponential backoff
	delay := float64(config.InitialDelay) * math.Pow(config.Multiplier, float64(attempt))

	// Apply max delay cap
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}

	// Apply jitter
	if config.Jitter > 0 {
		jitter := delay * config.Jitter
		delay = delay - jitter + (rand.Float64() * 2 * jitter)
	}

	return time.Duration(delay)
}

// IsRetryable wraps an error to mark it as retryable.
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError creates a retryable error wrapper.
func NewRetryableError(err error) *RetryableError {
	return &RetryableError{Err: err}
}

// IsRetryableError checks if an error is marked as retryable.
func IsRetryableError(err error) bool {
	var retryable *RetryableError
	return errors.As(err, &retryable)
}

// PermanentError wraps an error to mark it as permanent (non-retryable).
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

// NewPermanentError creates a permanent error wrapper.
func NewPermanentError(err error) *PermanentError {
	return &PermanentError{Err: err}
}

// IsPermanentError checks if an error is marked as permanent.
func IsPermanentError(err error) bool {
	var permanent *PermanentError
	return errors.As(err, &permanent)
}

// RetryOnlyRetryable returns a RetryIf function that only retries RetryableErrors.
func RetryOnlyRetryable() func(error) bool {
	return func(err error) bool {
		return IsRetryableError(err)
	}
}

// RetryUnlessPermanent returns a RetryIf function that retries unless PermanentError.
func RetryUnlessPermanent() func(error) bool {
	return func(err error) bool {
		return !IsPermanentError(err)
	}
}

// Do is a convenience function for simple retries.
func Do(fn func() error, attempts int) error {
	config := &Config{
		MaxRetries:   attempts - 1,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
	return Retry(context.Background(), config, fn)
}

// DoWithBackoff is a convenience function for retries with custom backoff.
func DoWithBackoff(fn func() error, attempts int, initial, max time.Duration) error {
	config := &Config{
		MaxRetries:   attempts - 1,
		InitialDelay: initial,
		MaxDelay:     max,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
	return Retry(context.Background(), config, fn)
}

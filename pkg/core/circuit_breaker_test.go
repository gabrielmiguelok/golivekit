package core

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreaker_Initial(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	if cb.State() != CircuitClosed {
		t.Errorf("expected initial state Closed, got %v", cb.State())
	}

	if err := cb.Allow(); err != nil {
		t.Errorf("expected Allow() to succeed, got %v", err)
	}
}

func TestCircuitBreaker_OpenAfterErrors(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxErrors:        3,
		ResetTimeout:     1 * time.Second,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)

	// Record errors until circuit opens
	for i := 0; i < 3; i++ {
		cb.RecordError()
	}

	if cb.State() != CircuitOpen {
		t.Errorf("expected state Open after 3 errors, got %v", cb.State())
	}

	err := cb.Allow()
	if err != ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_ResetToHalfOpen(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxErrors:        2,
		ResetTimeout:     50 * time.Millisecond,
		SuccessThreshold: 1,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.RecordError()
	cb.RecordError()

	if cb.State() != CircuitOpen {
		t.Fatalf("expected circuit to be open")
	}

	// Wait for reset timeout
	time.Sleep(100 * time.Millisecond)

	// Allow should transition to half-open
	if err := cb.Allow(); err != nil {
		t.Errorf("expected Allow() to succeed after timeout, got %v", err)
	}

	if cb.State() != CircuitHalfOpen {
		t.Errorf("expected state HalfOpen, got %v", cb.State())
	}
}

func TestCircuitBreaker_CloseFromHalfOpen(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxErrors:        2,
		ResetTimeout:     50 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.RecordError()
	cb.RecordError()

	// Wait and transition to half-open
	time.Sleep(100 * time.Millisecond)
	cb.Allow()

	// Record successes
	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != CircuitClosed {
		t.Errorf("expected state Closed after successes, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpenFromHalfOpen(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxErrors:        2,
		ResetTimeout:     50 * time.Millisecond,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.RecordError()
	cb.RecordError()

	// Wait and transition to half-open
	time.Sleep(100 * time.Millisecond)
	cb.Allow()

	if cb.State() != CircuitHalfOpen {
		t.Fatalf("expected state HalfOpen")
	}

	// Error in half-open state should open circuit again
	cb.RecordError()

	if cb.State() != CircuitOpen {
		t.Errorf("expected state Open after error in half-open, got %v", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := &CircuitBreakerConfig{
		MaxErrors:        2,
		ResetTimeout:     1 * time.Second,
		SuccessThreshold: 2,
	}
	cb := NewCircuitBreaker(config)

	// Open the circuit
	cb.RecordError()
	cb.RecordError()

	if cb.State() != CircuitOpen {
		t.Fatalf("expected circuit to be open")
	}

	// Manual reset
	cb.Reset()

	if cb.State() != CircuitClosed {
		t.Errorf("expected state Closed after reset, got %v", cb.State())
	}

	if err := cb.Allow(); err != nil {
		t.Errorf("expected Allow() to succeed after reset, got %v", err)
	}
}

func TestCircuitBreaker_Execute(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxErrors:        2,
		ResetTimeout:     1 * time.Second,
		SuccessThreshold: 1,
	})

	// Success
	err := cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Failures
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		err = cb.Execute(func() error {
			return testErr
		})
		if err != testErr {
			t.Errorf("expected test error, got %v", err)
		}
	}

	// Circuit should be open
	err = cb.Execute(func() error {
		return nil
	})
	if err != ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_ExecuteWithResult(t *testing.T) {
	cb := NewCircuitBreaker(nil)

	// Success
	result, err := ExecuteWithResult(cb, func() (int, error) {
		return 42, nil
	})
	if err != nil || result != 42 {
		t.Errorf("expected (42, nil), got (%d, %v)", result, err)
	}
}

func TestCircuitBreaker_OnStateChange(t *testing.T) {
	var changes []struct {
		from, to CircuitState
	}

	config := &CircuitBreakerConfig{
		MaxErrors:        2,
		ResetTimeout:     50 * time.Millisecond,
		SuccessThreshold: 1,
		OnStateChange: func(from, to CircuitState) {
			changes = append(changes, struct{ from, to CircuitState }{from, to})
		},
	}
	cb := NewCircuitBreaker(config)

	// Closed -> Open
	cb.RecordError()
	cb.RecordError()

	// Open -> HalfOpen
	time.Sleep(100 * time.Millisecond)
	cb.Allow()

	// HalfOpen -> Closed
	cb.RecordSuccess()

	if len(changes) != 3 {
		t.Errorf("expected 3 state changes, got %d", len(changes))
	}

	expected := []struct {
		from, to CircuitState
	}{
		{CircuitClosed, CircuitOpen},
		{CircuitOpen, CircuitHalfOpen},
		{CircuitHalfOpen, CircuitClosed},
	}

	for i, change := range changes {
		if change.from != expected[i].from || change.to != expected[i].to {
			t.Errorf("change %d: expected %v->%v, got %v->%v",
				i, expected[i].from, expected[i].to, change.from, change.to)
		}
	}
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxErrors:        100,
		ResetTimeout:     1 * time.Second,
		SuccessThreshold: 10,
	})

	var wg sync.WaitGroup
	var ops atomic.Int64

	// Concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cb.Allow()
				if j%2 == 0 {
					cb.RecordSuccess()
				} else {
					cb.RecordError()
				}
				ops.Add(1)
			}
		}()
	}

	wg.Wait()

	if ops.Load() != 1000 {
		t.Errorf("expected 1000 operations, got %d", ops.Load())
	}
}

func TestCircuitBreaker_Metrics(t *testing.T) {
	cb := NewCircuitBreaker(&CircuitBreakerConfig{
		MaxErrors:        5,
		ResetTimeout:     1 * time.Second,
		SuccessThreshold: 2,
	})

	cb.RecordError()
	cb.RecordError()
	cb.RecordSuccess()

	metrics := cb.Metrics()

	if metrics.State != CircuitClosed {
		t.Errorf("expected state Closed, got %v", metrics.State)
	}

	// After a success, error count should reset
	if metrics.ErrorCount != 0 {
		t.Errorf("expected error count 0, got %d", metrics.ErrorCount)
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	registry := NewCircuitBreakerRegistry(nil)

	cb1 := registry.Get("service-a")
	cb2 := registry.Get("service-b")
	cb1Again := registry.Get("service-a")

	if cb1 != cb1Again {
		t.Error("expected same circuit breaker for same name")
	}

	if cb1 == cb2 {
		t.Error("expected different circuit breakers for different names")
	}

	// Record errors on one circuit
	for i := 0; i < 5; i++ {
		cb1.RecordError()
	}

	// Check metrics
	metrics := registry.AllMetrics()
	if len(metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(metrics))
	}

	if metrics["service-a"].State != CircuitOpen {
		t.Errorf("expected service-a to be open")
	}

	if metrics["service-b"].State != CircuitClosed {
		t.Errorf("expected service-b to be closed")
	}

	// Remove
	registry.Remove("service-a")
	metrics = registry.AllMetrics()
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric after remove, got %d", len(metrics))
	}
}

func BenchmarkCircuitBreaker_Allow_Closed(b *testing.B) {
	cb := NewCircuitBreaker(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Allow()
	}
}

func BenchmarkCircuitBreaker_Allow_Parallel(b *testing.B) {
	cb := NewCircuitBreaker(nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Allow()
		}
	})
}

func BenchmarkCircuitBreaker_RecordSuccess(b *testing.B) {
	cb := NewCircuitBreaker(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.RecordSuccess()
	}
}

func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	cb := NewCircuitBreaker(nil)
	fn := func() error { return nil }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(fn)
	}
}

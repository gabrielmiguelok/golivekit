// Package testing provides testing utilities for GoliveKit.
// This file adds chaos testing helpers for resilience testing.
package testing

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// ChaosConfig configures chaos testing behavior.
type ChaosConfig struct {
	// Latency adds artificial latency to operations.
	Latency time.Duration

	// LatencyJitter adds random jitter to latency (0-1).
	LatencyJitter float64

	// DropRate is the probability of dropping a message (0-1).
	DropRate float64

	// ErrorRate is the probability of returning an error (0-1).
	ErrorRate float64

	// Enabled controls whether chaos is active.
	Enabled bool

	// Seed for random number generation (0 = time-based).
	Seed int64
}

// DefaultChaosConfig returns a config suitable for testing.
func DefaultChaosConfig() ChaosConfig {
	return ChaosConfig{
		Latency:       50 * time.Millisecond,
		LatencyJitter: 0.5,
		DropRate:      0.01,
		ErrorRate:     0.01,
		Enabled:       true,
	}
}

// MildChaosConfig returns a less aggressive chaos config.
func MildChaosConfig() ChaosConfig {
	return ChaosConfig{
		Latency:       10 * time.Millisecond,
		LatencyJitter: 0.2,
		DropRate:      0.001,
		ErrorRate:     0.001,
		Enabled:       true,
	}
}

// SevereChaosConfig returns an aggressive chaos config.
func SevereChaosConfig() ChaosConfig {
	return ChaosConfig{
		Latency:       200 * time.Millisecond,
		LatencyJitter: 0.8,
		DropRate:      0.1,
		ErrorRate:     0.1,
		Enabled:       true,
	}
}

// ChaosTransport wraps a transport with chaos injection.
type ChaosTransport struct {
	wrapped core.Transport
	config  ChaosConfig
	rng     *rand.Rand
	mu      sync.Mutex
}

// Common chaos errors.
var (
	ErrChaosInjected  = errors.New("chaos: simulated error")
	ErrChaosDropped   = errors.New("chaos: message dropped")
	ErrChaosTimeout   = errors.New("chaos: simulated timeout")
)

// NewChaosTransport creates a chaos-injecting transport wrapper.
func NewChaosTransport(wrapped core.Transport, config ChaosConfig) *ChaosTransport {
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &ChaosTransport{
		wrapped: wrapped,
		config:  config,
		rng:     rand.New(rand.NewSource(seed)),
	}
}

// Send sends a message with chaos injection.
func (ct *ChaosTransport) Send(msg core.Message) error {
	if !ct.config.Enabled {
		return ct.wrapped.Send(msg)
	}

	ct.mu.Lock()
	dropRate := ct.config.DropRate
	errorRate := ct.config.ErrorRate
	r1, r2 := ct.rng.Float64(), ct.rng.Float64()
	ct.mu.Unlock()

	// Inject latency
	ct.injectLatency()

	// Possibly drop the message
	if r1 < dropRate {
		return nil // Silent drop
	}

	// Possibly return an error
	if r2 < errorRate {
		return ErrChaosInjected
	}

	return ct.wrapped.Send(msg)
}

// Close closes the wrapped transport.
func (ct *ChaosTransport) Close() error {
	return ct.wrapped.Close()
}

// IsConnected checks if the wrapped transport is connected.
func (ct *ChaosTransport) IsConnected() bool {
	return ct.wrapped.IsConnected()
}

// injectLatency adds artificial latency.
func (ct *ChaosTransport) injectLatency() {
	if ct.config.Latency <= 0 {
		return
	}

	ct.mu.Lock()
	jitter := ct.rng.Float64() * ct.config.LatencyJitter
	ct.mu.Unlock()

	baseLatency := float64(ct.config.Latency)
	actualLatency := baseLatency * (1 + jitter - ct.config.LatencyJitter/2)

	time.Sleep(time.Duration(actualLatency))
}

// SetConfig updates the chaos configuration.
func (ct *ChaosTransport) SetConfig(config ChaosConfig) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.config = config
}

// ChaosMiddleware is HTTP middleware that injects chaos.
type ChaosMiddleware struct {
	config ChaosConfig
	rng    *rand.Rand
	mu     sync.Mutex
}

// NewChaosMiddleware creates chaos-injecting HTTP middleware.
func NewChaosMiddleware(config ChaosConfig) *ChaosMiddleware {
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &ChaosMiddleware{
		config: config,
		rng:    rand.New(rand.NewSource(seed)),
	}
}

// FaultInjector provides programmatic fault injection.
type FaultInjector struct {
	faults map[string]*Fault
	mu     sync.RWMutex
}

// Fault represents an injectable fault.
type Fault struct {
	Name        string
	Probability float64
	Error       error
	Latency     time.Duration
	Active      bool
}

// NewFaultInjector creates a new fault injector.
func NewFaultInjector() *FaultInjector {
	return &FaultInjector{
		faults: make(map[string]*Fault),
	}
}

// Register registers a fault.
func (fi *FaultInjector) Register(name string, fault *Fault) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fault.Name = name
	fi.faults[name] = fault
}

// Activate activates a fault.
func (fi *FaultInjector) Activate(name string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	if f, ok := fi.faults[name]; ok {
		f.Active = true
	}
}

// Deactivate deactivates a fault.
func (fi *FaultInjector) Deactivate(name string) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	if f, ok := fi.faults[name]; ok {
		f.Active = false
	}
}

// DeactivateAll deactivates all faults.
func (fi *FaultInjector) DeactivateAll() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	for _, f := range fi.faults {
		f.Active = false
	}
}

// Check checks if a fault should be triggered.
func (fi *FaultInjector) Check(name string) error {
	fi.mu.RLock()
	fault, ok := fi.faults[name]
	fi.mu.RUnlock()

	if !ok || !fault.Active {
		return nil
	}

	if fault.Latency > 0 {
		time.Sleep(fault.Latency)
	}

	if fault.Probability > 0 && rand.Float64() < fault.Probability {
		return fault.Error
	}

	return nil
}

// ContextChaos provides context-based chaos injection.
type ContextChaos struct {
	ctx    context.Context
	config ChaosConfig
	rng    *rand.Rand
	mu     sync.Mutex
}

// NewContextChaos creates a context-aware chaos injector.
func NewContextChaos(ctx context.Context, config ChaosConfig) *ContextChaos {
	seed := config.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return &ContextChaos{
		ctx:    ctx,
		config: config,
		rng:    rand.New(rand.NewSource(seed)),
	}
}

// MaybeError returns an error with configured probability.
func (cc *ContextChaos) MaybeError() error {
	if !cc.config.Enabled {
		return nil
	}

	cc.mu.Lock()
	r := cc.rng.Float64()
	cc.mu.Unlock()

	if r < cc.config.ErrorRate {
		return ErrChaosInjected
	}
	return nil
}

// MaybeLatency injects latency with configured probability.
func (cc *ContextChaos) MaybeLatency() {
	if !cc.config.Enabled || cc.config.Latency <= 0 {
		return
	}

	cc.mu.Lock()
	jitter := cc.rng.Float64() * cc.config.LatencyJitter
	cc.mu.Unlock()

	baseLatency := float64(cc.config.Latency)
	actualLatency := baseLatency * (1 + jitter - cc.config.LatencyJitter/2)

	select {
	case <-time.After(time.Duration(actualLatency)):
	case <-cc.ctx.Done():
	}
}

// NetworkPartition simulates a network partition.
type NetworkPartition struct {
	active    bool
	duration  time.Duration
	startTime time.Time
	mu        sync.RWMutex
}

// NewNetworkPartition creates a new network partition simulator.
func NewNetworkPartition() *NetworkPartition {
	return &NetworkPartition{}
}

// Start starts a network partition for the given duration.
func (np *NetworkPartition) Start(duration time.Duration) {
	np.mu.Lock()
	defer np.mu.Unlock()
	np.active = true
	np.duration = duration
	np.startTime = time.Now()
}

// Stop stops the network partition.
func (np *NetworkPartition) Stop() {
	np.mu.Lock()
	defer np.mu.Unlock()
	np.active = false
}

// IsActive returns true if the partition is active.
func (np *NetworkPartition) IsActive() bool {
	np.mu.RLock()
	defer np.mu.RUnlock()

	if !np.active {
		return false
	}

	// Check if duration has passed
	if np.duration > 0 && time.Since(np.startTime) > np.duration {
		np.mu.RUnlock()
		np.Stop()
		np.mu.RLock()
		return false
	}

	return true
}

// WrapTransport wraps a transport with partition simulation.
func (np *NetworkPartition) WrapTransport(t core.Transport) *PartitionedTransport {
	return &PartitionedTransport{
		wrapped:   t,
		partition: np,
	}
}

// PartitionedTransport simulates network partitions.
type PartitionedTransport struct {
	wrapped   core.Transport
	partition *NetworkPartition
}

// Send sends a message, failing if partitioned.
func (pt *PartitionedTransport) Send(msg core.Message) error {
	if pt.partition.IsActive() {
		return ErrChaosTimeout
	}
	return pt.wrapped.Send(msg)
}

// Close closes the transport.
func (pt *PartitionedTransport) Close() error {
	return pt.wrapped.Close()
}

// IsConnected returns connection status.
func (pt *PartitionedTransport) IsConnected() bool {
	if pt.partition.IsActive() {
		return false
	}
	return pt.wrapped.IsConnected()
}

// SlowTransport wraps a transport with configurable slowness.
type SlowTransport struct {
	wrapped  core.Transport
	delay    time.Duration
	jitter   float64
	rng      *rand.Rand
	mu       sync.Mutex
}

// NewSlowTransport creates a slow transport wrapper.
func NewSlowTransport(wrapped core.Transport, delay time.Duration, jitter float64) *SlowTransport {
	return &SlowTransport{
		wrapped: wrapped,
		delay:   delay,
		jitter:  jitter,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Send sends a message with artificial delay.
func (st *SlowTransport) Send(msg core.Message) error {
	st.mu.Lock()
	j := st.rng.Float64() * st.jitter
	st.mu.Unlock()

	actualDelay := float64(st.delay) * (1 + j - st.jitter/2)
	time.Sleep(time.Duration(actualDelay))

	return st.wrapped.Send(msg)
}

// Close closes the transport.
func (st *SlowTransport) Close() error {
	return st.wrapped.Close()
}

// IsConnected returns connection status.
func (st *SlowTransport) IsConnected() bool {
	return st.wrapped.IsConnected()
}

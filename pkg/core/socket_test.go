package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MockTransport implements Transport for testing.
type MockTransport struct {
	connected bool
	messages  []Message
	mu        sync.Mutex
	closeCh   chan struct{}
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		connected: true,
		messages:  make([]Message, 0),
		closeCh:   make(chan struct{}),
	}
}

func (m *MockTransport) Send(msg Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.connected {
		return ErrSocketClosed
	}
	m.messages = append(m.messages, msg)
	return nil
}

func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connected {
		m.connected = false
		close(m.closeCh)
	}
	return nil
}

func (m *MockTransport) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *MockTransport) Messages() []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Message, len(m.messages))
	copy(result, m.messages)
	return result
}

func TestNewSocket(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	if socket.ID() != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", socket.ID())
	}

	if !socket.IsConnected() {
		t.Error("expected socket to be connected")
	}

	if socket.Assigns() == nil {
		t.Error("expected assigns to be initialized")
	}
}

func TestSocket_Send(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	msg := Message{
		Topic:   "test-topic",
		Event:   "test-event",
		Payload: map[string]any{"key": "value"},
	}

	if err := socket.Send(msg); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	messages := transport.Messages()
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Event != "test-event" {
		t.Errorf("expected event 'test-event', got '%s'", messages[0].Event)
	}
}

func TestSocket_Send_Closed(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	socket.Close()

	err := socket.Send(Message{Event: "test"})
	if err != ErrSocketClosed {
		t.Errorf("expected ErrSocketClosed, got %v", err)
	}
}

func TestSocket_Send_Concurrent(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	const goroutines = 100
	const messagesPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				socket.Send(Message{
					Event:   "test",
					Payload: map[string]any{"id": id, "msg": j},
				})
			}
		}(i)
	}

	wg.Wait()

	messages := transport.Messages()
	expected := goroutines * messagesPerGoroutine
	if len(messages) != expected {
		t.Errorf("expected %d messages, got %d", expected, len(messages))
	}
}

func TestSocket_LastActivity_Atomic(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	initial := socket.LastActivity()

	// Sleep to ensure time difference
	time.Sleep(10 * time.Millisecond)

	// Send a message (should update LastActivity)
	socket.Send(Message{Event: "test"})

	after := socket.LastActivity()

	if !after.After(initial) {
		t.Error("expected LastActivity to be updated after Send")
	}
}

func TestSocket_LastActivity_Concurrent(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	// Run concurrent reads and writes
	var wg sync.WaitGroup
	const goroutines = 50

	// Writers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				socket.UpdateActivity()
			}
		}()
	}

	// Readers
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = socket.LastActivity()
			}
		}()
	}

	wg.Wait()
	// If we get here without a race condition, test passes
}

func TestSocket_Assign(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	socket.Assign("count", 42)
	socket.Assign("name", "test")

	if v := socket.Assigns().Get("count"); v != 42 {
		t.Errorf("expected count 42, got %v", v)
	}

	if v := socket.Assigns().Get("name"); v != "test" {
		t.Errorf("expected name 'test', got %v", v)
	}
}

func TestSocket_SendDiff_FullRender(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	payload := &DiffPayload{
		Version: 1,
		Full:    "<div>Hello</div>",
	}

	if err := socket.SendOptimizedDiff(payload); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	messages := transport.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Event != "diff" {
		t.Errorf("expected event 'diff', got '%s'", messages[0].Event)
	}

	if messages[0].Payload["f"] != "<div>Hello</div>" {
		t.Errorf("expected full render in payload, got %v", messages[0].Payload)
	}
}

func TestSocket_SendDiff_Nil(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	if err := socket.SendOptimizedDiff(nil); err != nil {
		t.Errorf("expected no error for nil diff, got %v", err)
	}

	if len(transport.Messages()) != 0 {
		t.Error("expected no messages for nil diff")
	}
}

func TestSocket_SendOptimizedDiff_TextSlots(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	payload := &DiffPayload{
		Version: 2,
		Slots: map[string]string{
			"count": "42",
			"name":  "test",
		},
	}

	if err := socket.SendOptimizedDiff(payload); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	messages := transport.Messages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Event != "diff" {
		t.Errorf("expected event 'diff', got '%s'", messages[0].Event)
	}

	slots, ok := messages[0].Payload["s"].(map[string]string)
	if !ok || slots["count"] != "42" {
		t.Errorf("expected slots with count=42, got %v", messages[0].Payload["s"])
	}
}

func TestSocket_SendOptimizedDiff_Empty(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	// Empty payload should not send anything
	payload := &DiffPayload{Version: 1}

	if err := socket.SendOptimizedDiff(payload); err != nil {
		t.Errorf("expected no error for empty diff, got %v", err)
	}

	if len(transport.Messages()) != 0 {
		t.Error("expected no messages for empty diff")
	}
}

func TestDiffPayload_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		payload  *DiffPayload
		expected bool
	}{
		{"nil slots", &DiffPayload{}, true},
		{"with text slots", &DiffPayload{Slots: map[string]string{"a": "1"}}, false},
		{"with html slots", &DiffPayload{HTMLSlots: map[string]string{"a": "1"}}, false},
		{"with list ops", &DiffPayload{ListOps: map[string][]ListOp{"a": {{Op: "i"}}}}, false},
		{"with full", &DiffPayload{Full: "<div>"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.payload.IsEmpty(); got != tt.expected {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDiffPayload_Size(t *testing.T) {
	payload := &DiffPayload{
		Slots:     map[string]string{"a": "123"},
		HTMLSlots: map[string]string{"b": "4567"},
		ListOps: map[string][]ListOp{
			"c": {{Op: "i", Content: "89"}},
		},
		Full: "0",
	}

	// 3 + 4 + 2 + 1 = 10
	if size := payload.Size(); size != 10 {
		t.Errorf("Size() = %d, want 10", size)
	}
}

func TestSocketManager_Add_Remove(t *testing.T) {
	sm := NewSocketManager()

	socket1 := NewSocket("socket-1", NewMockTransport())
	socket2 := NewSocket("socket-2", NewMockTransport())

	sm.Add(socket1)
	sm.Add(socket2)

	if sm.Count() != 2 {
		t.Errorf("expected count 2, got %d", sm.Count())
	}

	s, ok := sm.Get("socket-1")
	if !ok || s.ID() != "socket-1" {
		t.Error("expected to find socket-1")
	}

	sm.Remove("socket-1")

	if sm.Count() != 1 {
		t.Errorf("expected count 1, got %d", sm.Count())
	}

	_, ok = sm.Get("socket-1")
	if ok {
		t.Error("expected socket-1 to be removed")
	}
}

func TestSocketManager_Broadcast(t *testing.T) {
	sm := NewSocketManager()

	transports := make([]*MockTransport, 10)
	for i := 0; i < 10; i++ {
		transport := NewMockTransport()
		transports[i] = transport
		sm.Add(NewSocket("socket-"+string(rune('0'+i)), transport))
	}

	msg := Message{
		Topic: "broadcast",
		Event: "test",
	}

	sm.Broadcast(msg)

	for i, transport := range transports {
		messages := transport.Messages()
		if len(messages) != 1 {
			t.Errorf("socket %d: expected 1 message, got %d", i, len(messages))
		}
	}
}

func TestSocketManager_Broadcast_Concurrent(t *testing.T) {
	sm := NewSocketManager()

	// Add sockets
	for i := 0; i < 100; i++ {
		sm.Add(NewSocket("socket-"+string(rune(i)), NewMockTransport()))
	}

	// Concurrent broadcasts
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sm.Broadcast(Message{Event: "test", Payload: map[string]any{"id": id}})
		}(i)
	}

	wg.Wait()
	// If we get here without a deadlock, test passes
}

func TestSocketManager_CleanupInactive(t *testing.T) {
	sm := NewSocketManager()

	// Add sockets with old activity
	for i := 0; i < 5; i++ {
		socket := NewSocket("socket-"+string(rune('0'+i)), NewMockTransport())
		sm.Add(socket)
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Add fresh sockets
	for i := 5; i < 10; i++ {
		socket := NewSocket("socket-"+string(rune('0'+i)), NewMockTransport())
		sm.Add(socket)
		socket.UpdateActivity() // Mark as active
	}

	// Cleanup with short timeout
	ctx := context.Background()
	removed := sm.CleanupInactive(ctx, 50*time.Millisecond)

	if removed != 5 {
		t.Errorf("expected to remove 5 sockets, removed %d", removed)
	}

	if sm.Count() != 5 {
		t.Errorf("expected 5 sockets remaining, got %d", sm.Count())
	}
}

// Benchmark tests

func BenchmarkSocket_Send(b *testing.B) {
	transport := NewMockTransport()
	socket := NewSocket("bench-id", transport)
	msg := Message{Event: "test", Payload: map[string]any{"key": "value"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		socket.Send(msg)
	}
}

func BenchmarkSocket_Send_Parallel(b *testing.B) {
	transport := NewMockTransport()
	socket := NewSocket("bench-id", transport)
	msg := Message{Event: "test", Payload: map[string]any{"key": "value"}}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			socket.Send(msg)
		}
	})
}

func BenchmarkSocket_LastActivity(b *testing.B) {
	transport := NewMockTransport()
	socket := NewSocket("bench-id", transport)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = socket.LastActivity()
	}
}

func BenchmarkSocket_UpdateActivity(b *testing.B) {
	transport := NewMockTransport()
	socket := NewSocket("bench-id", transport)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		socket.UpdateActivity()
	}
}

func BenchmarkSocketManager_Broadcast_100(b *testing.B) {
	sm := NewSocketManager()
	for i := 0; i < 100; i++ {
		sm.Add(NewSocket("socket-"+string(rune(i)), NewMockTransport()))
	}
	msg := Message{Event: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Broadcast(msg)
	}
}

func BenchmarkSocketManager_Broadcast_1000(b *testing.B) {
	sm := NewSocketManager()
	for i := 0; i < 1000; i++ {
		sm.Add(NewSocket("socket-"+string(rune(i)), NewMockTransport()))
	}
	msg := Message{Event: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Broadcast(msg)
	}
}

// Race detection test - run with -race flag
func TestSocket_RaceCondition(t *testing.T) {
	transport := NewMockTransport()
	socket := NewSocket("test-id", transport)

	var ops atomic.Int64
	const iterations = 1000

	var wg sync.WaitGroup

	// Concurrent sends
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				socket.Send(Message{Event: "test"})
				ops.Add(1)
			}
		}()
	}

	// Concurrent activity updates
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				socket.UpdateActivity()
				ops.Add(1)
			}
		}()
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = socket.LastActivity()
				_ = socket.IsConnected()
				ops.Add(1)
			}
		}()
	}

	wg.Wait()

	expected := int64(30 * iterations)
	if ops.Load() != expected {
		t.Errorf("expected %d operations, got %d", expected, ops.Load())
	}
}

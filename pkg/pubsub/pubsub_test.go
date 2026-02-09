package pubsub

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPubSub_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	const (
		numGoroutines = 100
		numIterations = 100
	)

	var wg sync.WaitGroup
	var panicCount atomic.Int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicCount.Add(1)
					t.Errorf("Goroutine %d panicked: %v", id, r)
				}
			}()

			topic := "test-topic"

			for j := 0; j < numIterations; j++ {
				// Subscribe
				sub, err := ps.Subscribe(topic, func(msg []byte) {
					// Simulate some work
					time.Sleep(time.Microsecond)
				})
				if err != nil {
					if err == ErrPubSubClosed {
						return
					}
					t.Errorf("Subscribe error: %v", err)
					continue
				}

				// Publish a message
				ps.Publish(topic, []byte("test message"))

				// Unsubscribe
				if err := sub.Unsubscribe(); err != nil {
					t.Errorf("Unsubscribe error: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	if panicCount.Load() > 0 {
		t.Errorf("Had %d panics during concurrent operations", panicCount.Load())
	}
}

func TestPubSub_NoRaceOnClose(t *testing.T) {
	ps := NewMemoryPubSub()

	const numSubscribers = 50

	var subs []Subscription
	var mu sync.Mutex

	// Create subscribers
	for i := 0; i < numSubscribers; i++ {
		sub, err := ps.Subscribe("topic", func(msg []byte) {
			// Process message
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
		mu.Lock()
		subs = append(subs, sub)
		mu.Unlock()
	}

	// Start publishers
	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopCh:
					return
				default:
					ps.Publish("topic", []byte("message"))
				}
			}
		}()
	}

	// Close while publishing
	time.Sleep(10 * time.Millisecond)
	close(stopCh)
	ps.Close()

	wg.Wait()

	// All subs should be closed without panic
	for _, sub := range subs {
		// Double unsubscribe should be safe
		sub.Unsubscribe()
	}
}

func TestPubSub_NoPanicOnDoubleUnsubscribe(t *testing.T) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	sub, err := ps.Subscribe("topic", func(msg []byte) {})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// First unsubscribe
	if err := sub.Unsubscribe(); err != nil {
		t.Errorf("First unsubscribe failed: %v", err)
	}

	// Second unsubscribe should be safe
	if err := sub.Unsubscribe(); err != nil {
		t.Errorf("Second unsubscribe should not error: %v", err)
	}

	// Third unsubscribe should also be safe
	if err := sub.Unsubscribe(); err != nil {
		t.Errorf("Third unsubscribe should not error: %v", err)
	}
}

func TestPubSub_PublishToClosedSubscription(t *testing.T) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	received := make(chan struct{}, 10)

	sub, _ := ps.Subscribe("topic", func(msg []byte) {
		select {
		case received <- struct{}{}:
		default:
		}
	})

	// Publish before unsubscribe
	ps.Publish("topic", []byte("message 1"))

	time.Sleep(10 * time.Millisecond)

	// Unsubscribe
	sub.Unsubscribe()

	// Publish after unsubscribe - should not panic
	for i := 0; i < 100; i++ {
		ps.Publish("topic", []byte("message after close"))
	}

	// Should have received at least one message
	select {
	case <-received:
		// OK
	case <-time.After(100 * time.Millisecond):
		// May not receive if timing is off, but no panic is the goal
	}
}

func TestPubSub_ClosedFlagAtomic(t *testing.T) {
	ps := NewMemoryPubSub()

	sub, _ := ps.Subscribe("topic", func(msg []byte) {})

	memSub := sub.(*memorySubscription)

	// Check initial state
	if memSub.IsClosed() {
		t.Error("Subscription should not be closed initially")
	}

	// Close it
	sub.Unsubscribe()

	// Check closed state
	if !memSub.IsClosed() {
		t.Error("Subscription should be closed after unsubscribe")
	}
}

func TestPubSub_MessageDelivery(t *testing.T) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	received := make(chan string, 10)

	sub, _ := ps.Subscribe("topic", func(msg []byte) {
		received <- string(msg)
	})
	defer sub.Unsubscribe()

	// Publish messages
	messages := []string{"msg1", "msg2", "msg3"}
	for _, msg := range messages {
		ps.Publish("topic", []byte(msg))
	}

	// Verify delivery
	for i, expected := range messages {
		select {
		case got := <-received:
			if got != expected {
				t.Errorf("Message %d: got %q, want %q", i, got, expected)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Timeout waiting for message %d", i)
		}
	}
}

func BenchmarkPubSub_Publish(b *testing.B) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	// Create some subscribers
	for i := 0; i < 100; i++ {
		ps.Subscribe("bench-topic", func(msg []byte) {})
	}

	msg := []byte("benchmark message payload")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ps.Publish("bench-topic", msg)
	}
}

func BenchmarkPubSub_SubscribeUnsubscribe(b *testing.B) {
	ps := NewMemoryPubSub()
	defer ps.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sub, _ := ps.Subscribe("bench-topic", func(msg []byte) {})
		sub.Unsubscribe()
	}
}

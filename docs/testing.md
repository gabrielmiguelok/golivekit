# Testing in GoliveKit

GoliveKit provides comprehensive testing utilities including unit testing, chaos testing, and fuzz testing.

## Unit Testing Components

### Basic Component Test

```go
func TestCounter(t *testing.T) {
    testing.Mount(t, NewCounter()).
        AssertText("0").
        Click("[lv-click=increment]").
        AssertText("1").
        Click("[lv-click=increment]").
        AssertText("2").
        Click("[lv-click=decrement]").
        AssertText("1")
}
```

### Testing with Initial State

```go
func TestCounterWithInitialValue(t *testing.T) {
    counter := &Counter{Count: 10}

    testing.Mount(t, counter).
        AssertText("10").
        AssertAssign("count", 10)
}
```

### Testing Events with Payloads

```go
func TestTodo(t *testing.T) {
    testing.Mount(t, NewTodo()).
        Submit("form", map[string]any{"text": "Buy milk"}).
        AssertText("Buy milk").
        Click("[data-key='1'] [lv-click=delete]").
        AssertNoText("Buy milk")
}
```

### Assertion Methods

| Method | Description |
|--------|-------------|
| `AssertText(text)` | Assert text exists in rendered HTML |
| `AssertNoText(text)` | Assert text does not exist |
| `AssertHTML(html)` | Assert exact HTML match |
| `AssertAssign(key, value)` | Assert assign value equals expected |
| `AssertAssignExists(key)` | Assert assign key exists |
| `AssertClass(selector, class)` | Assert element has class |
| `AssertAttr(selector, attr, value)` | Assert element attribute |
| `AssertVisible(selector)` | Assert element is visible |
| `AssertHidden(selector)` | Assert element is hidden |
| `AssertCount(selector, n)` | Assert number of matching elements |
| `AssertEnabled(selector)` | Assert element is enabled |
| `AssertDisabled(selector)` | Assert element is disabled |
| `AssertFocused(selector)` | Assert element is focused |

### Action Methods

| Method | Description |
|--------|-------------|
| `Click(selector)` | Simulate click event |
| `Input(selector, value)` | Simulate input event |
| `Change(selector, value)` | Simulate change event |
| `Submit(selector, data)` | Simulate form submission |
| `Focus(selector)` | Focus an element |
| `Blur(selector)` | Blur an element |
| `Hover(selector)` | Hover over element |
| `Wait(duration)` | Wait for specified duration |
| `WaitFor(condition)` | Wait for condition to be true |

## Mock Objects

### MockSocket

```go
func TestSocketCommunication(t *testing.T) {
    socket := testing.NewMockSocket()

    // Simulate receiving a message
    socket.SimulateReceive(core.Message{
        Event:   "user_joined",
        Payload: map[string]any{"user": "alice"},
    })

    // Check sent messages
    sent := socket.GetSentMessages()
    if len(sent) != 1 {
        t.Errorf("expected 1 message, got %d", len(sent))
    }

    // Assert specific message was sent
    socket.AssertSent(t, "user_joined", map[string]any{"user": "alice"})
}
```

### MockPubSub

```go
func TestPubSubBroadcast(t *testing.T) {
    ps := testing.NewMockPubSub()

    // Subscribe to topic
    ps.Subscribe("room:123")

    // Broadcast message
    ps.Broadcast("room:123", "new_message", map[string]any{
        "text": "Hello!",
    })

    // Assert message was published
    ps.AssertPublished(t, "room:123", "new_message")

    // Get all published messages
    messages := ps.GetPublishedMessages()
}
```

### MockTimer

```go
func TestTimerBasedLogic(t *testing.T) {
    timer := testing.NewMockTimer()

    component := NewAutoSave(timer)
    testing.Mount(t, component)

    // Advance time manually
    timer.Advance(5 * time.Second)

    // Assert auto-save was triggered
    component.AssertSaved(t)
}
```

## Chaos Testing

Test resilience by injecting faults and network issues.

### ChaosTransport

```go
import "github.com/gabrielmiguelok/golivekit/pkg/testing"

func TestWithNetworkIssues(t *testing.T) {
    transport := createTestTransport()

    // Wrap with chaos injection
    chaos := testing.NewChaosTransport(transport, testing.DefaultChaosConfig())

    // Configure specific behaviors
    chaos.SetLatency(100*time.Millisecond, 50*time.Millisecond) // mean, stddev
    chaos.SetDropRate(0.05)  // 5% message drop rate
    chaos.SetErrorRate(0.01) // 1% error rate

    // Run your test
    // Messages will experience latency, drops, and errors
}
```

### Chaos Configurations

```go
// Mild faults - occasional issues
config := testing.MildChaosConfig()

// Default - moderate faults
config := testing.DefaultChaosConfig()

// Severe - stress testing
config := testing.SevereChaosConfig()
```

| Config | Latency | Drop Rate | Error Rate |
|--------|---------|-----------|------------|
| Mild | 10-30ms | 1% | 0.1% |
| Default | 50-100ms | 5% | 1% |
| Severe | 100-500ms | 20% | 5% |

### FaultInjector

```go
func TestErrorHandling(t *testing.T) {
    injector := testing.NewFaultInjector()

    // Inject specific errors
    injector.InjectError("connect", errors.New("connection refused"))
    injector.InjectError("send", errors.New("write timeout"))

    // Clear faults
    injector.ClearFault("connect")

    // Clear all faults
    injector.ClearAll()
}
```

### NetworkPartition

Simulate network partitions:

```go
func TestPartitionRecovery(t *testing.T) {
    partition := testing.NewNetworkPartition()

    // Partition the network
    partition.Isolate("node-1", "node-2")

    // Messages between node-1 and node-2 will fail

    // Heal the partition
    partition.Heal("node-1", "node-2")
}
```

## Fuzz Testing

GoliveKit includes fuzz tests for protocol parsing.

### Running Fuzz Tests

```bash
# Fuzz message parsing (30 seconds)
go test -fuzz=FuzzParseMessage ./pkg/protocol/... -fuzztime=30s

# Fuzz Phoenix tuple parsing
go test -fuzz=FuzzParsePhoenixTuple ./pkg/protocol/... -fuzztime=30s

# Fuzz codec encoding/decoding
go test -fuzz=FuzzCodec ./pkg/protocol/... -fuzztime=30s
```

### Fuzz Test Targets

| Target | Description |
|--------|-------------|
| `FuzzParseMessage` | Tests JSON message parsing |
| `FuzzParsePhoenixTuple` | Tests Phoenix protocol parsing |
| `FuzzCodec` | Tests encode/decode roundtrip |

### Writing Custom Fuzz Tests

```go
func FuzzMyParser(f *testing.F) {
    // Seed corpus with valid inputs
    f.Add([]byte(`{"event":"click"}`))
    f.Add([]byte(`{"event":"submit","payload":{}}`))

    f.Fuzz(func(t *testing.T, data []byte) {
        // Parser should not panic on any input
        result, err := ParseMessage(data)

        if err == nil {
            // If parsing succeeded, result should be valid
            if result.Event == "" {
                t.Error("empty event on successful parse")
            }
        }
    })
}
```

## Integration Testing

### Test Server

```go
func TestIntegration(t *testing.T) {
    // Create test server
    server := testing.NewTestServer(t)
    defer server.Close()

    // Register components
    server.Register("counter", NewCounter)

    // Create test client
    client := server.NewClient()

    // Navigate to page
    client.Navigate("/")

    // Interact
    client.Click("[lv-click=increment]")

    // Assert
    client.AssertText("1")
}
```

### WebSocket Testing

```go
func TestWebSocket(t *testing.T) {
    server := testing.NewTestServer(t)
    defer server.Close()

    // Connect via WebSocket
    ws := server.ConnectWebSocket("/ws")
    defer ws.Close()

    // Send message
    ws.Send(core.Message{
        Event:   "increment",
        Payload: nil,
    })

    // Receive response
    msg := ws.Receive()

    // Assert diff was sent
    if msg.Event != "diff" {
        t.Errorf("expected diff, got %s", msg.Event)
    }
}
```

## Benchmarking

### Component Benchmarks

```go
func BenchmarkCounterRender(b *testing.B) {
    counter := NewCounter()
    counter.Mount(context.Background(), nil, nil)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        counter.Render(context.Background())
    }
}

func BenchmarkCounterEvent(b *testing.B) {
    counter := NewCounter()
    counter.Mount(context.Background(), nil, nil)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        counter.HandleEvent(context.Background(), "increment", nil)
    }
}
```

### Running Benchmarks

```bash
# Run all benchmarks
go test ./... -bench=. -benchmem

# Run specific benchmark
go test ./pkg/core -bench=BenchmarkSocket -benchmem

# Run with CPU profiling
go test ./pkg/core -bench=. -cpuprofile=cpu.prof

# Run with memory profiling
go test ./pkg/core -bench=. -memprofile=mem.prof
```

## Test Coverage

```bash
# Generate coverage report
go test ./... -coverprofile=coverage.out

# View in browser
go tool cover -html=coverage.out

# Show coverage percentage
go tool cover -func=coverage.out
```

## CI/CD Integration

GoliveKit includes GitHub Actions workflow for testing:

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go: ['1.21', '1.22', '1.23']

    steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: ${{ matrix.go }}

    - name: Test with race detector
      run: go test ./... -race -v

    - name: Benchmarks
      run: go test ./... -bench=. -benchmem
```

// Package testing provides testing utilities for GoliveKit components.
// It enables testing LiveView components without a browser or WebSocket connection.
package testing

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/gabrielmiguelok/golivekit/pkg/core"
)

// LiveViewTest provides a testing harness for LiveView components.
type LiveViewTest struct {
	component core.Component
	socket    *MockSocket
	assigns   *core.Assigns
	rendered  string
	events    []core.Event
	t         *testing.T
}

// MountOption configures the test mount.
type MountOption func(*LiveViewTest)

// WithParams sets mount parameters.
func WithParams(params core.Params) MountOption {
	return func(lvt *LiveViewTest) {
		lvt.socket.SetMetadata("params", params)
	}
}

// WithSession sets session data.
func WithSession(session core.Session) MountOption {
	return func(lvt *LiveViewTest) {
		lvt.socket.SetMetadata("session", session)
	}
}

// WithAssigns pre-sets assigns.
func WithAssigns(assigns map[string]any) MountOption {
	return func(lvt *LiveViewTest) {
		lvt.assigns.SetAll(assigns)
	}
}

// Mount creates and mounts a component for testing.
func Mount(t *testing.T, comp core.Component, opts ...MountOption) *LiveViewTest {
	t.Helper()

	lvt := &LiveViewTest{
		component: comp,
		socket:    NewMockSocket(),
		assigns:   core.NewAssigns(),
		t:         t,
	}

	// Apply options
	for _, opt := range opts {
		opt(lvt)
	}

	// Set socket on component if it has SetSocket method
	if setter, ok := comp.(interface{ SetSocket(*core.Socket) }); ok {
		// Create a real socket from mock transport
		realSocket := core.NewSocket(lvt.socket.ID, lvt.socket)
		setter.SetSocket(realSocket)
	}

	// Get params and session
	params, _ := lvt.socket.GetMetadata("params").(core.Params)
	if params == nil {
		params = core.Params{}
	}

	session, _ := lvt.socket.GetMetadata("session").(core.Session)
	if session == nil {
		session = core.Session{}
	}

	// Mount the component
	ctx := context.Background()
	if err := comp.Mount(ctx, params, session); err != nil {
		t.Fatalf("Mount failed: %v", err)
	}

	// Initial render
	lvt.render()

	return lvt
}

// Click simulates a click event.
func (lvt *LiveViewTest) Click(selector string, opts ...EventOption) *LiveViewTest {
	lvt.t.Helper()

	event := core.Event{
		Type:   "click",
		Target: selector,
	}

	for _, opt := range opts {
		opt(&event)
	}

	lvt.pushEvent(event)
	return lvt
}

// Change simulates an input change event.
func (lvt *LiveViewTest) Change(selector string, value string) *LiveViewTest {
	lvt.t.Helper()

	event := core.Event{
		Type:    "change",
		Target:  selector,
		Payload: map[string]any{"value": value},
	}

	lvt.pushEvent(event)
	return lvt
}

// Input simulates an input event.
func (lvt *LiveViewTest) Input(selector string, value string) *LiveViewTest {
	lvt.t.Helper()

	event := core.Event{
		Type:    "input",
		Target:  selector,
		Payload: map[string]any{"value": value},
	}

	lvt.pushEvent(event)
	return lvt
}

// Submit simulates a form submission.
func (lvt *LiveViewTest) Submit(selector string, data map[string]string) *LiveViewTest {
	lvt.t.Helper()

	payload := make(map[string]any)
	for k, v := range data {
		payload[k] = v
	}

	event := core.Event{
		Type:    "submit",
		Target:  selector,
		Payload: payload,
	}

	lvt.pushEvent(event)
	return lvt
}

// Focus simulates a focus event.
func (lvt *LiveViewTest) Focus(selector string) *LiveViewTest {
	lvt.t.Helper()

	event := core.Event{
		Type:   "focus",
		Target: selector,
	}

	lvt.pushEvent(event)
	return lvt
}

// Blur simulates a blur event.
func (lvt *LiveViewTest) Blur(selector string) *LiveViewTest {
	lvt.t.Helper()

	event := core.Event{
		Type:   "blur",
		Target: selector,
	}

	lvt.pushEvent(event)
	return lvt
}

// EventOption configures an event.
type EventOption func(*core.Event)

// WithPayload adds payload to the event.
func WithPayload(payload map[string]any) EventOption {
	return func(e *core.Event) {
		if e.Payload == nil {
			e.Payload = make(map[string]any)
		}
		for k, v := range payload {
			e.Payload[k] = v
		}
	}
}

// WithValue adds a value to the event payload.
func WithValue(key string, value any) EventOption {
	return func(e *core.Event) {
		if e.Payload == nil {
			e.Payload = make(map[string]any)
		}
		e.Payload[key] = value
	}
}

// pushEvent processes an event and re-renders.
func (lvt *LiveViewTest) pushEvent(event core.Event) {
	lvt.events = append(lvt.events, event)

	ctx := context.Background()
	if err := lvt.component.HandleEvent(ctx, event.Type, event.Payload); err != nil {
		lvt.t.Errorf("HandleEvent failed: %v", err)
		return
	}

	lvt.render()
}

// SendInfo sends an info message to the component.
func (lvt *LiveViewTest) SendInfo(msg any) *LiveViewTest {
	lvt.t.Helper()

	ctx := context.Background()
	if err := lvt.component.HandleInfo(ctx, msg); err != nil {
		lvt.t.Errorf("HandleInfo failed: %v", err)
		return lvt
	}

	lvt.render()
	return lvt
}

// render re-renders the component.
func (lvt *LiveViewTest) render() {
	ctx := context.Background()
	renderer := lvt.component.Render(ctx)

	var buf bytes.Buffer
	if err := renderer.Render(ctx, &buf); err != nil {
		lvt.t.Fatalf("Render failed: %v", err)
	}

	lvt.rendered = buf.String()
}

// Rendered returns the current rendered HTML.
func (lvt *LiveViewTest) Rendered() string {
	return lvt.rendered
}

// AssertHasElement verifies that an element exists in the rendered output.
func (lvt *LiveViewTest) AssertHasElement(selector string) *LiveViewTest {
	lvt.t.Helper()

	// Simple check - in production would use proper HTML parsing
	if !strings.Contains(lvt.rendered, selector) {
		lvt.t.Errorf("Element not found: %s\nRendered HTML:\n%s", selector, lvt.rendered)
	}

	return lvt
}

// AssertNoElement verifies that an element does not exist.
func (lvt *LiveViewTest) AssertNoElement(selector string) *LiveViewTest {
	lvt.t.Helper()

	if strings.Contains(lvt.rendered, selector) {
		lvt.t.Errorf("Element should not exist: %s", selector)
	}

	return lvt
}

// AssertText verifies the rendered output contains text.
func (lvt *LiveViewTest) AssertText(text string) *LiveViewTest {
	lvt.t.Helper()

	if !strings.Contains(lvt.rendered, text) {
		lvt.t.Errorf("Text not found: %q\nRendered HTML:\n%s", text, lvt.rendered)
	}

	return lvt
}

// AssertNoText verifies the rendered output does not contain text.
func (lvt *LiveViewTest) AssertNoText(text string) *LiveViewTest {
	lvt.t.Helper()

	if strings.Contains(lvt.rendered, text) {
		lvt.t.Errorf("Text should not exist: %q", text)
	}

	return lvt
}

// AssertAssign verifies an assign value.
func (lvt *LiveViewTest) AssertAssign(key string, expected any) *LiveViewTest {
	lvt.t.Helper()

	// Get assigns from component if available via interface
	var actual any
	if getter, ok := lvt.component.(interface{ Assigns() *core.Assigns }); ok {
		actual = getter.Assigns().Get(key)
	} else {
		// Fall back to our local assigns
		actual = lvt.assigns.Get(key)
	}

	if !reflect.DeepEqual(actual, expected) {
		lvt.t.Errorf("Assign %s mismatch:\n  Expected: %v (%T)\n  Actual:   %v (%T)",
			key, expected, expected, actual, actual)
	}

	return lvt
}

// AssertAssignExists verifies an assign exists.
func (lvt *LiveViewTest) AssertAssignExists(key string) *LiveViewTest {
	lvt.t.Helper()

	var exists bool
	if getter, ok := lvt.component.(interface{ Assigns() *core.Assigns }); ok {
		exists = getter.Assigns().Get(key) != nil
	}

	if !exists {
		lvt.t.Errorf("Assign %s should exist", key)
	}

	return lvt
}

// AssertSocketSentCount verifies the number of messages sent.
func (lvt *LiveViewTest) AssertSocketSentCount(count int) *LiveViewTest {
	lvt.t.Helper()

	actual := lvt.socket.SentCount()
	if actual != count {
		lvt.t.Errorf("Socket sent count mismatch: expected %d, got %d", count, actual)
	}

	return lvt
}

// Socket returns the mock socket.
func (lvt *LiveViewTest) Socket() *MockSocket {
	return lvt.socket
}

// Component returns the component under test.
func (lvt *LiveViewTest) Component() core.Component {
	return lvt.component
}

// Events returns all events that were processed.
func (lvt *LiveViewTest) Events() []core.Event {
	return lvt.events
}

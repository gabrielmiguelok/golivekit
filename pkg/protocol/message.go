// Package protocol defines the wire protocol for GoliveKit communication.
package protocol

import (
	"encoding/json"
	"time"
)

// MessageType identifies the type of protocol message.
type MessageType uint8

const (
	// MsgJoin is sent when a client joins a channel.
	MsgJoin MessageType = iota
	// MsgLeave is sent when a client leaves a channel.
	MsgLeave
	// MsgEvent is sent for user interactions.
	MsgEvent
	// MsgReply is sent as a response to a request.
	MsgReply
	// MsgDiff is sent when component state changes.
	MsgDiff
	// MsgError is sent when an error occurs.
	MsgError
	// MsgHeartbeat is sent for connection keepalive.
	MsgHeartbeat
	// MsgBroadcast is sent to all clients in a channel.
	MsgBroadcast
	// MsgPresence is sent for presence updates.
	MsgPresence
)

// String returns a string representation of the message type.
func (mt MessageType) String() string {
	switch mt {
	case MsgJoin:
		return "join"
	case MsgLeave:
		return "leave"
	case MsgEvent:
		return "event"
	case MsgReply:
		return "reply"
	case MsgDiff:
		return "diff"
	case MsgError:
		return "error"
	case MsgHeartbeat:
		return "heartbeat"
	case MsgBroadcast:
		return "broadcast"
	case MsgPresence:
		return "presence"
	default:
		return "unknown"
	}
}

// Message represents a protocol message exchanged between client and server.
type Message struct {
	// Type identifies what kind of message this is
	Type MessageType `json:"t" msgpack:"t"`

	// Ref is a correlation ID for request/response matching
	Ref string `json:"ref,omitempty" msgpack:"ref,omitempty"`

	// Topic is the channel this message belongs to (e.g., "lv:socket-id")
	Topic string `json:"topic" msgpack:"topic"`

	// Event is the specific event name (e.g., "click", "submit")
	Event string `json:"event,omitempty" msgpack:"event,omitempty"`

	// Payload contains the message data
	Payload map[string]any `json:"payload,omitempty" msgpack:"payload,omitempty"`

	// Timestamp when the message was created
	Timestamp int64 `json:"ts,omitempty" msgpack:"ts,omitempty"`

	// JoinRef is the join reference for the channel
	JoinRef string `json:"join_ref,omitempty" msgpack:"join_ref,omitempty"`
}

// NewMessage creates a new message with the given parameters.
func NewMessage(msgType MessageType, topic, event string) *Message {
	return &Message{
		Type:      msgType,
		Topic:     topic,
		Event:     event,
		Payload:   make(map[string]any),
		Timestamp: time.Now().UnixMilli(),
	}
}

// WithRef adds a reference ID to the message.
func (m *Message) WithRef(ref string) *Message {
	m.Ref = ref
	return m
}

// WithPayload sets the message payload.
func (m *Message) WithPayload(payload map[string]any) *Message {
	m.Payload = payload
	return m
}

// WithJoinRef sets the join reference.
func (m *Message) WithJoinRef(joinRef string) *Message {
	m.JoinRef = joinRef
	return m
}

// SetPayloadValue sets a single value in the payload.
func (m *Message) SetPayloadValue(key string, value any) *Message {
	if m.Payload == nil {
		m.Payload = make(map[string]any)
	}
	m.Payload[key] = value
	return m
}

// GetPayloadString retrieves a string value from the payload.
func (m *Message) GetPayloadString(key string) string {
	if m.Payload == nil {
		return ""
	}
	if v, ok := m.Payload[key].(string); ok {
		return v
	}
	return ""
}

// GetPayloadInt retrieves an int value from the payload.
func (m *Message) GetPayloadInt(key string) int {
	if m.Payload == nil {
		return 0
	}
	switch v := m.Payload[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

// GetPayloadBool retrieves a bool value from the payload.
func (m *Message) GetPayloadBool(key string) bool {
	if m.Payload == nil {
		return false
	}
	if v, ok := m.Payload[key].(bool); ok {
		return v
	}
	return false
}

// IsReply returns true if this message is a reply.
func (m *Message) IsReply() bool {
	return m.Type == MsgReply
}

// IsError returns true if this message is an error.
func (m *Message) IsError() bool {
	return m.Type == MsgError
}

// IsHeartbeat returns true if this is a heartbeat message.
func (m *Message) IsHeartbeat() bool {
	return m.Type == MsgHeartbeat
}

// Clone creates a copy of the message.
func (m *Message) Clone() *Message {
	clone := &Message{
		Type:      m.Type,
		Ref:       m.Ref,
		Topic:     m.Topic,
		Event:     m.Event,
		Timestamp: m.Timestamp,
		JoinRef:   m.JoinRef,
	}
	if m.Payload != nil {
		clone.Payload = make(map[string]any, len(m.Payload))
		for k, v := range m.Payload {
			clone.Payload[k] = v
		}
	}
	return clone
}

// Reply convenience functions

// JoinMessage creates a join message.
func JoinMessage(topic string, params map[string]any) *Message {
	return NewMessage(MsgJoin, topic, "phx_join").WithPayload(params)
}

// LeaveMessage creates a leave message.
func LeaveMessage(topic string) *Message {
	return NewMessage(MsgLeave, topic, "phx_leave")
}

// EventMessage creates an event message.
func EventMessage(topic, event string, payload map[string]any) *Message {
	return NewMessage(MsgEvent, topic, event).WithPayload(payload)
}

// ReplyMessage creates a reply message.
func ReplyMessage(ref, topic string, status string, response map[string]any) *Message {
	return NewMessage(MsgReply, topic, "phx_reply").
		WithRef(ref).
		WithPayload(map[string]any{
			"status":   status,
			"response": response,
		})
}

// OkReply creates a successful reply message.
func OkReply(ref, topic string, response map[string]any) *Message {
	return ReplyMessage(ref, topic, "ok", response)
}

// ErrorReply creates an error reply message.
func ErrorReply(ref, topic string, reason string) *Message {
	return ReplyMessage(ref, topic, "error", map[string]any{"reason": reason})
}

// DiffMessage creates a diff message.
func DiffMessage(topic string, diff map[string]any) *Message {
	return NewMessage(MsgDiff, topic, "diff").WithPayload(diff)
}

// HeartbeatMessage creates a heartbeat message.
func HeartbeatMessage() *Message {
	return NewMessage(MsgHeartbeat, "phoenix", "heartbeat")
}

// BroadcastMessage creates a broadcast message.
func BroadcastMessage(topic, event string, payload map[string]any) *Message {
	return NewMessage(MsgBroadcast, topic, event).WithPayload(payload)
}

// PresenceMessage creates a presence update message.
func PresenceMessage(topic string, diff map[string]any) *Message {
	return NewMessage(MsgPresence, topic, "presence_diff").WithPayload(diff)
}

// Envelope wraps messages for batching.
type Envelope struct {
	Messages []*Message `json:"messages"`
}

// NewEnvelope creates a new envelope.
func NewEnvelope() *Envelope {
	return &Envelope{
		Messages: make([]*Message, 0),
	}
}

// Add adds a message to the envelope.
func (e *Envelope) Add(msg *Message) {
	e.Messages = append(e.Messages, msg)
}

// Count returns the number of messages.
func (e *Envelope) Count() int {
	return len(e.Messages)
}

// IsEmpty returns true if the envelope has no messages.
func (e *Envelope) IsEmpty() bool {
	return len(e.Messages) == 0
}

// MarshalJSON implements json.Marshaler for the envelope.
func (e *Envelope) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Messages)
}

// UnmarshalJSON implements json.Unmarshaler for the envelope.
func (e *Envelope) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &e.Messages)
}

package protocol

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/vmihailenco/msgpack/v5"
)

// Common codec errors.
var (
	ErrInvalidMessage = errors.New("invalid message format")
	ErrUnknownCodec   = errors.New("unknown codec type")
)

// Codec handles message encoding/decoding.
type Codec interface {
	// Encode serializes a message to bytes.
	Encode(msg *Message) ([]byte, error)

	// Decode deserializes bytes to a message.
	Decode(data []byte) (*Message, error)

	// Name returns the codec name.
	Name() string

	// ContentType returns the MIME type.
	ContentType() string
}

// JSONCodec implements Codec using JSON encoding.
// Good for debugging and development.
type JSONCodec struct{}

// NewJSONCodec creates a new JSON codec.
func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

// Encode encodes a message to JSON.
func (c *JSONCodec) Encode(msg *Message) ([]byte, error) {
	return json.Marshal(msg)
}

// Decode decodes JSON to a message.
func (c *JSONCodec) Decode(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Name returns "json".
func (c *JSONCodec) Name() string {
	return "json"
}

// ContentType returns the JSON MIME type.
func (c *JSONCodec) ContentType() string {
	return "application/json"
}

// MsgPackCodec implements Codec using MessagePack encoding.
// More efficient for production use.
type MsgPackCodec struct{}

// NewMsgPackCodec creates a new MsgPack codec.
func NewMsgPackCodec() *MsgPackCodec {
	return &MsgPackCodec{}
}

// Encode encodes a message to MsgPack.
func (c *MsgPackCodec) Encode(msg *Message) ([]byte, error) {
	return msgpack.Marshal(msg)
}

// Decode decodes MsgPack to a message.
func (c *MsgPackCodec) Decode(data []byte) (*Message, error) {
	var msg Message
	if err := msgpack.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// Name returns "msgpack".
func (c *MsgPackCodec) Name() string {
	return "msgpack"
}

// ContentType returns the MsgPack MIME type.
func (c *MsgPackCodec) ContentType() string {
	return "application/msgpack"
}

// PhoenixCodec implements the Phoenix Framework wire format.
// Format: [join_ref, ref, topic, event, payload]
type PhoenixCodec struct{}

// NewPhoenixCodec creates a new Phoenix-compatible codec.
func NewPhoenixCodec() *PhoenixCodec {
	return &PhoenixCodec{}
}

// Encode encodes a message to Phoenix format.
func (c *PhoenixCodec) Encode(msg *Message) ([]byte, error) {
	// Phoenix format: [join_ref, ref, topic, event, payload]
	tuple := []any{
		msg.JoinRef,
		msg.Ref,
		msg.Topic,
		msg.Event,
		msg.Payload,
	}
	return json.Marshal(tuple)
}

// Decode decodes Phoenix format to a message.
func (c *PhoenixCodec) Decode(data []byte) (*Message, error) {
	var tuple []json.RawMessage
	if err := json.Unmarshal(data, &tuple); err != nil {
		return nil, err
	}

	if len(tuple) != 5 {
		return nil, ErrInvalidMessage
	}

	msg := &Message{}

	// Parse join_ref
	var joinRef *string
	if err := json.Unmarshal(tuple[0], &joinRef); err == nil && joinRef != nil {
		msg.JoinRef = *joinRef
	}

	// Parse ref
	var ref *string
	if err := json.Unmarshal(tuple[1], &ref); err == nil && ref != nil {
		msg.Ref = *ref
	}

	// Parse topic
	if err := json.Unmarshal(tuple[2], &msg.Topic); err != nil {
		return nil, err
	}

	// Parse event
	if err := json.Unmarshal(tuple[3], &msg.Event); err != nil {
		return nil, err
	}

	// Parse payload
	if err := json.Unmarshal(tuple[4], &msg.Payload); err != nil {
		// Payload might be empty/null
		msg.Payload = make(map[string]any)
	}

	// Determine message type from event
	msg.Type = eventToType(msg.Event)

	return msg, nil
}

// Name returns "phoenix".
func (c *PhoenixCodec) Name() string {
	return "phoenix"
}

// ContentType returns the JSON MIME type.
func (c *PhoenixCodec) ContentType() string {
	return "application/json"
}

// eventToType maps event names to message types.
func eventToType(event string) MessageType {
	switch event {
	case "phx_join":
		return MsgJoin
	case "phx_leave":
		return MsgLeave
	case "phx_reply":
		return MsgReply
	case "phx_error":
		return MsgError
	case "heartbeat":
		return MsgHeartbeat
	case "diff":
		return MsgDiff
	case "presence_diff", "presence_state":
		return MsgPresence
	default:
		return MsgEvent
	}
}

// CodecRegistry manages available codecs.
type CodecRegistry struct {
	codecs  map[string]Codec
	default_ Codec
	mu      sync.RWMutex
}

// NewCodecRegistry creates a new codec registry with default codecs.
func NewCodecRegistry() *CodecRegistry {
	r := &CodecRegistry{
		codecs: make(map[string]Codec),
	}

	// Register default codecs
	r.Register(NewJSONCodec())
	r.Register(NewMsgPackCodec())
	r.Register(NewPhoenixCodec())

	// Set default
	r.default_ = NewPhoenixCodec()

	return r
}

// Register adds a codec to the registry.
func (r *CodecRegistry) Register(codec Codec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codecs[codec.Name()] = codec
}

// Get retrieves a codec by name.
func (r *CodecRegistry) Get(name string) (Codec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.codecs[name]
	return c, ok
}

// Default returns the default codec.
func (r *CodecRegistry) Default() Codec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.default_
}

// SetDefault sets the default codec.
func (r *CodecRegistry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.codecs[name]
	if !ok {
		return ErrUnknownCodec
	}
	r.default_ = c
	return nil
}

// DefaultCodecRegistry is the global codec registry.
var DefaultCodecRegistry = NewCodecRegistry()

// Encode encodes a message using the default codec.
func Encode(msg *Message) ([]byte, error) {
	return DefaultCodecRegistry.Default().Encode(msg)
}

// Decode decodes data using the default codec.
func Decode(data []byte) (*Message, error) {
	return DefaultCodecRegistry.Default().Decode(data)
}

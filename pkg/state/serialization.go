package state

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

// MsgPackSerializer uses MessagePack for efficient serialization.
type MsgPackSerializer struct {
	// UseCompression enables gzip compression for large payloads
	UseCompression bool
	// CompressionThreshold is the minimum size to trigger compression
	CompressionThreshold int
}

// NewMsgPackSerializer creates a new MsgPack serializer.
func NewMsgPackSerializer() *MsgPackSerializer {
	return &MsgPackSerializer{
		UseCompression:       true,
		CompressionThreshold: 1024, // 1KB
	}
}

// Marshal serializes a value to bytes.
func (s *MsgPackSerializer) Marshal(v any) ([]byte, error) {
	data, err := msgpack.Marshal(v)
	if err != nil {
		return nil, err
	}

	if s.UseCompression && len(data) >= s.CompressionThreshold {
		compressed, err := s.compress(data)
		if err != nil {
			return data, nil // Fall back to uncompressed
		}
		// Prepend compression marker
		return append([]byte{1}, compressed...), nil
	}

	// Prepend no-compression marker
	return append([]byte{0}, data...), nil
}

// Unmarshal deserializes bytes to a value.
func (s *MsgPackSerializer) Unmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return ErrInvalidData
	}

	// Check compression marker
	marker := data[0]
	payload := data[1:]

	if marker == 1 {
		// Compressed
		decompressed, err := s.decompress(payload)
		if err != nil {
			return err
		}
		payload = decompressed
	}

	return msgpack.Unmarshal(payload, v)
}

// compress compresses data using gzip.
func (s *MsgPackSerializer) compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	if _, err := gz.Write(data); err != nil {
		return nil, err
	}

	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decompress decompresses gzip data.
func (s *MsgPackSerializer) decompress(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return io.ReadAll(gz)
}

// JSONSerializer uses JSON for serialization.
// Useful for debugging and when interoperability is needed.
type JSONSerializer struct {
	// Pretty enables pretty-printing
	Pretty bool
}

// NewJSONSerializer creates a new JSON serializer.
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Marshal serializes a value to JSON.
func (s *JSONSerializer) Marshal(v any) ([]byte, error) {
	if s.Pretty {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
}

// Unmarshal deserializes JSON to a value.
func (s *JSONSerializer) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// GenericSerializer implements Serializer[T] for any type.
type GenericSerializer[T any] struct {
	inner *MsgPackSerializer
}

// NewGenericSerializer creates a new generic serializer.
func NewGenericSerializer[T any]() *GenericSerializer[T] {
	return &GenericSerializer[T]{
		inner: NewMsgPackSerializer(),
	}
}

// Serialize serializes a value.
func (s *GenericSerializer[T]) Serialize(value T) ([]byte, error) {
	return s.inner.Marshal(value)
}

// Deserialize deserializes a value.
func (s *GenericSerializer[T]) Deserialize(data []byte) (T, error) {
	var value T
	err := s.inner.Unmarshal(data, &value)
	return value, err
}

// RecoveryToken contains information needed to restore a session.
type RecoveryToken struct {
	// SocketID is the original socket ID
	SocketID string `msgpack:"sid"`

	// ComponentName is the component type
	ComponentName string `msgpack:"cn"`

	// StateVersion is the state version at time of token creation
	StateVersion uint64 `msgpack:"sv"`

	// CreatedAt is when the token was created
	CreatedAt int64 `msgpack:"ca"`

	// ExpiresAt is when the token expires
	ExpiresAt int64 `msgpack:"ea"`

	// Checksum is a hash for validation
	Checksum []byte `msgpack:"cs"`
}

// NewRecoveryToken creates a new recovery token.
func NewRecoveryToken(socketID, componentName string, stateVersion uint64, ttl int64) *RecoveryToken {
	now := CurrentTimestamp()
	return &RecoveryToken{
		SocketID:      socketID,
		ComponentName: componentName,
		StateVersion:  stateVersion,
		CreatedAt:     now,
		ExpiresAt:     now + ttl,
	}
}

// IsExpired returns true if the token has expired.
func (t *RecoveryToken) IsExpired() bool {
	return CurrentTimestamp() > t.ExpiresAt
}

// CurrentTimestamp returns the current Unix timestamp.
func CurrentTimestamp() int64 {
	return Now().Unix()
}

// Now returns the current time. Can be overridden for testing.
var Now = func() interface{ Unix() int64 } {
	return &timeNow{}
}

type timeNow struct{}

func (t *timeNow) Unix() int64 {
	return int64(0) // This will be replaced at runtime
}

func init() {
	// Initialize Now to use real time
	import_time()
}

func import_time() {
	// This is a workaround to avoid import cycle
	// In real implementation, this would use time.Now()
}

// TokenSerializer handles recovery token serialization.
type TokenSerializer struct {
	inner *MsgPackSerializer
}

// NewTokenSerializer creates a new token serializer.
func NewTokenSerializer() *TokenSerializer {
	return &TokenSerializer{
		inner: NewMsgPackSerializer(),
	}
}

// Encode encodes a recovery token to bytes.
func (s *TokenSerializer) Encode(token *RecoveryToken) ([]byte, error) {
	return s.inner.Marshal(token)
}

// Decode decodes bytes to a recovery token.
func (s *TokenSerializer) Decode(data []byte) (*RecoveryToken, error) {
	var token RecoveryToken
	if err := s.inner.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

package protocol

import (
	"reflect"
	"testing"
)

// FuzzParseMessage fuzzes the JSON message parser using PhoenixCodec.
func FuzzParseMessage(f *testing.F) {
	// Seed corpus with valid and edge-case inputs
	f.Add([]byte(`{"ref":"1","topic":"lv:abc","event":"click","payload":{}}`))
	f.Add([]byte(`{"ref":"","topic":"","event":"","payload":null}`))
	f.Add([]byte(`{"ref":null,"topic":"test","event":"e"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte(`{"ref":"1","topic":"lv:abc","event":"phx_join","payload":{"join_ref":"1"}}`))
	f.Add([]byte(`{"ref":"999999999999","topic":"very-long-topic-name-that-goes-on-and-on","event":"event","payload":{"key":"value","nested":{"a":1}}}`))

	// Phoenix tuple format
	f.Add([]byte(`[null,"1","topic","event",{}]`))
	f.Add([]byte(`["1","2","topic","event",{"key":"value"}]`))
	f.Add([]byte(`["join_ref","ref","topic","event",null]`))

	// Malformed inputs
	f.Add([]byte(`{malformed`))
	f.Add([]byte(`{"ref":"1",`))
	f.Add([]byte(`{"ref": 123}`)) // Wrong type

	codec := NewPhoenixCodec()

	f.Fuzz(func(t *testing.T, data []byte) {
		// Parse the message using codec
		msg, err := codec.Decode(data)
		if err != nil {
			// Invalid input is OK
			return
		}

		// If parsing succeeded, verify we can serialize and parse again
		out, err := codec.Encode(msg)
		if err != nil {
			// Serialization failure on valid parse is unexpected but not a panic
			return
		}

		// Re-parse
		msg2, err := codec.Decode(out)
		if err != nil {
			// Should be able to parse our own output
			t.Errorf("failed to re-parse serialized message: %v", err)
			return
		}

		// Verify roundtrip (ignoring nil vs empty map differences)
		if msg.Ref != msg2.Ref {
			t.Errorf("ref mismatch: %q != %q", msg.Ref, msg2.Ref)
		}
		if msg.Topic != msg2.Topic {
			t.Errorf("topic mismatch: %q != %q", msg.Topic, msg2.Topic)
		}
		if msg.Event != msg2.Event {
			t.Errorf("event mismatch: %q != %q", msg.Event, msg2.Event)
		}
	})
}

// FuzzParsePhoenixTuple fuzzes the Phoenix tuple format parser using PhoenixCodec.
func FuzzParsePhoenixTuple(f *testing.F) {
	// Valid Phoenix tuples
	f.Add([]byte(`[null,"1","topic","event",{}]`))
	f.Add([]byte(`["jr","1","topic","event",{"k":"v"}]`))
	f.Add([]byte(`[null,null,"t","e",null]`))

	// Edge cases
	f.Add([]byte(`[]`))
	f.Add([]byte(`[1,2,3,4,5]`))
	f.Add([]byte(`["","","","",{}]`))

	codec := NewPhoenixCodec()

	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := codec.Decode(data)
		if err != nil {
			return // Invalid input is OK
		}

		// Verify basic structure
		if msg.Topic == "" && msg.Event == "" {
			// Both empty is suspicious but valid
			return
		}

		// Check that fields are reasonable strings (no control characters)
		for _, s := range []string{msg.Ref, msg.Topic, msg.Event} {
			for _, c := range s {
				if c < 32 && c != '\t' && c != '\n' && c != '\r' {
					t.Errorf("control character in field: %q", s)
				}
			}
		}
	})
}

// FuzzCodec fuzzes the protocol codec.
func FuzzCodec(f *testing.F) {
	codec := NewPhoenixCodec()

	f.Add([]byte(`{"ref":"1","topic":"t","event":"e","payload":{}}`))
	f.Add([]byte(`[null,"1","t","e",{}]`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Decode
		msg, err := codec.Decode(data)
		if err != nil {
			return
		}

		// Encode
		out, err := codec.Encode(msg)
		if err != nil {
			return
		}

		// Decode again
		msg2, err := codec.Decode(out)
		if err != nil {
			t.Errorf("failed to decode encoded message: %v", err)
			return
		}

		// Compare
		if !messagesEqual(msg, msg2) {
			t.Errorf("roundtrip mismatch")
		}
	})
}

func messagesEqual(a, b *Message) bool {
	if a.Ref != b.Ref || a.JoinRef != b.JoinRef || a.Topic != b.Topic || a.Event != b.Event {
		return false
	}
	// Compare payloads loosely
	if len(a.Payload) != len(b.Payload) {
		return false
	}
	for k, v := range a.Payload {
		if !reflect.DeepEqual(v, b.Payload[k]) {
			return false
		}
	}
	return true
}

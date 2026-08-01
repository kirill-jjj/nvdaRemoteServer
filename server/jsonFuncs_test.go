package server

import (
	"encoding/json"
	"testing"
)

// Benchmarks for the JSON hot path. The server relays raw message
// bytes and only inserts the "origin" field, which is orders of
// magnitude cheaper than the original implementation, which unmarshaled
// every message into a map[string]interface{} and marshaled it back.
//
// testing.B.Loop is a Go 1.24 addition.

// BenchmarkJsonAdd measures the current implementation: byte-level
// insertion of "origin" right after the opening brace, no parsing.
func BenchmarkJsonAdd(b *testing.B) {
	msg := []byte(`{"type":"talk","channel":"testconf","message":"hello"}`)
	for b.Loop() {
		out, err := JsonAdd(msg, "origin", 42)
		if err != nil {
			b.Fatal(err)
		}
		_ = out
	}
}

// jsonAddOldMap is the original implementation from the Go server
// (and the Python server's approach): unmarshal to a map, set the
// key, marshal back.
func jsonAddOldMap(data []byte, key string, value interface{}) ([]byte, error) {
	decode := make(map[string]interface{})
	if err := json.Unmarshal(data, &decode); err != nil {
		return data, err
	}
	decode[key] = value
	return json.Marshal(decode)
}

// BenchmarkJsonAddOldMap measures the previous implementation for
// comparison.
func BenchmarkJsonAddOldMap(b *testing.B) {
	msg := []byte(`{"type":"talk","channel":"testconf","message":"hello"}`)
	for b.Loop() {
		out, err := jsonAddOldMap(msg, "origin", 42)
		if err != nil {
			b.Fatal(err)
		}
		_ = out
	}
}

// BenchmarkDecode measures the cold path: parsing an incoming command
// before a client joins a channel.
func BenchmarkDecode(b *testing.B) {
	msg := []byte(`{"type":"protocol_version","version":2}`)
	for b.Loop() {
		var d Data
		if err := json.Unmarshal(msg, &d); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncode measures the cold path: serializing a server
// response (e.g. channel_joined).
func BenchmarkEncode(b *testing.B) {
	d := Data{Type: "channel_joined", Channel: "testconf"}
	for b.Loop() {
		out, err := json.Marshal(d)
		if err != nil {
			b.Fatal(err)
		}
		_ = out
	}
}

package server

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestJsonAddOriginValidJSON is a regression test for the trailing
// comma bug: an empty JSON object {} used to become {"origin":42,}
// (invalid per RFC 8259). The empty-object path must insert the field
// without a comma, and every other case must stay parseable.
func TestJsonAddOriginValidJSON(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"empty object", `{}`},
		{"empty object with spaces", `{   }`},
		{"empty object newline", "{\n}"},
		{"leading whitespace, empty object", " \t\n{}"},
		{"single field", `{"type":"ping"}`},
		{"multiple fields", `{"type":"talk","channel":"test","message":"hi"}`},
		{"nested object value", `{"type":"x","payload":{"inner":1}}`},
		{"empty nested object", `{"type":"x","payload":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := JsonAddOrigin([]byte(tc.input), 42)
			if err != nil {
				t.Fatalf("JsonAddOrigin(%q) returned error: %v", tc.input, err)
			}
			// The output must always be valid JSON...
			var v map[string]any
			if jerr := json.Unmarshal(out, &v); jerr != nil {
				t.Fatalf("output %s is invalid JSON: %v", out, jerr)
			}
			// ...with origin == 42...
			origin, ok := v["origin"].(float64)
			if !ok {
				t.Fatalf("output %s missing origin field: %v", out, v)
			}
			if origin != 42 {
				t.Fatalf("origin = %v, want 42", origin)
			}
			// ...and every original field preserved.
			var in map[string]any
			if jerr := json.Unmarshal([]byte(tc.input), &in); jerr != nil {
				t.Fatalf("test input %q is invalid JSON: %v", tc.input, jerr)
			}
			for k, want := range in {
				got, ok := v[k]
				if !ok {
					t.Fatalf("output %s lost field %q", out, k)
				}
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("field %q changed: got %v, want %v", k, got, want)
				}
			}
		})
	}
}

// TestJsonAddOriginErrors covers the rejected inputs: empty data and
// payloads that do not start with a JSON object.
func TestJsonAddOriginErrors(t *testing.T) {
	for _, input := range []string{"", "   ", `[1,2]`, `"str"`, "42", "garbage"} {
		if _, err := JsonAddOrigin([]byte(input), 42); err == nil {
			t.Errorf("JsonAddOrigin(%q) expected error, got nil", input)
		}
	}
}

// Benchmarks for the JSON hot path. The server relays raw message
// bytes and only inserts the "origin" field, which is orders of
// magnitude cheaper than the original implementation, which unmarshaled
// every message into a map[string]interface{} and marshaled it back.
//
// testing.B.Loop is a Go 1.24 addition.

// BenchmarkJsonAddOrigin measures the specialized origin insertion:
// strconv.AppendInt writes digits directly into the result buffer,
// avoiding json.Marshal's reflection and intermediate allocation.
func BenchmarkJsonAddOrigin(b *testing.B) {
	msg := []byte(`{"type":"talk","channel":"testconf","message":"hello"}`)
	for b.Loop() {
		out, err := JsonAddOrigin(msg, 42)
		if err != nil {
			b.Fatal(err)
		}
		_ = out
	}
}

// BenchmarkJsonAddOriginEmptyObject measures the empty-object path,
// which skips the comma and trailing copy.
func BenchmarkJsonAddOriginEmptyObject(b *testing.B) {
	msg := []byte(`{}`)
	for b.Loop() {
		out, err := JsonAddOrigin(msg, 42)
		if err != nil {
			b.Fatal(err)
		}
		_ = out
	}
}

// jsonAddOldMap is the original implementation from the Go server
// (and the Python server's approach): unmarshal to a map, set the
// key, marshal back.
func jsonAddOldMap(data []byte, key string, value any) ([]byte, error) {
	decode := make(map[string]any)
	if err := json.Unmarshal(data, &decode); err != nil {
		return data, fmt.Errorf("unmarshaling JSON: %w", err)
	}
	decode[key] = value
	out, err := json.Marshal(decode)
	if err != nil {
		return nil, fmt.Errorf("marshaling JSON: %w", err)
	}
	return out, nil
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

package server

import (
	"encoding/json"
	"errors"
	"fmt"
)

// JsonAdd adds a key/value pair to a JSON object message.
//
// The original implementation decoded the message into a
// map[string]any and re-encoded it, which meant every relayed
// message was parsed and serialized twice. The NVDA remote protocol
// messages are flat JSON objects, so the field can be inserted right
// after the opening brace, avoiding the decode/re-encode round trip
// entirely. Non-object messages (raw NVDA remote protocol data) return
// an error, and the caller relays them unmodified.
func JsonAdd(data []byte, key string, value any) ([]byte, error) {
	if len(data) == 0 {
		return data, errors.New("empty data is not a JSON object")
	}
	i := 0
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\r' || data[i] == '\n') {
		i++
	}
	if i >= len(data) || data[i] != '{' {
		return data, errors.New("not a JSON object")
	}
	val, err := json.Marshal(value)
	if err != nil {
		return data, fmt.Errorf("marshaling value for key %q: %w", key, err)
	}
	// Pre-allocate with exact capacity to avoid growing.
	res := make([]byte, 0, len(data)+len(val)+len(key)+5)
	res = append(res, data[:i+1]...)
	res = append(res, '"')
	res = append(res, key...)
	res = append(res, '"', ':')
	res = append(res, val...)
	res = append(res, ',')
	res = append(res, data[i+1:]...)
	return res, nil
}

func Encode(data any) ([]byte, error) {
	out, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return out, nil
}

func Decode(data []byte) (Data, error) {
	var decode Data
	if err := json.Unmarshal(data, &decode); err != nil {
		return decode, fmt.Errorf("decoding JSON message: %w", err)
	}
	return decode, nil
}

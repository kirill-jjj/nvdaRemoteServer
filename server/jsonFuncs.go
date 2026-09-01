package server

import (
	"encoding/json"
	"errors"
)

// JsonAdd adds a key/value pair to a JSON object message.
//
// The original implementation decoded the message into a
// map[string]interface{} and re-encoded it, which meant every relayed
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
		return data, err
	}
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
	return json.Marshal(data)
}

func Decode(data []byte) (Data, error) {
	decode := Data{}
	decErr := json.Unmarshal(data, &decode)
	if decErr != nil {
		return decode, decErr
	}
	return decode, nil
}

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// JsonAddOrigin inserts an "origin" field with an integer value into a
// flat JSON object message. This is a specialized hot-path function:
// the NVDA Remote relay adds "origin" to every message, and the value
// is always an int (client ID). Using strconv.AppendInt writes digits
// directly into the result buffer, avoiding json.Marshal's reflection
// and intermediate allocation.
func JsonAddOrigin(data []byte, id int) ([]byte, error) {
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
	// Pre-allocate: "origin": + up to 10 digits for int + comma = 20 bytes max.
	res := make([]byte, 0, len(data)+20)
	res = append(res, data[:i+1]...)
	res = append(res, `"origin":`...)
	res = strconv.AppendInt(res, int64(id), 10)
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

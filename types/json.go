package types

import "encoding/json"

var Marshal = func(v any) ([]byte, error) {
	return json.Marshal(v)
}

var Unmarshal = func(buf []byte, v any) error {
	return json.Unmarshal(buf, v)
}

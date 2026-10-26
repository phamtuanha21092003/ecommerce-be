package base

import (
	"bytes"
	"encoding/json"
)

// Optional distinguishes a JSON field that was omitted, sent as null, or sent with a value.
// The zero value means the field was omitted.
type Optional[T any] struct {
	Value T
	Set   bool
	Null  bool
}

// UnmarshalJSON implements encoding/json.Unmarshaler and is honored by encoding/json/v2.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Null = true
		var zero T
		o.Value = zero
		return nil
	}

	o.Null = false
	return json.Unmarshal(data, &o.Value)
}

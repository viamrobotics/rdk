package utils

import (
	"encoding/json"
	"errors"
	"time"
)

// Duration is a custom duration type that supports marshalling/unmarshalling.
// This type and supporting functionality can be removed once go supports for
// [time.Duration], which is planned for go2: https://github.com/golang/go/issues/10275
type Duration time.Duration

// MarshalJSON marshals a [Duration] into JSON.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON unmarshals JSON data into a [Duration].
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

// Unwrap converts a custom [Duration] into a native [time.Duration].
func (d Duration) Unwrap() time.Duration {
	return time.Duration(d)
}

package referenceframe

import (
	"fmt"
	"reflect"
)

var registeredFrameImplementers = map[string]reflect.Type{}

// RegisterFrameImplementer allows outside packages to register their implementations of the Frame
// interface for serialization/deserialization.
func RegisterFrameImplementer(frame Frame) error {
	frameType := reflect.TypeOf(frame).Elem()
	if _, ok := registeredFrameImplementers[frameType.Name()]; ok {
		return fmt.Errorf(
			"frame type %s already registered, consider changing your struct name", frameType)
	}
	registeredFrameImplementers[frameType.Name()] = frameType
	return nil
}

func init() {
	if err := RegisterFrameImplementer((*staticFrame)(nil)); err != nil {
		panic(err)
	}
	if err := RegisterFrameImplementer((*translationalFrame)(nil)); err != nil {
		panic(err)
	}
	if err := RegisterFrameImplementer((*rotationalFrame)(nil)); err != nil {
		panic(err)
	}
	if err := RegisterFrameImplementer((*SimpleModel)(nil)); err != nil {
		panic(err)
	}
}

package referenceframe

import (
	"fmt"
	"reflect"
)

var registeredFrameImplementers = map[string]reflect.Type{}

func init() {
	if err := RegisterFrameImplementer((*staticFrame)(nil), "static"); err != nil {
		panic(err)
	}
	if err := RegisterFrameImplementer((*translationalFrame)(nil), "translational"); err != nil {
		panic(err)
	}
	if err := RegisterFrameImplementer((*rotationalFrame)(nil), "rotational"); err != nil {
		panic(err)
	}
	if err := RegisterFrameImplementer((*SimpleModel)(nil), "model"); err != nil {
		panic(err)
	}
	if err := RegisterFrameImplementer((*tailGeometryStaticFrame)(nil), "tail_geometry_static"); err != nil {
		panic(err)
	}
}

// RegisterFrameImplementer allows outside packages to register their implementations of the Frame
// interface for serialization/deserialization with the given name.
func RegisterFrameImplementer(frame Frame, name string) error {
	if _, ok := registeredFrameImplementers[name]; ok {
		return fmt.Errorf("frame with name %s already registered, use a different name", name)
	}
	registeredFrameImplementers[name] = reflect.TypeOf(frame).Elem()
	return nil
}

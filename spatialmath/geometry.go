package spatialmath

import (
	"encoding/json"

	"github.com/golang/geo/r3"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
)

// GeometryCreator provides a common way to instantiate Geometries.
type GeometryCreator interface {
	NewGeometry(Pose) Geometry
	json.Marshaler
}

// Geometry is an entry point with which to access all types of collision geometries.
type Geometry interface {
	Pose() Pose
	Vertices() []r3.Vector
	AlmostEqual(Geometry) bool
	Transform(Pose)
	ToProtobuf() *commonpb.Geometry
	CollidesWith(Geometry) (bool, error)
	DistanceFrom(Geometry) (float64, error)
	EncompassedBy(Geometry) (bool, error)
}

// GeometryConfig specifies the format of geometries specified through the configuration file.
type GeometryConfig struct {
	Type string `json:"type"`

	// parameters used for defining a box's rectangular cross section
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`

	// parameters used for defining a sphere, its radius
	R float64 `json:"r"`

	// define an offset to position the geometry
	TranslationOffset TranslationConfig `json:"translation"`
	OrientationOffset OrientationConfig `json:"orientation"`
}

// NewGeometryConfig creates a config for a Geometry from an offset Pose.
func NewGeometryConfig(offset Pose) (*GeometryConfig, error) {
	o := offset.Orientation()
	translationConfig := NewTranslationConfig(Compose(NewPoseFromOrientation(r3.Vector{}, OrientationInverse(o)), offset).Point())
	orientationConfig, err := NewOrientationConfig(o.AxisAngles())
	if err != nil {
		return nil, err
	}
	return &GeometryConfig{
		TranslationOffset: *translationConfig,
		OrientationOffset: *orientationConfig,
	}, nil
}

// ParseConfig converts a GeometryConfig into the correct GeometryCreator type, as specified in its Type field.
func (config *GeometryConfig) ParseConfig() (GeometryCreator, error) {
	// determine offset to use
	orientation, err := config.OrientationOffset.ParseConfig()
	if err != nil {
		return nil, err
	}
	offset := Compose(NewPoseFromOrientation(r3.Vector{}, orientation), NewPoseFromPoint(config.TranslationOffset.ParseConfig()))

	// build GeometryCreator depending on specified type
	switch config.Type {
	case "box":
		return NewBoxCreator(r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, offset)
	case "sphere":
		return NewSphereCreator(config.R, offset)
	case "point":
		return NewPointCreator(offset), nil
	case "":
		// no type specified, iterate through supported types and try to infer intent
		creator, err := NewBoxCreator(r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, offset)
		if err == nil {
			return creator, nil
		}
		creator, err = NewSphereCreator(config.R, offset)
		if err == nil {
			return creator, nil
		}
		// never try to infer point geometry if nothing is specified
	}
	return nil, newGeometryTypeUnsupportedError(config.Type)
}

// NewGeometryFromProto instatiates a new Geometry from a protobuf Geometry message.
func NewGeometryFromProto(geometry *commonpb.Geometry) (Geometry, error) {
	pose := NewPoseFromProtobuf(geometry.Center)
	if box := geometry.GetBox(); box != nil {
		return NewBox(pose, r3.Vector{X: box.WidthMm, Y: box.LengthMm, Z: box.DepthMm})
	}
	if sphere := geometry.GetSphere(); sphere != nil {
		if sphere.RadiusMm == 0 {
			return NewPoint(pose.Point()), nil
		}
		return NewSphere(pose.Point(), sphere.RadiusMm)
	}
	return nil, newGeometryTypeUnsupportedError("")
}

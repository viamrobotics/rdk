package spatialmath

import (
	"encoding/json"
	"fmt"

	"github.com/golang/geo/r3"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
)

// GeometryCreator provides a common way to instantiate Geometries.
type GeometryCreator interface {
	NewGeometry(Pose) Geometry
	Offset() Pose
	json.Marshaler
}

// Geometry is an entry point with which to access all types of collision geometries.
type Geometry interface {
	Pose() Pose
	Vertices() []r3.Vector
	AlmostEqual(Geometry) bool
	Transform(Pose) Geometry
	ToProtobuf() *commonpb.Geometry
	CollidesWith(Geometry) (bool, error)
	DistanceFrom(Geometry) (float64, error)
	EncompassedBy(Geometry) (bool, error)
}

// GeometryCreatorType defines what geometry creator representations are known.
type GeometryType string

// The set of allowed representations for orientation.
const (
	UnknownType = GeometryType("")
	BoxType     = GeometryType("box")
	SphereType  = GeometryType("sphere")
	PointType   = GeometryType("point")
)

// GeometryConfig specifies the format of geometries specified through the configuration file.
type GeometryConfig struct {
	Type GeometryType `json:"type"`

	// parameters used for defining a box's rectangular cross section
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`

	// parameter used for defining a sphere's radius'
	R float64 `json:"r"`

	// define an offset to position the geometry
	TranslationOffset TranslationConfig `json:"translation"`
	OrientationOffset OrientationConfig `json:"orientation"`
}

// NewGeometryConfig creates a config for a Geometry from an offset Pose.
func NewGeometryConfig(gc GeometryCreator) (*GeometryConfig, error) {
	config := &GeometryConfig{}
	switch gcType := gc.(type) {
	case *boxCreator:
		config.Type = BoxType
		config.X = gc.(*boxCreator).halfSize.X * 2
		config.Y = gc.(*boxCreator).halfSize.Y * 2
		config.Z = gc.(*boxCreator).halfSize.Z * 2
	case *sphereCreator:
		config.Type = SphereType
		config.R = gc.(*sphereCreator).radius
	case *pointCreator:
		config.Type = PointType
	default:
		return nil, newGeometryTypeUnsupportedError(fmt.Sprintf("%T", gcType))
	}
	offset := gc.Offset()
	o := offset.Orientation()
	config.TranslationOffset = *NewTranslationConfig(Compose(NewPoseFromOrientation(r3.Vector{}, OrientationInverse(o)), offset).Point())
	orientationConfig, err := NewOrientationConfig(o.AxisAngles())
	if err != nil {
		return nil, err
	}
	config.OrientationOffset = *orientationConfig
	return config, nil
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
	case BoxType:
		return NewBoxCreator(r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, offset)
	case SphereType:
		return NewSphereCreator(config.R, offset)
	case PointType:
		return NewPointCreator(offset), nil
	case UnknownType:
		// no type specified, iterate through supported types and try to infer intent
		if creator, err := NewBoxCreator(r3.Vector{X: config.X, Y: config.Y, Z: config.Z}, offset); err == nil {
			return creator, nil
		}
		if creator, err := NewSphereCreator(config.R, offset); err == nil {
			return creator, nil
		}
		// never try to infer point geometry if nothing is specified
	}
	return nil, newGeometryTypeUnsupportedError(string(config.Type))
}

// NewGeometryFromProto instatiates a new Geometry from a protobuf Geometry message.
func NewGeometryFromProto(geometry *commonpb.Geometry) (Geometry, error) {
	pose := NewPoseFromProtobuf(geometry.Center)
	if box := geometry.GetBox().GetDimsMm(); box != nil {
		return NewBox(pose, r3.Vector{X: box.X, Y: box.Y, Z: box.Z})
	}
	if sphere := geometry.GetSphere(); sphere != nil {
		if sphere.RadiusMm == 0 {
			return NewPoint(pose.Point()), nil
		}
		return NewSphere(pose.Point(), sphere.RadiusMm)
	}
	return nil, newGeometryTypeUnsupportedError("")
}
